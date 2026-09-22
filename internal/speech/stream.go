package speech

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/protocol"
)

const (
	streamFormat        = "pcm-s16le-v1"
	streamContentType   = "application/x-foreverdubbed-pcm"
	maxPCMFrameBytes    = 1 << 20
	maxPCMResponseBytes = 20 << 20
	textChunkRunes      = 180
)

func (l *Local) receive(ctx context.Context, id, voice string, m protocol.Message, ready chan<- int, chunks chan<- []byte) error {
	text := strings.TrimSpace(m.Title + ". " + m.Text)
	if m.Title == "" {
		text = m.Text
	}
	rate := 0
	for _, chunk := range Chunks(text, textChunkRunes) {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, sampleRate, err := l.openStream(ctx, id, voice, chunk)
		if err != nil {
			return err
		}
		if rate == 0 {
			rate = sampleRate
			ready <- rate
		} else if rate != sampleRate {
			body.Close()
			return fmt.Errorf("TTS sample rate changed during speech")
		}
		err = readPCM(ctx, body, chunks)
		body.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Each frame is a little-endian byte count followed by mono signed 16-bit PCM.
// A zero-length frame is required; EOF alone indicates interrupted generation.
func readPCM(ctx context.Context, r io.Reader, out chan<- []byte) error {
	total := 0
	for {
		var size uint32
		if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
			return fmt.Errorf("truncated TTS stream: %w", err)
		}
		if size == 0 {
			if total == 0 {
				return fmt.Errorf("empty TTS stream")
			}
			return nil
		}
		if size > maxPCMFrameBytes || size%2 != 0 || total+int(size) > maxPCMResponseBytes {
			return fmt.Errorf("invalid TTS PCM frame size: %d", size)
		}
		total += int(size)
		// Limit individual playback buffers even if a server sends large frames.
		for remaining := int(size); remaining > 0; {
			n := remaining
			if n > platform.MaxPCMBufferBytes {
				n = platform.MaxPCMBufferBytes
			}
			pcm := make([]byte, n)
			if _, err := io.ReadFull(r, pcm); err != nil {
				return fmt.Errorf("truncated TTS PCM: %w", err)
			}
			select {
			case out <- pcm:
			case <-ctx.Done():
				return ctx.Err()
			}
			remaining -= n
		}
	}
}
