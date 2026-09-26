package platform

import (
	"bytes"
	"context"
	"encoding/binary"
	"sync/atomic"
	"testing"
	"time"
)

func TestPlaybackVolumeChangesDuringUtterance(t *testing.T) {
	chunks := make(chan []byte, 1)
	device := &testPCMDevice{queued: make(chan []byte, 1)}
	var percent atomic.Int64
	percent.Store(100)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx = WithPlaybackVolume(ctx, func() float64 { return float64(percent.Load()) / 100 })
	done := make(chan error, 1)
	go func() { done <- playPCM(ctx, 24000, chunks, device) }()
	original := []byte{0x10, 0x27, 0xf0, 0xd8} // +10000, -10000
	for _, level := range []int64{100, 50, 0} {
		percent.Store(level)
		chunks <- original
		select {
		case got := <-device.queued:
			for i, sample := range []int16{10000, -10000} {
				want := int16(int64(sample) * level / 100)
				if actual := int16(binary.LittleEndian.Uint16(got[i*2:])); actual != want {
					t.Fatalf("volume %d: got %d, want %d", level, actual, want)
				}
			}
		case <-ctx.Done():
			t.Fatal("playback stalled")
		}
	}
	if !bytes.Equal(original, []byte{0x10, 0x27, 0xf0, 0xd8}) {
		t.Fatal("modified source PCM")
	}
	close(chunks)
	device.finished.Store(true)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
