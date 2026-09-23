package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func putFile(t *testing.T, root, name, content string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func putVoices(t *testing.T, root string, voices ...string) {
	t.Helper()
	profiles := map[string]map[string]string{}
	for _, voice := range voices {
		profiles[voice] = map[string]string{"voice": voice}
	}
	data, err := json.Marshal(map[string]any{"profiles": profiles})
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, root, "tts/voices.json", string(data))
}

func TestPackageContentsAndLayout(t *testing.T) {
	root := t.TempDir()
	for _, name := range runtimeFiles {
		putFile(t, root, name, name)
	}
	for _, name := range []string{"docs/guide.md", "data/races.json", "addon/ForeverDubbed/ForeverDubbed.toc"} {
		putFile(t, root, name, name)
	}
	putFile(t, root, "tts/custom/used.safetensors", "voice")
	putFile(t, root, "tts/custom/unused.safetensors", "private")
	putFile(t, root, "tts/custom/draft.pending.safetensors", "unfinished")
	putFile(t, root, ".runtime/secret", "excluded")
	putVoices(t, root, `custom\used.safetensors`, "custom/used.safetensors", "alba")
	bundle, addon, err := packageFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle) != len(runtimeFiles)+4 {
		t.Fatalf("unexpected manifest: %v", bundle)
	}
	if _, ok := bundle["tts/voices.json"]; !ok {
		t.Fatal("missing voice config")
	}
	if _, ok := bundle["tts/custom/used.safetensors"]; !ok {
		t.Fatal("missing referenced voice")
	}
	if len(addon) != 1 || addon["ForeverDubbed/ForeverDubbed.toc"] == "" {
		t.Fatalf("bad addon layout: %v", addon)
	}
	destination := filepath.Join(t.TempDir(), "release.zip")
	// Replacement must remove stale ZIP members on subsequent builds.
	if err := writeZIP(destination, map[string]string{"stale.txt": bundle["README.md"]}); err != nil {
		t.Fatal(err)
	}
	if err := writeZIP(destination, bundle); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != len(bundle) {
		t.Fatal("stale or missing ZIP members")
	}
	previous := ""
	for _, file := range archive.File {
		if strings.Contains(file.Name, "\\") || file.Name <= previous {
			t.Fatalf("nonportable or unsorted ZIP path: %s", file.Name)
		}
		previous = file.Name
		source, ok := bundle[file.Name]
		if !ok {
			t.Fatalf("unexpected ZIP member: %s", file.Name)
		}
		want, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || string(got) != string(want) {
			t.Fatalf("ZIP content %s: %v", file.Name, err)
		}
	}
}

func TestRejectInvalidVoiceReferences(t *testing.T) {
	for _, voice := range []string{"../outside.wav", "/outside.wav", `C:\outside.wav`, `custom\..\outside.wav`, "custom/draft.pending.safetensors", "custom/missing.safetensors"} {
		t.Run(voice, func(t *testing.T) {
			root := t.TempDir()
			putFile(t, root, "tts/custom/draft.pending.safetensors", "unfinished")
			putVoices(t, root, voice)
			if _, err := voiceFiles(root); err == nil {
				t.Fatal("accepted invalid voice")
			}
		})
	}
}

func TestRejectVoiceSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	putFile(t, root, "outside.safetensors", "private")
	putFile(t, root, "tts/custom/regular.safetensors", "voice")
	if err := os.Symlink(filepath.Join(root, "outside.safetensors"), filepath.Join(root, "tts", "custom", "linked.safetensors")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	putVoices(t, root, "custom/linked.safetensors")
	if _, err := voiceFiles(root); err == nil {
		t.Fatal("bundled a voice outside custom/")
	}
}

func TestBuildEnvironmentIsolation(t *testing.T) {
	input := []string{"PATH=tools", "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=1", "LUA=lua"}
	copyOfInput := append([]string(nil), input...)
	want := []string{"PATH=tools", "LUA=lua", "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0"}
	if got := buildEnv(input, "windows", "amd64"); !reflect.DeepEqual(got, want) {
		t.Fatalf("environment: %v", got)
	}
	if !reflect.DeepEqual(input, copyOfInput) {
		t.Fatal("mutated caller environment")
	}
}
