package main

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/identity"
	"foreverdubbed/internal/protocol"
)

type testCapture struct {
	mu     sync.Mutex
	target string
	frames map[string]*image.RGBA
}

func (c *testCapture) Init(target string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.target = target
	return nil
}
func (c *testCapture) Desktop() image.Rectangle {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frames[c.target].Bounds()
}
func (c *testCapture) Capture(image.Rectangle) (*image.RGBA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frames[c.target], nil
}
func (c *testCapture) set(t *testing.T, target string, m protocol.Message) {
	t.Helper()
	frames, err := protocol.Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	im, err := protocol.Render(frames[0], 2)
	if err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames[target] = im
}

func waitCaptureState(t *testing.T, state *appstate.State, check func(appstate.Snapshot) bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !check(state.Snapshot()) {
		select {
		case <-deadline:
			t.Fatalf("capture did not settle: %+v", state.Snapshot())
		case <-time.After(time.Millisecond):
		}
	}
}

func runTestCapture(t *testing.T, backend *testCapture, state *appstate.State, mute bool, speak func(context.Context, protocol.Message) error) {
	t.Helper()
	resolver, err := identity.Load("")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- captureLoop(ctx, captureSettings{poll: time.Millisecond, scan: time.Millisecond, target: backend.target, backend: backend, mute: mute}, state, resolver, speak)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("capture worker did not stop")
		}
	})
}

func TestCaptureTargetChangeResetsAssembler(t *testing.T) {
	backend := &testCapture{target: "first", frames: map[string]*image.RGBA{}}
	// Even identical session/sequence IDs from another game must be decoded.
	backend.set(t, "first", protocol.Message{Session: 1, Sequence: 1, Text: "first game"})
	backend.set(t, "second", protocol.Message{Session: 1, Sequence: 1, Text: "second game"})
	state := appstate.New("first", "pocket", true)
	runTestCapture(t, backend, state, true, nil)
	waitCaptureState(t, state, func(s appstate.Snapshot) bool { return s.Message.Text == "first game" })
	state.SetCaptureTarget("second")
	waitCaptureState(t, state, func(s appstate.Snapshot) bool { return s.Message.Text == "second game" })
}

func TestOpticalControlsDuringPreparation(t *testing.T) {
	for _, kind := range []byte{protocol.KindStop, protocol.KindSkip} {
		t.Run(map[byte]string{protocol.KindStop: "stop", protocol.KindSkip: "skip"}[kind], func(t *testing.T) {
			backend := &testCapture{target: "game", frames: map[string]*image.RGBA{}}
			backend.set(t, "game", protocol.Message{Session: 1, Sequence: 1, Text: "first"})
			state := appstate.New("game", "pocket", false)
			state.SetQueueSpeech(true)
			runTestCapture(t, backend, state, false, func(ctx context.Context, m protocol.Message) error {
				// Deliberately never signal playback: controls must cancel synthesis.
				<-ctx.Done()
				return ctx.Err()
			})
			waitCaptureState(t, state, func(s appstate.Snapshot) bool { return s.PlaybackID == 1 && s.Audio == "Preparing speech" })
			backend.set(t, "game", protocol.Message{Session: 1, Sequence: 2, Text: "second"})
			waitCaptureState(t, state, func(s appstate.Snapshot) bool { return s.Queued == 1 })
			backend.set(t, "game", protocol.Message{Session: 1, Sequence: 3, Kind: kind})
			if kind == protocol.KindSkip {
				waitCaptureState(t, state, func(s appstate.Snapshot) bool {
					return s.PlaybackID == 2 && s.Queued == 0 && s.Audio == "Preparing speech"
				})
				backend.set(t, "game", protocol.Message{Session: 1, Sequence: 4, Kind: protocol.KindStop})
			}
			waitCaptureState(t, state, func(s appstate.Snapshot) bool { return s.PlaybackID == 0 && s.Queued == 0 && s.Audio == "Idle" })
		})
	}
}
