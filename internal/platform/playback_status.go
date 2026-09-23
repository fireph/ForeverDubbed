package platform

import (
	"context"
	"time"
)

type playbackObserverKey struct{}

// WithPlaybackObserver reports when the first PCM buffer is accepted by the
// audio device. It is scoped to one utterance, including cancellation/replacement.
func WithPlaybackObserver(ctx context.Context, started func()) context.Context {
	return context.WithValue(ctx, playbackObserverKey{}, started)
}
func playbackStarted(ctx context.Context) {
	if started, ok := ctx.Value(playbackObserverKey{}).(func()); ok && started != nil {
		started()
	}
}

type playbackProgressKey struct{}

// WithPlaybackProgress reports completed device buffers. Total duration is
// unknown until the PCM input closes; it is never estimated from text.
func WithPlaybackProgress(ctx context.Context, report func(time.Duration, time.Duration, bool)) context.Context {
	return context.WithValue(ctx, playbackProgressKey{}, report)
}
func playbackProgress(ctx context.Context, played, total time.Duration, known bool) {
	if report, ok := ctx.Value(playbackProgressKey{}).(func(time.Duration, time.Duration, bool)); ok && report != nil {
		report(played, total, known)
	}
}
