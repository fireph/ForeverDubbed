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

func playPCM(ctx context.Context, rate int, chunks <-chan []byte, device pcmDevice) (err error) {
	defer func() {
		if closeErr := device.Close(); err == nil {
			err = closeErr
		}
	}()
	if rate <= 0 {
		return fmt.Errorf("invalid PCM sample rate")
	}
	started := false
	var sizes []int
	var played, total int64
	report := func() {
		playbackProgress(ctx, time.Duration(played)*time.Second/time.Duration(rate*2), time.Duration(total)*time.Second/time.Duration(rate*2), chunks == nil)
	}
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
		for len(sizes) > pending {
			played += int64(sizes[0])
			sizes = sizes[1:]
		}
		if chunks == nil && pending == 0 {
			// Some devices return buffers before the final samples reach the
			// speaker. Drain normally, while cancellation still closes at once.
			if d, ok := device.(interface{ Drain(context.Context) error }); ok {
				if err := d.Drain(ctx); err != nil {
					return err
				}
			}
			report()
			return nil
		}
		report()
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
			sizes = append(sizes, len(pcm))
			total += int64(len(pcm))
			if !started {
				started = true
				playbackStarted(ctx)
			}
		}
	}
}
