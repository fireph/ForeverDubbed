package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
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
		{"Dragon", "female", "", "", "narrator_male"},
		{"", "", "", "", "narrator_male"},
		{"", "male", "", "", "narrator_male"},
		{"", "female", "", "", "narrator_male"},
		{"   ", "female", "", "", "narrator_male"},
		{"Human", "female", "", "", "human_female"},
		{"", "female", "4949", "", "orc_male"},
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
			json.NewEncoder(w).Encode(map[string]any{"ready": true, "engine": "pocket-tts", "config_digest": c.Digest, "model": "Pocket TTS", "device": "cpu", "stream_format": "pcm-s16le-v1"})
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
		w.Header().Set("Content-Type", "application/x-foreverdubbed-pcm")
		w.Header().Set("X-Sample-Rate", "24000")
		binary.Write(w, binary.LittleEndian, uint32(4))
		w.Write([]byte{1, 0, 2, 0})
		binary.Write(w, binary.LittleEndian, uint32(0))
	}))
	defer server.Close()
	c.Endpoint = server.URL
	local := Local{Config: c, Play: func(ctx context.Context, rate int, chunks <-chan []byte) error {
		for range chunks {
		}
		return nil
	}}
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
	local.Play = func(ctx context.Context, rate int, chunks <-chan []byte) error {
		<-chunks
		close(playing)
		<-ctx.Done()
		return ctx.Err()
	}
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
	for _, response := range []string{`{"ready":true,"model":"Other TTS","device":"cpu"}`, `{"ready":true,"engine":"pocket-tts","config_digest":"old"}`, `{"ready":true,"engine":"pocket-tts","config_digest":"` + c.Digest + `"}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(response)) }))
		c.Endpoint = s.URL
		l := Local{Config: c}
		if _, err := l.Health(context.Background()); err == nil {
			t.Fatal("accepted mismatched service")
		}
		s.Close()
	}
}

func TestPlaybackStartsBeforeSynthesisFinishes(t *testing.T) {
	c, _ := Load("../../tts/voices.json")
	playing := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cancel" {
			return
		}
		w.Header().Set("Content-Type", "application/x-foreverdubbed-pcm")
		w.Header().Set("X-Sample-Rate", "24000")
		binary.Write(w, binary.LittleEndian, uint32(4))
		w.Write([]byte{1, 0, 2, 0})
		w.(http.Flusher).Flush()
		select {
		case <-playing:
		case <-r.Context().Done():
			return
		}
		binary.Write(w, binary.LittleEndian, uint32(4))
		w.Write([]byte{3, 0, 4, 0})
		binary.Write(w, binary.LittleEndian, uint32(0))
	}))
	defer server.Close()
	c.Endpoint = server.URL
	var audio []byte
	local := Local{Config: c, Play: func(ctx context.Context, rate int, chunks <-chan []byte) error {
		if rate != 24000 {
			t.Errorf("rate = %d", rate)
		}
		for chunk := range chunks {
			if len(audio) == 0 {
				close(playing)
			}
			audio = append(audio, chunk...)
		}
		return ctx.Err()
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := local.Speak(ctx, protocol.Message{Text: "Hello traveler."}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(audio, []byte{1, 0, 2, 0, 3, 0, 4, 0}) {
		t.Fatalf("audio = %v", audio)
	}
}

func TestPCMStreamValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		size    uint32
		body    []byte
		end     bool
		wantErr bool
	}{
		{"valid", 4, []byte{1, 0, 2, 0}, true, false},
		{"missing terminator", 4, []byte{1, 0, 2, 0}, false, true},
		{"truncated", 4, []byte{1, 0}, false, true},
		{"odd", 3, []byte{1, 0, 2}, true, true},
		{"oversized", 1<<20 + 2, nil, false, true},
		{"empty", 0, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wire bytes.Buffer
			binary.Write(&wire, binary.LittleEndian, tc.size)
			wire.Write(tc.body)
			if tc.end {
				binary.Write(&wire, binary.LittleEndian, uint32(0))
			}
			err := readPCM(context.Background(), &wire, make(chan []byte, 4))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wire := bytes.NewReader([]byte{2, 0, 0, 0, 1, 0})
	if err := readPCM(ctx, wire, make(chan []byte)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStreamingCancellationWhileServerStalls(t *testing.T) {
	c, _ := Load("../../tts/voices.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cancel" {
			return
		}
		w.Header().Set("Content-Type", "application/x-foreverdubbed-pcm")
		w.Header().Set("X-Sample-Rate", "24000")
		binary.Write(w, binary.LittleEndian, uint32(2))
		w.Write([]byte{1, 0})
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	c.Endpoint = server.URL
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	local := Local{Config: c, Play: func(ctx context.Context, rate int, chunks <-chan []byte) error {
		select {
		case <-chunks:
			cancel()
		case <-ctx.Done():
		}
		<-ctx.Done()
		return ctx.Err()
	}}
	if err := local.Speak(ctx, protocol.Message{Text: "Hello"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStreamFailureStopsPlayback(t *testing.T) {
	c, _ := Load("../../tts/voices.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cancel" {
			return
		}
		w.Header().Set("Content-Type", "application/x-foreverdubbed-pcm")
		w.Header().Set("X-Sample-Rate", "24000")
		binary.Write(w, binary.LittleEndian, uint32(2))
		w.Write([]byte{1, 0}) // Abrupt EOF, without a successful terminator.
	}))
	defer server.Close()
	c.Endpoint = server.URL
	local := Local{Config: c, Play: func(ctx context.Context, rate int, chunks <-chan []byte) error { <-ctx.Done(); return ctx.Err() }}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := local.Speak(ctx, protocol.Message{Text: "Hello"}); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

func TestTextChunksShareOnePlaybackStream(t *testing.T) {
	c, _ := Load("../../tts/voices.json")
	text := strings.Repeat("Welcome traveler to the village. ", 20)
	want := Chunks(text, 180)
	var requests int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cancel" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if requests >= len(want) || body["text"] != want[requests] {
			t.Error("text chunks reordered")
		}
		requests++
		w.Header().Set("Content-Type", "application/x-foreverdubbed-pcm")
		w.Header().Set("X-Sample-Rate", "24000")
		binary.Write(w, binary.LittleEndian, uint32(2))
		w.Write([]byte{byte(requests), 0})
		binary.Write(w, binary.LittleEndian, uint32(0))
	}))
	defer server.Close()
	c.Endpoint = server.URL
	var calls, frames int
	local := Local{Config: c, Play: func(ctx context.Context, rate int, chunks <-chan []byte) error {
		calls++
		for pcm := range chunks {
			frames++
			if pcm[0] != byte(frames) {
				t.Error("audio reordered")
			}
		}
		return ctx.Err()
	}}
	if err := local.Speak(context.Background(), protocol.Message{Text: text}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || frames != len(want) {
		t.Fatalf("playback calls = %d, audio frames = %d; want 1, %d", calls, frames, len(want))
	}
}
