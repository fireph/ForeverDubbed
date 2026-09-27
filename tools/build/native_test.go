package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseAssetsSelectReferencedPresets(t *testing.T) {
	for _, tc := range []struct {
		name    string
		voices  []string
		presets int
		invalid bool
	}{
		{"custom only", []string{"custom/narrator.safetensors"}, 0, false},
		{"mixed", []string{"custom/narrator.safetensors", "anna", "mary"}, 2, false},
		{"unknown", []string{"missing"}, 0, true},
		{"traversal", []string{"../anna"}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			putVoices(t, root, tc.voices...)
			assets, err := releaseAssets(filepath.Join(root, "tts", "voices.json"))
			if tc.invalid {
				if err == nil {
					t.Fatal("accepted invalid preset")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			models, presets := 0, 0
			for _, asset := range assets {
				if strings.HasPrefix(asset.Path, "models/") {
					models++
				} else {
					presets++
					if asset.Path != "presets/anna.safetensors" && asset.Path != "presets/mary.safetensors" {
						t.Fatalf("unexpected preset: %s", asset.Path)
					}
				}
			}
			if models != 7 || presets != tc.presets {
				t.Fatalf("models=%d presets=%d", models, presets)
			}
		})
	}
}
