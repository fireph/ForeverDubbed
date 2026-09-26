//go:build windows || darwin

package platform

import (
	"context"
	"fmt"
)

// PlayPCM opens the native audio device and streams bounded PCM buffers.
func PlayPCM(ctx context.Context, rate int, chunks <-chan []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if rate < 8000 || rate > 96000 {
		return fmt.Errorf("unsupported PCM sample rate: %d", rate)
	}
	device, err := openAudio(rate)
	if err != nil {
		return err
	}
	return playPCM(ctx, rate, chunks, device)
}
