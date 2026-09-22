package platform

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type testPCMDevice struct {
	queued   chan []byte
	pending  int
	finished atomic.Bool
	closed   bool
	fail     error
}

func (d *testPCMDevice) Queue(p []byte) error { d.pending++; d.queued <- p; return d.fail }
func (d *testPCMDevice) Pending() (int, error) {
	if d.finished.Load() {
		d.pending = 0
	}
	return d.pending, nil
}
func (d *testPCMDevice) Close() error { d.closed = true; return nil }

func TestPCMQueueBoundAndCancellation(t *testing.T) {
	chunks := make(chan []byte, 8)
	for i := 0; i < 8; i++ {
		chunks <- []byte{byte(i), 0}
	}
	device := &testPCMDevice{queued: make(chan []byte, 8)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- playPCM(ctx, chunks, device) }()
	for i := 0; i < 4; i++ {
		select {
		case p := <-device.queued:
			if p[0] != byte(i) {
				t.Fatal("reordered audio")
			}
		case <-time.After(time.Second):
			t.Fatal("no playback")
		}
	}
	select {
	case <-device.queued:
		t.Fatal("queue exceeded four buffers")
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation hung")
	}
	if !device.closed {
		t.Fatal("device not closed")
	}
}

func TestPCMWaitsForDeviceDrain(t *testing.T) {
	chunks := make(chan []byte, 1)
	chunks <- []byte{1, 0}
	close(chunks)
	device := &testPCMDevice{queued: make(chan []byte, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- playPCM(ctx, chunks, device) }()
	select {
	case <-device.queued:
	case <-ctx.Done():
		t.Fatal("no playback")
	}
	select {
	case <-done:
		t.Fatal("cut off final buffer")
	case <-time.After(20 * time.Millisecond):
	}
	device.finished.Store(true)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !device.closed {
		t.Fatal("device not closed")
	}
}

func TestPCMClosesOnDeviceError(t *testing.T) {
	chunks := make(chan []byte, 1)
	chunks <- []byte{1, 0}
	device := &testPCMDevice{queued: make(chan []byte, 1), fail: errors.New("device failed")}
	if err := playPCM(context.Background(), chunks, device); !errors.Is(err, device.fail) {
		t.Fatal(err)
	}
	if !device.closed {
		t.Fatal("device not closed")
	}
}
