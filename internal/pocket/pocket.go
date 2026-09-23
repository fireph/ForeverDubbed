// Package pocket embeds PocketTTS.cpp through cgo and exposes cancellable PCM streams.
package pocket

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const SampleRate = 24000

type library interface {
	create(string, int) (unsafe.Pointer, error)
	start(unsafe.Pointer, string, string, int) error
	read(unsafe.Pointer, []int16) (int, error)
	stop(unsafe.Pointer)
	destroy(unsafe.Pointer)
	close()
}

type Engine struct {
	mu     sync.Mutex
	lib    library
	handle unsafe.Pointer
}

func DefaultDir() string {
	exe, _ := os.Executable()
	for _, dir := range []string{filepath.Join(filepath.Dir(exe), "native"), filepath.Join(filepath.Dir(exe), "..", ".runtime", "native"), filepath.Join(".runtime", "native")} {
		if _, err := os.Stat(filepath.Join(dir, "models", "bundle.json")); err == nil {
			absolute, _ := filepath.Abs(dir)
			return absolute
		}
	}
	return filepath.Join(filepath.Dir(exe), "native")
}
func Open(dir, models string, threads int) (*Engine, error) {
	if threads < 1 || threads > 256 {
		return nil, fmt.Errorf("CPU threads must be 1..256")
	}
	if err := VerifyModels(models); err != nil {
		return nil, fmt.Errorf("native models: %w", err)
	}
	lib, err := openLibrary()
	if err != nil {
		return nil, err
	}
	handle, err := lib.create(models, threads)
	if err != nil {
		lib.close()
		return nil, err
	}
	return &Engine{lib: lib, handle: handle}, nil
}
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.handle != nil {
		e.lib.destroy(e.handle)
		e.lib.close()
		e.handle = nil
	}
	return nil
}

// Calls are serialized because ONNX state and the imported voice cache are mutable.
func (e *Engine) Stream(ctx context.Context, text, voice string, steps int, emit func([]byte) error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.IndexByte(text, 0) >= 0 || strings.IndexByte(voice, 0) >= 0 {
		return fmt.Errorf("text and voice paths must not contain NUL bytes")
	}
	if e.handle == nil {
		return fmt.Errorf("native engine is closed")
	}
	if err := e.lib.start(e.handle, text, voice, steps); err != nil {
		return err
	}
	defer e.lib.stop(e.handle)
	buffer := make([]int16, 2400)
	total := 0
	timer := time.NewTicker(2 * time.Millisecond)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := e.lib.read(e.handle, buffer)
		if err != nil {
			return err
		}
		if n == -1 {
			if total == 0 {
				return fmt.Errorf("native engine returned no audio")
			}
			return nil
		}
		if n < -1 || n > len(buffer) {
			return fmt.Errorf("invalid native sample count: %d", n)
		}
		if n > 0 {
			pcm := make([]byte, n*2)
			for i, v := range buffer[:n] {
				binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
			}
			if err := emit(pcm); err != nil {
				return err
			}
			total += n
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}
