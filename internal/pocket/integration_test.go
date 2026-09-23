package pocket

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt in after building native libraries and downloading the pinned models.
func TestNativeRecovery(t *testing.T) {
	dir := os.Getenv("FDB_TEST_NATIVE_DIR")
	if dir == "" {
		t.Skip("set FDB_TEST_NATIVE_DIR to exercise the real native engine")
	}
	engine, err := Open(dir, filepath.Join(dir, "models"), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	emit := func([]byte) error { return nil }
	if err := engine.Stream(ctx, "Hello.", filepath.Join(t.TempDir(), "missing.safetensors"), 1, emit); err == nil {
		t.Fatal("missing voice succeeded")
	}
	malformed := filepath.Join(t.TempDir(), "broken.safetensors")
	if err := os.WriteFile(malformed, []byte("invalid voice state"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := engine.Stream(ctx, "Hello.", malformed, 1, emit); err == nil {
		t.Fatal("malformed voice succeeded")
	}
	voice := filepath.Join(dir, "presets", "anna.safetensors")
	boom := errors.New("playback stopped")
	if err := engine.Stream(ctx, "Welcome traveler. This stream should stop early.", voice, 1, func([]byte) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("callback error: %v", err)
	}
	chunks := 0
	if err := engine.Stream(ctx, "The engine is ready again.", voice, 1, func(pcm []byte) error {
		chunks++
		if len(pcm)%2 != 0 {
			t.Error("incomplete PCM sample")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if chunks < 2 {
		t.Fatalf("expected multiple chunks, got %d", chunks)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	if err := engine.Stream(ctx, "Hello.", voice, 1, emit); err == nil {
		t.Fatal("closed engine succeeded")
	}
}
