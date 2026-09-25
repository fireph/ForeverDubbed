package speech

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/pocket"
	"foreverdubbed/internal/protocol"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type Synthesizer interface {
	Stream(context.Context, string, string, int, func([]byte) error) error
	Close() error
}
type Local struct {
	Config     *Config
	Override   string
	PresetsDir string
	Engine     Synthesizer
	Play       func(context.Context, int, <-chan []byte) error
}

func OpenLocal(config *Config, override, nativeDir, modelsDir string, threads int) (*Local, error) {
	local := &Local{Config: config, Override: override, PresetsDir: filepath.Join(nativeDir, "presets")}
	for name := range config.Profiles {
		path, _, err := local.voiceFile(name)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("voice %s: %w (run go run ./tools/models)", name, err)
		}
	}
	if modelsDir == "" {
		modelsDir = filepath.Join(nativeDir, "models")
	}
	engine, err := pocket.Open(nativeDir, modelsDir, threads)
	if err != nil {
		return nil, err
	}
	local.Engine = engine
	return local, nil
}
func (l *Local) Close() error { return l.Engine.Close() }
func (l *Local) voiceFile(name string) (string, int, error) {
	p, ok := l.Config.Profiles[name]
	if !ok {
		return "", 0, fmt.Errorf("unknown voice %s", name)
	}
	steps := p.DecodeSteps
	if steps == 0 {
		steps = 1
	}
	path := p.Voice
	if filepath.Ext(path) == "" {
		if strings.ContainsAny(path, "/\\:") || path == "." || path == ".." {
			return "", 0, fmt.Errorf("invalid preset %q", path)
		}
		path = filepath.Join(l.PresetsDir, path+".safetensors")
	} else {
		if strings.ToLower(filepath.Ext(path)) != ".safetensors" {
			return "", 0, fmt.Errorf("voice %s needs an exported .safetensors state", name)
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(l.Config.BaseDir, path)
		}
	}
	return path, steps, nil
}

// Speak starts one continuous PCM playback queue while native inference produces
// short chunks. Both native generation and playback stop on cancellation.
func (l *Local) Speak(parent context.Context, m protocol.Message) error {
	name, err := l.Config.Voice(m, l.Override)
	if err != nil {
		return err
	}
	voice, steps, err := l.voiceFile(name)
	if err != nil {
		return err
	}
	gain := math.Pow(10, l.Config.Profiles[name].GainDB/20)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	chunks := make(chan []byte, platform.PCMQueueDepth)
	done := make(chan struct{})
	var streamErr error
	go func() {
		defer close(done)
		defer close(chunks)
		text := strings.TrimSpace(m.Title + ". " + m.Text)
		if m.Title == "" {
			text = m.Text
		}
		for _, text := range Chunks(text, 180) {
			streamErr = l.Engine.Stream(ctx, text, voice, steps, func(pcm []byte) error {
				pcm = amplifyPCM(pcm, gain)
				select {
				case chunks <- pcm:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			if streamErr != nil {
				cancel()
				return
			}
		}
	}()
	play := l.Play
	if play == nil {
		play = platform.PlayPCM
	}
	err = play(ctx, pocket.SampleRate, chunks)
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

// amplifyPCM scales signed 16-bit little-endian audio without changing timing.
// Saturate peaks to avoid integer wraparound; leave the engine's buffer intact.
func amplifyPCM(pcm []byte, gain float64) []byte {
	if gain == 1 {
		return pcm
	}
	out := make([]byte, len(pcm))
	copy(out, pcm)
	for i := 0; i+1 < len(out); i += 2 {
		sample := float64(int16(binary.LittleEndian.Uint16(pcm[i:]))) * gain
		sample = math.Max(-32768, math.Min(32767, math.Round(sample)))
		binary.LittleEndian.PutUint16(out[i:], uint16(int16(sample)))
	}
	return out
}
