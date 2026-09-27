package pocket

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"foreverdubbed/internal/buildtool"
)

func TestNativeSentenceFades(t *testing.T) {
	dir := os.Getenv("FDB_TEST_NATIVE_DIR")
	if dir == "" {
		t.Skip("set FDB_TEST_NATIVE_DIR to exercise the real native engine")
	}
	engine, err := Open(filepath.Join(dir, "models"), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	voice := filepath.Join(dir, "presets", "anna.safetensors")
	options := StreamOptions{DecodeSteps: 1, FadeInMS: 50, FadeOutMS: 100}
	var first, last int16
	chunks, endings, nonzero := 0, 0, 0
	if err := engine.Stream(ctx, "Welcome, traveler. It is good to see you.", voice, options, func(pcm []byte) error {
		if len(pcm) == 0 || len(pcm)%2 != 0 || len(pcm) > 4800 {
			t.Fatalf("invalid PCM buffer: %d", len(pcm))
		}
		if chunks == 0 {
			first = int16(binary.LittleEndian.Uint16(pcm))
		}
		chunks++
		last = int16(binary.LittleEndian.Uint16(pcm[len(pcm)-2:]))
		if last == 0 {
			endings++
		}
		for i := 0; i < len(pcm); i += 2 {
			if binary.LittleEndian.Uint16(pcm[i:]) != 0 {
				nonzero++
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if first != 0 || last != 0 || endings < 2 || chunks < 4 || nonzero == 0 {
		t.Fatalf("first=%d last=%d endings=%d chunks=%d nonzero=%d", first, last, endings, chunks, nonzero)
	}
	boom := errors.New("stop during faded speech")
	if err := engine.Stream(ctx, "Welcome back.", voice, options, func([]byte) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	// Same cached voice, next request without fades: per-request options reset.
	if err := engine.Stream(ctx, "The test is complete.", voice, StreamOptions{DecodeSteps: 1}, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

// Opt in after building native libraries and downloading the pinned models.
func TestNativeRecovery(t *testing.T) {
	dir := os.Getenv("FDB_TEST_NATIVE_DIR")
	if dir == "" {
		t.Skip("set FDB_TEST_NATIVE_DIR to exercise the real native engine")
	}
	engine, err := Open(filepath.Join(dir, "models"), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	emit := func([]byte) error { return nil }
	if err := engine.Stream(ctx, "Hello.", filepath.Join(t.TempDir(), "missing.safetensors"), StreamOptions{DecodeSteps: 1}, emit); err == nil {
		t.Fatal("missing voice succeeded")
	}
	malformed := filepath.Join(t.TempDir(), "broken.safetensors")
	if err := os.WriteFile(malformed, []byte("invalid voice state"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := engine.Stream(ctx, "Hello.", malformed, StreamOptions{DecodeSteps: 1}, emit); err == nil {
		t.Fatal("malformed voice succeeded")
	}
	voice := filepath.Join(dir, "presets", "anna.safetensors")
	boom := errors.New("playback stopped")
	if err := engine.Stream(ctx, "Welcome traveler. This stream should stop early.", voice, StreamOptions{DecodeSteps: 1}, func([]byte) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("callback error: %v", err)
	}
	// Exercise short final decoder batches and recovery after cancellation.
	// Larger inference batches must still emit bounded PCM playback buffers.
	for _, text := range []string{"Hi.", "The engine is ready again."} {
		chunks := 0
		if err := engine.Stream(ctx, text, voice, StreamOptions{DecodeSteps: 1}, func(pcm []byte) error {
			chunks++
			if len(pcm) == 0 || len(pcm)%2 != 0 || len(pcm) > 4800 {
				t.Errorf("invalid playback buffer size: %d", len(pcm))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if chunks < 2 {
			t.Fatalf("expected multiple chunks, got %d", chunks)
		}
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	if err := engine.Stream(ctx, "Hello.", voice, StreamOptions{DecodeSteps: 1}, emit); err == nil {
		t.Fatal("closed engine succeeded")
	}
}

// Exercise the actual release model set: no encoder, not even an empty placeholder.
func TestNativeSavedVoiceWithoutEncoder(t *testing.T) {
	dir := os.Getenv("FDB_TEST_NATIVE_DIR")
	if dir == "" {
		t.Skip("set FDB_TEST_NATIVE_DIR to exercise the real native engine")
	}
	models := t.TempDir()
	for _, asset := range PlaybackAssets() {
		if filepath.Dir(asset.Path) != "models" {
			continue
		}
		if err := buildtool.CopyFile(filepath.Join(dir, filepath.FromSlash(asset.Path)), filepath.Join(models, filepath.Base(asset.Path))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(models, "mimi_encoder.onnx")); !os.IsNotExist(err) {
		t.Fatalf("encoder must be absent: %v", err)
	}
	engine, err := Open(models, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	voice := filepath.Join(dir, "presets", "anna.safetensors")
	for _, text := range []string{"Welcome, traveler.", "Your next adventure awaits."} {
		nonzero := false
		if err := engine.Stream(ctx, text, voice, StreamOptions{DecodeSteps: 1}, func(pcm []byte) error {
			for _, b := range pcm {
				if b != 0 {
					nonzero = true
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if !nonzero {
			t.Fatal("saved voice produced no audible samples")
		}
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	// Excluding the encoder must not weaken validation of synthesis models.
	if err := os.Remove(filepath.Join(models, "mimi_decoder_int8.onnx")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyModels(models); err == nil {
		t.Fatal("accepted missing synthesis decoder")
	}
}
