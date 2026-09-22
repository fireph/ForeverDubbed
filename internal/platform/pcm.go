package platform

import (
	"context"
	"fmt"
	"time"
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
			return nil
		}
		input := chunks
		if pending >= 4 {
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
			if len(pcm) == 0 || len(pcm)%2 != 0 || len(pcm) > 4800 {
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
