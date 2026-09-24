// Package appstate shares runtime status without coupling capture or speech to a GUI.
package appstate

import (
	"sync"
	"time"

	"foreverdubbed/internal/protocol"
)

// SpeechFilters maps the addon's wire kinds to user-facing dialogue categories.
type SpeechFilters struct {
	Quests, Conversations, NPCSpeech bool
}

func (f SpeechFilters) Allows(kind byte) bool {
	switch kind {
	case 1:
		return f.Conversations // Gossip and quest-giver greeting windows.
	case 2, 3, 4:
		return f.Quests // Quest offer, progress, and completion.
	case 5:
		return f.NPCSpeech // Ambient NPC/boss chat and emotes.
	default:
		return true // Connection tests, books, and playback controls.
	}
}

type Snapshot struct {
	Filters                               SpeechFilters
	Target, Backend                       string
	Ready, Stopped                        bool
	Window, Tile                          bool
	CaptureError, SpeechError, FatalError string
	Audio                                 string
	QueueSpeech                           bool
	Queued                                int
	PlaybackID                            uint64
	Played, Duration                      time.Duration
	DurationKnown                         bool
	PlayingSpeaker                        string
	Voice                                 string
	Message                               protocol.Message
	Received                              time.Time
}

type State struct {
	mu            sync.RWMutex
	value         Snapshot
	stopAudio     chan uint64
	speechChanges chan struct{}
}

func New(target, backend string, muted bool) *State {
	audio := "Starting"
	if muted {
		audio = "Muted"
	}
	return &State{stopAudio: make(chan uint64, 1), speechChanges: make(chan struct{}, 1), value: Snapshot{Target: target, Backend: backend, Audio: audio, Filters: SpeechFilters{true, true, true}}}
}
func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}
func (s *State) Update(fn func(*Snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.value)
}
func (s *State) Capture(window, tile bool, err error) {
	s.Update(func(v *Snapshot) {
		v.Window, v.Tile = window, window && tile
		v.CaptureError = ""
		if err != nil {
			v.CaptureError = err.Error()
		}
	})
}
func (s *State) Received(m protocol.Message) {
	s.Update(func(v *Snapshot) { v.Message, v.Received = m, time.Now() })
}
func (s *State) Audio(phase string) {
	s.Update(func(v *Snapshot) { v.Audio = phase })
}
func (s *State) Finished(err error) {
	s.Update(func(v *Snapshot) {
		v.Stopped = true
		v.Ready, v.Window, v.Tile = false, false, false
		v.Audio = "Stopped"
		v.PlaybackID = 0
		if err != nil {
			v.FatalError = err.Error()
		}
	})
}

// StopAudio requests cancellation of the utterance visible when clicked.
// IDs prevent a delayed click from interrupting its replacement.
func (s *State) StopAudio() {
	id := s.Snapshot().PlaybackID
	if id == 0 {
		return
	}
	select {
	case s.stopAudio <- id:
	default:
	}
}
func (s *State) AudioStops() <-chan uint64 { return s.stopAudio }
func (s *State) ResetPlayback() {
	s.Update(func(v *Snapshot) {
		v.Audio = "Idle"
		v.PlaybackID = 0
		v.Played, v.Duration = 0, 0
		v.DurationKnown = false
		v.PlayingSpeaker = ""
	})
}

// SetQueueSpeech wakes the speech worker so mode changes apply immediately.
func (s *State) SetQueueSpeech(enabled bool) {
	s.Update(func(v *Snapshot) { v.QueueSpeech = enabled })
	select {
	case s.speechChanges <- struct{}{}:
	default:
	}
}
func (s *State) SpeechChanges() <-chan struct{} { return s.speechChanges }

// SetSpeechFilters applies immediately to active and queued speech.
func (s *State) SetSpeechFilters(filters SpeechFilters) {
	s.Update(func(v *Snapshot) { v.Filters = filters })
	select {
	case s.speechChanges <- struct{}{}:
	default:
	}
}
