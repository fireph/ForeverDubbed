package speech

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"foreverdubbed/internal/protocol"
)

func TestVoiceSelection(t *testing.T) {
	c, err := Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	c.NPCOverrides["4949"] = "orc_male"
	for _, tc := range []struct{ race, gender, npc, override, want string }{
		{"Human", "male", "", "", "human_male"},
		{"Skyborne", "male", "254100", "", "skyborne_male"},
		{"Skyborne Elf", "female", "", "", "skyborne_female"},
		{"Goblin", "male", "", "", "goblin_male"},
		{"Goblin", "female", "", "", "goblin_female"},
		{"Blood Elf", "female", "", "", "bloodelf_female"},
		{"Draenei", "male", "", "", "draenei_male"},
		{"Night Elf", "female", "", "", "nightelf_female"},
		{"Scourge", "male", "", "", "undead_male"},
		{"Orc", "", "", "", "orc_male"},
		{"Dragon", "female", "", "", "human_female"},
		{"", "", "", "", "narrator_male"},
		{"Human", "female", "4949", "", "orc_male"},
		{"Human", "female", "4949", "gnome_female", "gnome_female"},
	} {
		got, err := c.Voice(protocol.Message{Race: tc.race, Gender: tc.gender, NPCID: tc.npc}, tc.override)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: %s %v", tc, got, err)
		}
	}
	if _, err := c.Voice(protocol.Message{}, "missing"); err == nil {
		t.Fatal("accepted unknown voice")
	}
}

func TestChunksUnicodeAndLimit(t *testing.T) {
	text := "Welcome, traveler! " + strings.Repeat("Café 世界 ", 60)
	chunks := Chunks(text, 180)
	if strings.Join(chunks, " ") != strings.Join(strings.Fields(text), " ") {
		t.Fatal("text lost")
	}
	for _, s := range chunks {
		if !utf8.ValidString(s) || utf8.RuneCountInString(s) > 180 {
			t.Fatal("invalid chunk")
		}
	}
	if got := Chunks(strings.Repeat("世", 401), 180); len(got) != 3 || strings.Join(got, "") != strings.Repeat("世", 401) {
		t.Fatal("long word lost")
	}
}

func TestPocketPipelineAndCancellation(t *testing.T) {
	c, err := Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var voices, texts []string
	cancelled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]any{"ready": true, "engine": "pocket-tts", "config_digest": c.Digest, "model": "Pocket TTS", "device": "cpu"})
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path == "/cancel" {
			select {
			case cancelled <- struct{}{}:
			default:
			}
			w.Write([]byte("{}"))
			return
		}
		mu.Lock()
		voices = append(voices, body["voice"])
		texts = append(texts, body["text"])
		mu.Unlock()
		w.Write([]byte("test audio"))
	}))
	defer server.Close()
	c.Endpoint = server.URL
	local := Local{Config: c, Play: func(context.Context, []byte) error { return nil }}
	if _, err := local.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := local.Speak(context.Background(), protocol.Message{Race: "Orc", Gender: "male", Title: "A quest", Text: "Hello traveler."}); err != nil {
		t.Fatal(err)
	}
	<-cancelled
	mu.Lock()
	if len(voices) != 1 || voices[0] != "orc_male" || texts[0] != "A quest. Hello traveler." {
		t.Fatalf("%v %v", voices, texts)
	}
	mu.Unlock()
	playing := make(chan struct{})
	local.Play = func(ctx context.Context, b []byte) error { close(playing); <-ctx.Done(); return ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- local.Speak(ctx, protocol.Message{Text: "Speak until interrupted."}) }()
	select {
	case <-playing:
	case <-time.After(2 * time.Second):
		t.Fatal("no playback")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation hung")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("service not cancelled")
	}
}

func TestRejectWrongServiceAndChangedConfig(t *testing.T) {
	c, err := Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, response := range []string{`{"ready":true,"model":"Other TTS","device":"cpu"}`, `{"ready":true,"engine":"pocket-tts","config_digest":"old"}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(response)) }))
		c.Endpoint = s.URL
		l := Local{Config: c}
		if _, err := l.Health(context.Background()); err == nil {
			t.Fatal("accepted mismatched service")
		}
		s.Close()
	}
}
