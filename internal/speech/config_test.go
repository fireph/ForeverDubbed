package speech

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile Profile
		wantErr string
	}{
		{name: "defaults", profile: Profile{Voice: "alba"}},
		{name: "minimums", profile: Profile{Voice: "alba", DecodeSteps: 1, GainDB: -24}},
		{name: "maximums", profile: Profile{Voice: "alba", DecodeSteps: 64, GainDB: 12, FadeInMS: 500, FadeOutMS: 500}},
		{name: "custom", profile: Profile{Voice: "custom/voice.safetensors", DecodeSteps: 4, GainDB: 4, FadeInMS: 50, FadeOutMS: 100}},
		{name: "empty voice", wantErr: "needs a Pocket TTS voice"},
		{name: "blank voice", profile: Profile{Voice: " "}, wantErr: "needs a Pocket TTS voice"},
		{name: "backslash path", profile: Profile{Voice: `custom\voice.safetensors`}, wantErr: "forward slashes (/)"},
		{name: "negative steps", profile: Profile{Voice: "alba", DecodeSteps: -1}, wantErr: "decode_steps"},
		{name: "excess steps", profile: Profile{Voice: "alba", DecodeSteps: 65}, wantErr: "decode_steps"},
		{name: "low gain", profile: Profile{Voice: "alba", GainDB: -25}, wantErr: "gain_db"},
		{name: "high gain", profile: Profile{Voice: "alba", GainDB: 13}, wantErr: "gain_db"},
		{name: "negative fade in", profile: Profile{Voice: "alba", FadeInMS: -1}, wantErr: "fade_in_ms"},
		{name: "negative fade out", profile: Profile{Voice: "alba", FadeOutMS: -1}, wantErr: "fade_out_ms"},
		{name: "excess fade in", profile: Profile{Voice: "alba", FadeInMS: 501}, wantErr: "fade_in_ms"},
		{name: "excess fade out", profile: Profile{Voice: "alba", FadeOutMS: 501}, wantErr: "fade_out_ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := Config{
				Default:  map[string]string{"male": "test", "female": "test", "unknown": "test"},
				Profiles: map[string]Profile{"test": tc.profile},
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
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := loaded.Profiles["test"]; got != tc.profile {
				t.Fatalf("profile = %+v, want %+v", got, tc.profile)
			}
		})
	}
}
