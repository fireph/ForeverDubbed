package main

import (
	"context"
	"log"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/protocol"
)

func speakLoop(ctx context.Context, requests <-chan protocol.Message, speak func(context.Context, protocol.Message) error, state *appstate.State) {
	var cancel context.CancelFunc
	var finished chan error
	var nextID uint64
	var activeKind byte
	var pending []protocol.Message
	updateQueue := func() { state.Update(func(v *appstate.Snapshot) { v.Queued = len(pending) }) }
	stopCurrent := func() {
		if cancel != nil {
			cancel()
			<-finished
			cancel = nil
			finished = nil
			state.ResetPlayback()
		}
	}
	defer func() { stopCurrent(); pending = nil; updateQueue() }()
	start := func(message protocol.Message) {
		// Select quest content when playback starts, so queued quests use the
		// latest preference. Keep the original received message in app state.
		if message.IsQuest() {
			filters := state.Snapshot().Filters
			message.Text = message.DialogueText(filters.QuestTitle, filters.QuestObjectives)
			if message.Text == "" {
				return
			}
		}
		activeKind = message.Kind
		nextID++
		state.Update(func(v *appstate.Snapshot) {
			v.Audio = "Preparing speech"
			v.SpeechError = ""
			v.PlaybackID = nextID
			v.PlayingSpeaker = message.Speaker
		})
		child, c := context.WithCancel(ctx)
		cancel = c
		result := make(chan error, 1)
		finished = result
		go func() { result <- speak(child, message) }()
	}
	for {
		if ctx.Err() != nil {
			return
		}
		// Remove disabled categories before starting any queued message.
		filters := state.Snapshot().Filters
		kept := pending[:0]
		for _, message := range pending {
			if filters.Allows(message.Kind) {
				kept = append(kept, message)
			}
		}
		clear(pending[len(kept):])
		if len(kept) != len(pending) {
			pending = kept
			updateQueue()
		}
		if finished != nil && !filters.Allows(activeKind) {
			stopCurrent()
		}
		// Switching back to interrupt mode keeps only the newest waiting message.
		if !state.Snapshot().QueueSpeech && len(pending) > 0 {
			newest := pending[len(pending)-1]
			pending = nil
			updateQueue()
			stopCurrent()
			start(newest)
		}
		if finished == nil && len(pending) > 0 {
			next := pending[0]
			pending[0] = protocol.Message{}
			pending = pending[1:]
			updateQueue()
			start(next)
		}
		if finished == nil && len(pending) > 0 {
			continue // An empty quest body must not stall the remaining queue.
		}
		select {
		case <-ctx.Done():
			return
		case <-state.SpeechChanges():
			// Apply queue mode and category changes at the top of the loop.
		case id := <-state.AudioStops():
			if id == state.Snapshot().PlaybackID {
				stopCurrent()
			}
		case message, ok := <-requests:
			if !ok {
				requests = nil
				continue
			}
			if message.IsControl() {
				// Controls bypass queue mode. Cancelling the current utterance
				// advances any pending dialogue, just like the desktop buttons.
				stopCurrent()
				continue
			}
			if !state.Snapshot().Filters.Allows(message.Kind) {
				continue
			}
			if state.Snapshot().QueueSpeech && finished != nil {
				pending = append(pending, message)
				updateQueue()
			} else {
				pending = nil
				updateQueue()
				stopCurrent()
				start(message)
			}
		case err := <-finished:
			state.ResetPlayback()
			if err != nil && ctx.Err() == nil {
				log.Printf("TTS error: %v", err)
				state.Update(func(v *appstate.Snapshot) { v.SpeechError = err.Error() })
			}
			cancel()
			cancel = nil
			finished = nil
		}
	}
}
