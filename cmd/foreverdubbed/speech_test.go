package main

import (
	"context"
	"testing"
	"time"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/protocol"
)

func TestQueuedQuestContentUsesCurrentPreference(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state := appstate.New("game", "pocket", false)
	state.SetQueueSpeech(true)
	requests := make(chan protocol.Message)
	started := make(chan protocol.Message, 4)
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		speakLoop(ctx, requests, func(ctx context.Context, m protocol.Message) error {
			started <- m
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}, state)
	}()
	defer func() { cancel(); <-done }()
	receive := func(want string) {
		t.Helper()
		select {
		case m := <-started:
			if m.Speech() != want || m.DialogueText(false, false) != want {
				t.Fatalf("spoken text: %+v; want %q", m, want)
			}
		case <-ctx.Done():
			t.Fatal("speech worker stalled")
		}
	}
	quest := protocol.Message{Kind: 2, Speaker: "Thrall", Title: "Quest title", Text: "Main dialogue.", Objectives: "Bring supplies."}
	state.Received(quest)
	requests <- quest
	receive("Main dialogue.")
	requests <- protocol.Message{Kind: 2, Title: "Empty quest", Objectives: "Objectives only."}
	requests <- quest
	filters := state.Snapshot().Filters
	filters.QuestObjectives = true
	state.SetSpeechFilters(filters)
	release <- struct{}{}
	receive("Objectives only.")
	filters.QuestObjectives = false
	state.SetSpeechFilters(filters)
	// Empty bodies are skipped with objectives off, including within the queue.
	requests <- protocol.Message{Kind: 2, Title: "Empty quest", Objectives: "Skip this."}
	requests <- quest
	release <- struct{}{}
	receive("Main dialogue.")
	release <- struct{}{}
	receive("Main dialogue.")
	requests <- quest
	filters.QuestTitle = true
	state.SetSpeechFilters(filters)
	release <- struct{}{}
	receive("Quest title.\n\nMain dialogue.")
	requests <- quest
	filters.QuestObjectives = true
	state.SetSpeechFilters(filters)
	release <- struct{}{}
	receive("Quest title.\n\nMain dialogue.\n\nBring supplies.")
	if state.Snapshot().Message != quest {
		t.Fatal("speech preferences modified received data")
	}
}

func TestSpeechReplacementAndQuitWaitForCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requests := make(chan protocol.Message, 1)
	started := make(chan string, 3)
	stopped := make(chan string, 3)
	state := appstate.New("game", "pocket", false)
	speak := func(ctx context.Context, m protocol.Message) error {
		state.Audio("Playing audio")
		started <- m.Text
		<-ctx.Done()
		stopped <- m.Text
		return ctx.Err()
	}
	done := make(chan struct{})
	go func() { defer close(done); speakLoop(ctx, requests, speak, state) }()
	receive := func(ch <-chan string, want string) {
		t.Helper()
		select {
		case got := <-ch:
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("speech worker hung")
		}
	}
	requests <- protocol.Message{Text: "first"}
	receive(started, "first")
	requests <- protocol.Message{Text: "second"}
	receive(stopped, "first")
	receive(started, "second")
	state.StopAudio()
	receive(stopped, "second")
	requests <- protocol.Message{Text: "third"}
	receive(started, "third")
	if v := state.Snapshot(); v.SpeechError != "" || v.PlaybackID == 0 {
		t.Fatal("stop prevented subsequent speech", v)
	}
	cancel()
	receive(stopped, "third")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown hung")
	}
	if v := state.Snapshot(); v.Audio != "Idle" || v.SpeechError != "" {
		t.Fatal(v)
	}
}

func TestSpeechQueueAndModeChange(t *testing.T) {
	for _, switchMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordered playback and stop", true: "switch to newest"}[switchMode], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			state := appstate.New("game", "pocket", false)
			state.SetQueueSpeech(true)
			requests := make(chan protocol.Message)
			started := make(chan string, 4)
			complete := make(chan struct{})
			speak := func(ctx context.Context, m protocol.Message) error {
				state.Audio("Playing audio")
				started <- m.Text
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-complete:
					return nil
				}
			}
			done := make(chan struct{})
			go func() { defer close(done); speakLoop(ctx, requests, speak, state) }()
			defer func() { cancel(); <-done }()
			send := func(text string) {
				t.Helper()
				select {
				case requests <- protocol.Message{Text: text}:
				case <-ctx.Done():
					t.Fatal("worker hung")
				}
			}
			expect := func(want string) {
				t.Helper()
				select {
				case got := <-started:
					if got != want {
						t.Fatalf("got %q want %q", got, want)
					}
				case <-ctx.Done():
					t.Fatal("speech did not start")
				}
			}
			send("first")
			expect("first")
			send("second")
			send("third")
			for state.Snapshot().Queued != 2 {
				select {
				case <-ctx.Done():
					t.Fatal("messages not queued")
				case <-time.After(time.Millisecond):
				}
			}
			select {
			case got := <-started:
				t.Fatalf("queue interrupted first with %q", got)
			default:
			}
			if switchMode {
				state.SetQueueSpeech(false)
				expect("third")
			} else {
				complete <- struct{}{}
				expect("second")
				select {
				case requests <- protocol.Message{Kind: protocol.KindSkip}:
				case <-ctx.Done():
					t.Fatal("skip command blocked")
				}
				expect("third")
			}
			if v := state.Snapshot(); v.Queued != 0 || v.SpeechError != "" {
				t.Fatal(v)
			}
			// Skipping the last message should leave playback idle.
			select {
			case requests <- protocol.Message{Kind: protocol.KindStop}:
			case <-ctx.Done():
				t.Fatal("stop command blocked")
			}
			for state.Snapshot().PlaybackID != 0 {
				select {
				case <-ctx.Done():
					t.Fatal("last message did not stop")
				case <-time.After(time.Millisecond):
				}
			}
			if v := state.Snapshot(); v.Audio != "Idle" || v.SpeechError != "" {
				t.Fatal(v)
			}

		})
	}
}

func TestSpeechFiltersCancelAndPruneQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state := appstate.New("game", "pocket", false)
	state.SetQueueSpeech(true)
	requests := make(chan protocol.Message)
	started := make(chan byte, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		speakLoop(ctx, requests, func(ctx context.Context, m protocol.Message) error {
			started <- m.Kind
			<-ctx.Done()
			return ctx.Err()
		}, state)
	}()
	defer func() { cancel(); <-done }()
	send := func(kind byte) {
		t.Helper()
		select {
		case requests <- protocol.Message{Kind: kind, Text: "dialogue"}:
		case <-ctx.Done():
			t.Fatal("send blocked")
		}
	}
	expect := func(kind byte) {
		t.Helper()
		select {
		case got := <-started:
			if got != kind {
				t.Fatalf("spoke %d, want %d", got, kind)
			}
		case <-ctx.Done():
			t.Fatal("no speech")
		}
	}
	wait := func(check func(appstate.Snapshot) bool) {
		t.Helper()
		for !check(state.Snapshot()) {
			select {
			case <-ctx.Done():
				t.Fatal("state did not update")
			case <-time.After(time.Millisecond):
			}
		}
	}
	send(2)
	expect(2)
	send(3)
	send(5)
	wait(func(s appstate.Snapshot) bool { return s.Queued == 2 })
	state.SetSpeechFilters(appstate.SpeechFilters{Conversations: true, NPCSpeech: true})
	expect(5) // Stops the active quest and discards its queued progress message.
	wait(func(s appstate.Snapshot) bool { return s.Queued == 0 })
	send(4) // Filtered incoming quests must neither queue nor interrupt.
	send(1)
	wait(func(s appstate.Snapshot) bool { return s.Queued == 1 })
	select {
	case got := <-started:
		t.Fatalf("filtered message interrupted speech: %d", got)
	default:
	}
	state.SetSpeechFilters(appstate.SpeechFilters{Conversations: true})
	expect(1)
	state.SetSpeechFilters(appstate.SpeechFilters{})
	wait(func(s appstate.Snapshot) bool { return s.PlaybackID == 0 })
	send(protocol.KindSkip) // Controls remain usable with all categories disabled.
	send(0)                 // Connection tests remain audible.
	expect(0)
}

func TestVoiceChoicesMuteAndPruneQueue(t *testing.T) {
	for _, backend := range []string{"pocket", "system"} {
		t.Run(backend, func(t *testing.T) { testVoiceChoicesMuteAndPruneQueue(t, backend) })
	}
}

func testVoiceChoicesMuteAndPruneQueue(t *testing.T, backend string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state := appstate.New("game", backend, false)
	state.SetQueueSpeech(true)
	requests := make(chan protocol.Message)
	started := make(chan string, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		speakLoop(ctx, requests, func(ctx context.Context, m protocol.Message) error {
			started <- m.Text
			<-ctx.Done()
			return ctx.Err()
		}, state)
	}()
	defer func() { cancel(); <-done }()
	send := func(text, race string) {
		t.Helper()
		select {
		case requests <- protocol.Message{Kind: 1, Text: text, Race: race, Gender: "male"}:
		case <-ctx.Done():
			t.Fatal("send blocked")
		}
	}
	expect := func(want string) {
		t.Helper()
		select {
		case got := <-started:
			if got != want {
				t.Fatalf("spoke %q, want %q", got, want)
			}
		case <-ctx.Done():
			t.Fatal("no speech")
		}
	}
	wait := func(check func(appstate.Snapshot) bool) {
		t.Helper()
		for !check(state.Snapshot()) {
			select {
			case <-ctx.Done():
				t.Fatal("state did not update")
			case <-time.After(time.Millisecond):
			}
		}
	}
	state.SetVoiceChoices(map[string]string{"dwarf:male": "none"})
	send("shunned", "Dwarf") // Silenced races neither queue nor interrupt.
	select {
	case got := <-started:
		t.Fatalf("silenced race spoke: %q", got)
	case <-time.After(20 * time.Millisecond):
	}
	send("active", "Human")
	expect("active")
	send("silenced while active", "Dwarf")
	send("queued one", "Human")
	send("queued two", "Human")
	wait(func(s appstate.Snapshot) bool { return s.Queued == 2 })
	// Silencing the active race stops playback and prunes its queued messages.
	state.SetVoiceChoices(map[string]string{"human:male": "none"})
	wait(func(s appstate.Snapshot) bool { return s.PlaybackID == 0 && s.Queued == 0 })
	state.SetVoiceChoices(nil)
	send("restored", "Human")
	expect("restored")
}
