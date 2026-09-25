package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"foreverdubbed/internal/pocket"
	"foreverdubbed/internal/protocol"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func pcmSamples(samples ...int16) []byte {
	pcm := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(pcm[2*i:], uint16(sample))
	}
	return pcm
}

func TestVoicePlaybackGain(t *testing.T) {
	for _, tc := range []struct {
		name string
		db   float64
		want []byte
	}{
		{"default", 0, pcmSamples(0, 1000, -1000, 20000, -20000)},
		{"boost", 20 * math.Log10(2), pcmSamples(0, 2000, -2000, 32767, -32768)},
		{"reduce", -20 * math.Log10(2), pcmSamples(0, 500, -500, 10000, -10000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := Load("../../tts/voices.json")
			if err != nil {
				t.Fatal(err)
			}
			profile := config.Profiles["tauren_male"]
			profile.GainDB = tc.db
			config.Profiles["tauren_male"] = profile
			input := pcmSamples(0, 1000, -1000, 20000, -20000)
			original := bytes.Clone(input)
			local := Local{Config: config, Engine: fakeEngine{func(_ context.Context, _, _ string, _ pocket.StreamOptions, emit func([]byte) error) error {
				return emit(input)
			}}}
			var got []byte
			local.Play = func(_ context.Context, rate int, chunks <-chan []byte) error {
				if rate != 24000 {
					t.Errorf("sample rate changed: %d", rate)
				}
				for chunk := range chunks {
					got = append(got, chunk...)
				}
				return nil
			}
			if err := local.Speak(context.Background(), protocol.Message{Race: "Tauren", Gender: "male", Text: "Welcome."}); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("playback %v, want %v", got, tc.want)
			}
			if !bytes.Equal(input, original) {
				t.Fatal("modified engine buffer")
			}
		})
	}
}

func TestProfileGainValidation(t *testing.T) {
	for _, db := range []float64{-25, -24, 0, 4, 12, 13} {
		config := Config{
			Default:  map[string]string{"male": "test", "female": "test", "unknown": "test"},
			Profiles: map[string]Profile{"test": {Voice: "test", GainDB: db}},
		}
		data, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "voices.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(path)
		valid := db >= -24 && db <= 12
		if (err == nil) != valid {
			t.Fatalf("gain %v: %v", db, err)
		}
		if valid && loaded.Profiles["test"].GainDB != db {
			t.Fatal("gain lost loading config")
		}
	}
}
