package platform

import (
	"context"
	"fmt"
	"time"
)

const (
	// MaxPCMBufferBytes caps each buffer at 100 ms of 24 kHz mono 16-bit audio.
	MaxPCMBufferBytes = 4800
	// PCMQueueDepth bounds each of the receiving and device queues.
	PCMQueueDepth = 4
)

// pcmDevice owns buffers until Pending reports they have finished playing.
type pcmDevice interface {
	Queue([]byte) error
	Pending() (int, error)
	Close() error
}

func playPCM(ctx context.Context, chunks <-chan []byte, device pcmDevice) (err error) {
	defer func() {
		if closeErr := device.Close(); err == nil {
			err = closeErr
		}
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		pending, err := device.Pending()
		if err != nil {
			return err
		}
		if chunks == nil && pending == 0 {
			// Some devices return buffers before the final samples reach the
			// speaker. Drain normally, while cancellation still closes at once.
			if d, ok := device.(interface{ Drain(context.Context) error }); ok {
				return d.Drain(ctx)
			}
			return nil
		}
		input := chunks
		if pending >= PCMQueueDepth {
			input = nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case pcm, ok := <-input:
			if !ok {
				chunks = nil
				continue
			}
			if len(pcm) == 0 || len(pcm)%2 != 0 || len(pcm) > MaxPCMBufferBytes {
				return fmt.Errorf("invalid PCM playback buffer")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := device.Queue(pcm); err != nil {
				return err
			}
		}
	}
}
