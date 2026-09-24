package main

import (
	"context"
	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/protocol"
	"testing"
	"time"
)

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
				state.StopAudio()
				expect("third")
			}
			if v := state.Snapshot(); v.Queued != 0 || v.SpeechError != "" {
				t.Fatal(v)
			}
			// Skipping the last message should leave playback idle.
			state.StopAudio()
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
