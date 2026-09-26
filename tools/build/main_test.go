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
	putFile(t, root, "data/races.go", "package racedata")
	putFile(t, root, "tts/custom/used.safetensors", "voice")
	putFile(t, root, "tts/custom/unused.safetensors", "private")
	putFile(t, root, "tts/custom/draft.pending.safetensors", "unfinished")
	putFile(t, root, ".runtime/secret", "excluded")
	putVoices(t, root, "custom/./used.safetensors", "custom/used.safetensors", "alba")
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
	for _, tc := range []struct{ voice, wantErr string }{
		{"../outside.safetensors", "finished files under tts/custom"},
		{"/outside.safetensors", "finished files under tts/custom"},
		{"C:/outside.safetensors", "finished files under tts/custom"},
		{"custom/../outside.safetensors", "finished files under tts/custom"},
		{`custom\used.safetensors`, "forward slashes (/)"},
		{"custom/draft.pending.safetensors", "finished files under tts/custom"},
		{"custom/missing.safetensors", "voice custom/missing.safetensors"},
		{"custom/reference.wav", "exported .safetensors"},
		{"custom/reference.mp3", "exported .safetensors"},
	} {
		t.Run(tc.voice, func(t *testing.T) {
			root := t.TempDir()
			putFile(t, root, "tts/custom/draft.pending.safetensors", "unfinished")
			putFile(t, root, "tts/custom/reference.wav", "reference audio")
			putFile(t, root, "tts/custom/reference.mp3", "reference audio")
			putFile(t, root, "tts/custom/used.safetensors", "exported voice")
			putVoices(t, root, tc.voice)
			if _, err := voiceFiles(root); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("voiceFiles error = %v, want %q", err, tc.wantErr)
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
	if got := buildEnv(input, "windows", "amd64", false); !reflect.DeepEqual(got, want) {
		t.Fatalf("environment: %v", got)
	}
	want[len(want)-1] = "CGO_ENABLED=1"
	if got := buildEnv(input, "windows", "amd64", true); !reflect.DeepEqual(got, want) {
		t.Fatalf("native environment: %v", got)
	}

	if !reflect.DeepEqual(input, copyOfInput) {
		t.Fatal("mutated caller environment")
	}
}
