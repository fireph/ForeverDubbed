package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

var defaultHTTPClient = &http.Client{Timeout: 60 * time.Second}

func (l *Local) client() *http.Client {
	if l.Client != nil {
		return l.Client
	}
	return defaultHTTPClient
}

func (l *Local) Health(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.Config.Endpoint+"/health", nil)
	if err != nil {
		return "", err
	}
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
	if health.StreamFormat != streamFormat {
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

// cancelRequest is best-effort and must outlive the cancelled playback context.
func (l *Local) cancelRequest(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if response, err := l.post(ctx, "/cancel", map[string]string{"id": id}); err == nil {
		response.Body.Close()
	}
}

// openStream transfers response body ownership to the caller only on success.
func (l *Local) openStream(ctx context.Context, id, voice, text string) (io.ReadCloser, int, error) {
	response, err := l.post(ctx, "/stream", map[string]string{"id": id, "voice": voice, "text": text})
	if err != nil {
		return nil, 0, err
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 500))
		return nil, 0, fmt.Errorf("local TTS %s: %s", response.Status, body)
	}
	rate, err := strconv.Atoi(response.Header.Get("X-Sample-Rate"))
	if err != nil || rate < 8000 || rate > 96000 || response.Header.Get("Content-Type") != streamContentType {
		response.Body.Close()
		return nil, 0, fmt.Errorf("invalid TTS stream format")
	}
	return response.Body, rate, nil
}
