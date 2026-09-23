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
