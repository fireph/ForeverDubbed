package platform

import (
	"context"
	"encoding/binary"
	"math"
)

type playbackVolumeKey struct{}

// WithPlaybackVolume supplies a live gain for voice audio, independent of the
// system volume. The callback must be safe to call from the playback worker.
func WithPlaybackVolume(ctx context.Context, volume func() float64) context.Context {
	return context.WithValue(ctx, playbackVolumeKey{}, volume)
}

func playbackVolume(ctx context.Context) float64 {
	if source, ok := ctx.Value(playbackVolumeKey{}).(func() float64); ok && source != nil {
		v := source()
		if !(v >= 0) {
			return 0
		}
		return math.Min(v, 1)
	}
	return 1
}

func scalePlayback(pcm []byte, volume float64) []byte {
	if volume == 1 {
		return pcm
	}
	out := make([]byte, len(pcm))
	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(pcm[i:]))
		binary.LittleEndian.PutUint16(out[i:], uint16(int16(math.Round(float64(sample)*volume))))
	}
	return out
}
