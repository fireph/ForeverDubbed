package speech

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/protocol"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Local struct {
	Config   *Config
	Override string
	Client   *http.Client
	Play     func(context.Context, int, <-chan []byte) error
}

func (l *Local) client() *http.Client {
	if l.Client != nil {
		return l.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (l *Local) Health(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, l.Config.Endpoint+"/health", nil)
	r, err := l.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("local TTS is not running; start Start-ForeverDubbed.cmd: %w", err)
	}
	defer r.Body.Close()
	var health struct {
		Ready        bool   `json:"ready"`
		Device       string `json:"device"`
		Model        string `json:"model"`
		Engine       string `json:"engine"`
		Digest       string `json:"config_digest"`
		StreamFormat string `json:"stream_format"`
	}
	if r.StatusCode != 200 {
		return "", fmt.Errorf("TTS health returned %s", r.Status)
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16384)).Decode(&health); err != nil {
		return "", err
	}
	if health.Engine != "pocket-tts" {
		return "", fmt.Errorf("endpoint is not a ForeverDubbed Pocket TTS service")
	}
	if health.Digest != l.Config.Digest {
		return "", fmt.Errorf("TTS voice config changed; restart Start-ForeverDubbed.cmd")
	}
	if !health.Ready {
		return "", fmt.Errorf("TTS is still loading")
	}
	if health.StreamFormat != "pcm-s16le-v1" {
		return "", fmt.Errorf("TTS helper does not support streaming; restart Start-ForeverDubbed.cmd with the updated helper")
	}
	return health.Model + " on " + health.Device, nil
}

func (l *Local) post(ctx context.Context, path string, value any) (*http.Response, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.Config.Endpoint+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return l.client().Do(req)
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
	defer cancel()
	defer func() {
		cancel()
		cancelCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if r, e := l.post(cancelCtx, "/cancel", map[string]string{"id": id}); e == nil {
			r.Body.Close()
		}
	}()
	chunks := make(chan []byte, 4)
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

func (l *Local) receive(ctx context.Context, id, voice string, m protocol.Message, ready chan<- int, chunks chan<- []byte) error {
	text := strings.TrimSpace(m.Title + ". " + m.Text)
	if m.Title == "" {
		text = m.Text
	}
	rate := 0
	for _, chunk := range Chunks(text, 180) {
		if err := ctx.Err(); err != nil {
			return err
		}
		r, err := l.post(ctx, "/stream", map[string]string{"id": id, "voice": voice, "text": chunk})
		if err != nil {
			return err
		}
		err = func() error {
			defer r.Body.Close()
			if r.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(r.Body, 500))
				return fmt.Errorf("local TTS %s: %s", r.Status, body)
			}
			sampleRate, err := strconv.Atoi(r.Header.Get("X-Sample-Rate"))
			if err != nil || sampleRate < 8000 || sampleRate > 96000 || r.Header.Get("Content-Type") != "application/x-foreverdubbed-pcm" {
				return fmt.Errorf("invalid TTS stream format")
			}
			if rate == 0 {
				rate = sampleRate
				ready <- rate
			} else if rate != sampleRate {
				return fmt.Errorf("TTS sample rate changed during speech")
			}
			return readPCM(ctx, r.Body, chunks)
		}()
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
		if size > 1<<20 || size%2 != 0 || total+int(size) > 20<<20 {
			return fmt.Errorf("invalid TTS PCM frame size: %d", size)
		}
		total += int(size)
		// Limit individual playback buffers even if a server sends large frames.
		for remaining := int(size); remaining > 0; {
			n := remaining
			if n > 4800 {
				n = 4800
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

// Chunks keeps words intact where possible and prefers sentence boundaries.
func Chunks(text string, maxRunes int) []string {
	if maxRunes < 1 {
		return nil
	}
	var chunks []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			chunks = append(chunks, string(current))
			current = nil
		}
	}
	for _, word := range strings.Fields(text) {
		runes := []rune(word)
		if len(current) > 0 && len(current)+1+len(runes) > maxRunes {
			flush()
		}
		for len(runes) > maxRunes {
			flush()
			chunks = append(chunks, string(runes[:maxRunes]))
			runes = runes[maxRunes:]
		}
		if len(current) > 0 {
			current = append(current, ' ')
		}
		current = append(current, runes...)
		if len(current) >= 40 && strings.ContainsAny(string(current[len(current)-1]), ".!?;") {
			flush()
		}
	}
	flush()
	return chunks
}
