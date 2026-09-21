package speech

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/protocol"
	"io"
	"net/http"
	"strings"
	"time"
)

type Local struct {
	Config   *Config
	Override string
	Client   *http.Client
	Play     func(context.Context, []byte) error
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
		Ready  bool   `json:"ready"`
		Device string `json:"device"`
		Model  string `json:"model"`
		Engine string `json:"engine"`
		Digest string `json:"config_digest"`
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

// Speak pipelines synthesis one chunk ahead of playback. Cancellation stops
// playback immediately and discards old audio. The service finishes any active
// short generation before accepting another, because Pocket 3.1 has no stop API.
func (l *Local) Speak(ctx context.Context, m protocol.Message) error {
	voice, err := l.Config.Voice(m, l.Override)
	if err != nil {
		return err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(token[:])
	ctx, cancel := context.WithCancel(ctx)
	type result struct {
		wav []byte
		err error
	}
	results := make(chan result)
	done := make(chan struct{})
	defer func() {
		cancel()
		cancelCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if r, e := l.post(cancelCtx, "/cancel", map[string]string{"id": id}); e == nil {
			r.Body.Close()
		}
		<-done
	}()
	go func() {
		defer close(done)
		defer close(results)
		text := strings.TrimSpace(m.Title + ". " + m.Text)
		if m.Title == "" {
			text = m.Text
		}
		for _, chunk := range Chunks(text, 180) {
			if ctx.Err() != nil {
				return
			}
			r, e := l.post(ctx, "/synthesize", map[string]string{"id": id, "voice": voice, "text": chunk})
			var wav []byte
			if e == nil {
				wav, e = io.ReadAll(io.LimitReader(r.Body, (20<<20)+1))
				r.Body.Close()
				if e == nil && len(wav) > 20<<20 {
					e = fmt.Errorf("TTS audio exceeds 20 MiB")
				}
				if e == nil && r.StatusCode != 200 {
					e = fmt.Errorf("local TTS %s: %.500s", r.Status, wav)
				}
			}
			select {
			case results <- result{wav, e}:
			case <-ctx.Done():
				return
			}
			if e != nil {
				return
			}
		}
	}()
	play := l.Play
	if play == nil {
		play = platform.PlayWAV
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r, ok := <-results:
			if !ok {
				return nil
			}
			if r.err != nil {
				return r.err
			}
			if err := play(ctx, r.wav); err != nil {
				return err
			}
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
