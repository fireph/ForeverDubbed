package speech

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFadeConfig(t *testing.T) {
	for _, fades := range [][2]int{{0, 0}, {50, 100}, {500, 500}, {-1, 0}, {0, -1}, {501, 0}, {0, 501}} {
		config := Config{
			Default:  map[string]string{"male": "test", "female": "test", "unknown": "test"},
			Profiles: map[string]Profile{"test": {Voice: "test", FadeInMS: fades[0], FadeOutMS: fades[1]}},
		}
		data, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "voices.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		valid := fades[0] >= 0 && fades[0] <= 500 && fades[1] >= 0 && fades[1] <= 500
		if (err == nil) != valid {
			t.Fatalf("fades %v: %v", fades, err)
		}
		if valid && got.Profiles["test"] != config.Profiles["test"] {
			t.Fatal("fade settings lost")
		}
	}
}
