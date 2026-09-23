package platform

import "context"

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
