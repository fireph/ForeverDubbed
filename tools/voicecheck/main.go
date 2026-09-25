// Generate reviewable samples through the same native streaming API as the app.
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"foreverdubbed/internal/pocket"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dir := flag.String("native-dir", pocket.DefaultDir(), "native runtime directory")
	config := flag.String("config", "tts/voices.json", "voice configuration")
	out := flag.String("out", ".runtime/voice-samples", "sample directory")
	only := flag.String("voice", "", "one profile (default: all)")
	text := flag.String("text", "Welcome, traveler. The shadows are restless tonight. Speak quickly, and tell me what brings you here.", "sample text")
	flag.Parse()
	data, err := os.ReadFile(*config)
	if err != nil {
		return err
	}
	var cfg struct {
		Profiles map[string]struct {
			Voice     string `json:"voice"`
			Steps     int    `json:"decode_steps"`
			FadeInMS  int    `json:"fade_in_ms"`
			FadeOutMS int    `json:"fade_out_ms"`
		}
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	engine, err := pocket.Open(*dir, filepath.Join(*dir, "models"), 2)
	if err != nil {
		return err
	}
	defer engine.Close()
	if err = os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	names := []string{}
	for name := range cfg.Profiles {
		if *only == "" || name == *only {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("unknown profile %q", *only)
	}
	reports := []map[string]any{}
	for _, name := range names {
		p := cfg.Profiles[name]
		voice := filepath.Join(filepath.Dir(*config), p.Voice)
		if filepath.Ext(p.Voice) == "" {
			voice = filepath.Join(*dir, "presets", p.Voice+".safetensors")
		}
		if p.Steps == 0 {
			p.Steps = 1
		}
		start := time.Now()
		first := time.Duration(0)
		chunks := 0
		pcm := []byte{}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err = engine.Stream(ctx, *text, voice, pocket.StreamOptions{DecodeSteps: p.Steps, FadeInMS: p.FadeInMS, FadeOutMS: p.FadeOutMS}, func(chunk []byte) error {
			if chunks == 0 {
				first = time.Since(start)
			}
			chunks++
			pcm = append(pcm, chunk...)
			return nil
		})
		cancel()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		elapsed := time.Since(start)
		energy := 0.0
		for i := 0; i < len(pcm); i += 2 {
			v := float64(int16(binary.LittleEndian.Uint16(pcm[i:]))) / 32768
			energy += v * v
		}
		rms := math.Sqrt(energy / float64(len(pcm)/2))
		if chunks < 2 || first >= elapsed || rms < 0.001 {
			return fmt.Errorf("%s failed streaming/audio validation", name)
		}
		f, err := os.Create(filepath.Join(*out, name+".wav"))
		if err != nil {
			return err
		}
		header := make([]byte, 44)
		copy(header, "RIFF")
		binary.LittleEndian.PutUint32(header[4:], uint32(len(pcm)+36))
		copy(header[8:], "WAVEfmt ")
		binary.LittleEndian.PutUint32(header[16:], 16)
		binary.LittleEndian.PutUint16(header[20:], 1)
		binary.LittleEndian.PutUint16(header[22:], 1)
		binary.LittleEndian.PutUint32(header[24:], 24000)
		binary.LittleEndian.PutUint32(header[28:], 48000)
		binary.LittleEndian.PutUint16(header[32:], 2)
		binary.LittleEndian.PutUint16(header[34:], 16)
		copy(header[36:], "data")
		binary.LittleEndian.PutUint32(header[40:], uint32(len(pcm)))
		_, err = f.Write(append(header, pcm...))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		r := map[string]any{"voice": name, "chunks": chunks, "first_audio_ms": first.Milliseconds(), "generation_ms": elapsed.Milliseconds(), "audio_seconds": float64(len(pcm)) / 48000, "rms": rms}
		reports = append(reports, r)
		j, _ := json.Marshal(r)
		fmt.Println(string(j))
	}
	// Cancel a real generation at its first chunk, then prove the model is reusable.
	name := names[0]
	p := cfg.Profiles[name]
	voice := filepath.Join(filepath.Dir(*config), p.Voice)
	if filepath.Ext(p.Voice) == "" {
		voice = filepath.Join(*dir, "presets", p.Voice+".safetensors")
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = engine.Stream(ctx, *text, voice, pocket.StreamOptions{DecodeSteps: 1, FadeInMS: p.FadeInMS, FadeOutMS: p.FadeOutMS}, func([]byte) error { cancel(); return ctx.Err() })
	cancel()
	if err != context.Canceled {
		return fmt.Errorf("cancellation failed: %v", err)
	}
	if err = engine.Stream(context.Background(), "The test is complete.", voice, pocket.StreamOptions{DecodeSteps: 1, FadeInMS: p.FadeInMS, FadeOutMS: p.FadeOutMS}, func([]byte) error { return nil }); err != nil {
		return fmt.Errorf("reuse after cancellation: %w", err)
	}
	data, _ = json.MarshalIndent(map[string]any{"samples": reports, "cancel_and_reuse": "passed", "text": *text}, "", "  ")
	return os.WriteFile(filepath.Join(*out, "report.json"), data, 0644)
}
