package speech

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/protocol"
)

type Local struct {
	Config   *Config
	Override string
	Client   *http.Client
	Play     func(context.Context, int, <-chan []byte) error
}

// Speak streams model PCM directly into a bounded playback queue. The helper
// drains cancelled generation before reusing the model, but playback stops now.
func (l *Local) Speak(parent context.Context, m protocol.Message) error {
	voice, err := l.Config.Voice(m, l.Override)
	if err != nil {
		return err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(token[:])
	ctx, cancel := context.WithCancel(parent)
	defer func() {
		cancel()
		l.cancelRequest(id)
	}()
	chunks := make(chan []byte, platform.PCMQueueDepth)
	ready := make(chan int, 1)
	done := make(chan struct{})
	var streamErr error
	go func() {
		defer close(done)
		defer close(ready)
		defer close(chunks)
		streamErr = l.receive(ctx, id, voice, m, ready, chunks)
		if streamErr != nil {
			cancel()
		}
	}()
	var rate int
	select {
	case value, ok := <-ready:
		if !ok {
			<-done
			return streamErr
		}
		rate = value
	case <-ctx.Done():
		<-done
		if parent.Err() != nil {
			return parent.Err()
		}
		return streamErr
	}
	play := l.Play
	if play == nil {
		play = platform.PlayPCM
	}
	err = play(ctx, rate, chunks)
	cancel()
	<-done
	if parent.Err() != nil {
		return parent.Err()
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if streamErr != nil {
		return streamErr
	}
	return err
}
