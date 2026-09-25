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
	closeErr error
}

func (d *testPCMDevice) Queue(p []byte) error { d.pending++; d.queued <- p; return d.fail }
func (d *testPCMDevice) Pending() (int, error) {
	if d.finished.Load() {
		d.pending = 0
	}
	return d.pending, nil
}
func (d *testPCMDevice) Close() error { d.closed = true; return d.closeErr }

func TestPCMQueueBoundAndCancellation(t *testing.T) {
	chunks := make(chan []byte, 8)
	for i := 0; i < 8; i++ {
		chunks <- []byte{byte(i), 0}
	}
	device := &testPCMDevice{queued: make(chan []byte, 8)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- playPCM(ctx, 24000, chunks, device) }()
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
	go func() { done <- playPCM(ctx, 24000, chunks, device) }()
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
	if err := playPCM(context.Background(), 24000, chunks, device); !errors.Is(err, device.fail) {
		t.Fatal(err)
	}
	if !device.closed {
		t.Fatal("device not closed")
	}
}

type drainingPCMDevice struct {
	testPCMDevice
	draining chan struct{}
	release  chan struct{}
}

func (d *drainingPCMDevice) Drain(ctx context.Context) error {
	close(d.draining)
	select {
	case <-d.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func TestPCMHardwareDrainAndCancellation(t *testing.T) {
	for _, cancelDrain := range []bool{false, true} {
		chunks := make(chan []byte)
		close(chunks)
		d := &drainingPCMDevice{draining: make(chan struct{}), release: make(chan struct{})}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := make(chan error, 1)
		go func() { done <- playPCM(ctx, 24000, chunks, d) }()
		select {
		case <-d.draining:
		case <-ctx.Done():
			t.Fatal("drain never started")
		}
		select {
		case <-done:
			t.Fatal("closed before hardware drain")
		default:
		}
		if cancelDrain {
			cancel()
		} else {
			close(d.release)
		}
		err := <-done
		cancel()
		if cancelDrain && !errors.Is(err, context.Canceled) || !cancelDrain && err != nil {
			t.Fatal(err)
		}
		if !d.closed {
			t.Fatal("device not closed after drain")
		}
	}
}

func TestPlaybackObserverStartsOnlyAfterSuccessfulQueue(t *testing.T) {
	for _, fail := range []error{nil, errors.New("queue failed")} {
		chunks := make(chan []byte, 2)
		chunks <- []byte{1, 0}
		chunks <- []byte{2, 0}
		close(chunks)
		device := &testPCMDevice{queued: make(chan []byte, 2), fail: fail}
		device.finished.Store(true)
		calls := 0
		ctx := WithPlaybackObserver(context.Background(), func() { calls++ })
		err := playPCM(ctx, 24000, chunks, device)
		if fail == nil && (err != nil || calls != 1) {
			t.Fatalf("successful playback: calls=%d err=%v", calls, err)
		}
		if fail != nil && (err == nil || calls != 0) {
			t.Fatalf("failed queue reported playback: calls=%d err=%v", calls, err)
		}
	}
}

func TestPlaybackProgressCountsCompletedBuffers(t *testing.T) {
	chunks := make(chan []byte, 2)
	chunks <- make([]byte, 4800)
	chunks <- make([]byte, 2400)
	close(chunks)
	device := &testPCMDevice{queued: make(chan []byte, 2)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ready := make(chan struct{}, 1)
	var played, total time.Duration
	var known bool
	ctx = WithPlaybackProgress(ctx, func(p, d time.Duration, k bool) {
		played, total, known = p, d, k
		if k && p == 0 && d == 150*time.Millisecond {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
	})
	done := make(chan error, 1)
	go func() { done <- playPCM(ctx, 24000, chunks, device) }()
	select {
	case <-ready: // Queued audio is not counted as played.
	case <-ctx.Done():
		t.Fatal("missing queued duration")
	}
	device.finished.Store(true)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !known || played != 150*time.Millisecond || total != played {
		t.Fatalf("played=%v total=%v known=%v", played, total, known)
	}
}

func TestPCMRejectsInvalidBuffersAndClosesDevice(t *testing.T) {
	for _, size := range []int{0, 1, 3, MaxPCMBufferBytes + 2} {
		chunks := make(chan []byte, 1)
		chunks <- make([]byte, size)
		close(chunks)
		device := &testPCMDevice{queued: make(chan []byte, 1)}
		if err := playPCM(context.Background(), 24000, chunks, device); err == nil {
			t.Fatalf("accepted buffer of %d bytes", size)
		}
		if !device.closed || len(device.queued) != 0 {
			t.Fatalf("invalid buffer reached the device or leaked it: %d bytes", size)
		}
	}
}

func TestPCMCloseErrorDoesNotHidePlaybackFailure(t *testing.T) {
	closeErr := errors.New("device close failed")
	queueErr := errors.New("device queue failed")
	for _, fail := range []error{nil, queueErr} {
		chunks := make(chan []byte, 1)
		chunks <- []byte{1, 0}
		close(chunks)
		device := &testPCMDevice{queued: make(chan []byte, 1), fail: fail, closeErr: closeErr}
		device.finished.Store(true)
		want := closeErr
		if fail != nil {
			want = fail
		}
		if err := playPCM(context.Background(), 24000, chunks, device); !errors.Is(err, want) {
			t.Fatalf("got %v, want %v", err, want)
		}
	}
}
