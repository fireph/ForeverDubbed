package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRejectsConflictingParentsBeforeExtraction(t *testing.T) {
	for _, files := range []map[string]string{
		{"Data/first": "a", "data/second": "b"},
		{"data": "file", "data/child": "child"},
		{"data/": "", "DATA/child": "child"},
	} {
		root := t.TempDir()
		archive, destination := filepath.Join(root, "update.zip"), filepath.Join(root, "payload")
		if err := os.WriteFile(archive, makeZIP(t, files), 0600); err != nil {
			t.Fatal(err)
		}
		if err := extract(t.Context(), archive, destination, nil); err == nil {
			t.Errorf("accepted conflicting archive paths: %v", files)
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Errorf("archive wrote files before rejecting conflicting paths: %v", files)
		}
	}
}

func TestManifestRejectsConflictingPaths(t *testing.T) {
	for _, names := range [][]string{
		{"voice", "VOICE"},
		{"Data/first", "data/second"},
		{"data", "data/child"},
		{ManifestName},
	} {
		manifest := Manifest{Version: "1.0.0", Files: map[string]string{}}
		for _, name := range names {
			manifest.Files[name] = digest([]byte("content"))
		}
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		filename := filepath.Join(t.TempDir(), ManifestName)
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readManifest(filename); err == nil {
			t.Errorf("accepted conflicting manifest paths: %v", names)
		}
	}
}

func TestArchiveAllowsExplicitDirectories(t *testing.T) {
	root := t.TempDir()
	archive, destination := filepath.Join(root, "update.zip"), filepath.Join(root, "payload")
	files := map[string]string{"data/": "", "data/nested/": "", "data/nested/voice": "voice"}
	if err := os.WriteFile(archive, makeZIP(t, files), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extract(t.Context(), archive, destination, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(destination, "data", "nested", "voice"))
	if err != nil || string(got) != "voice" {
		t.Fatalf("extracted contents = %q, error = %v", got, err)
	}
}
