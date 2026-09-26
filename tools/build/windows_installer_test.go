package main

import (
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"foreverdubbed/internal/buildinfo"
)

func TestInstallerManifest(t *testing.T) {
	files := map[string]string{
		"foreverdubbed.exe":            "app.exe",
		"native/models/model.onnx":     "model.onnx",
		"tts/custom/voice.safetensors": "voice.safetensors",
		"docs/a $name.txt":             "notice.txt",
	}
	script, err := installerScript("setup.exe", files)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`VIProductVersion "` + buildinfo.Version + `.0"`, `"DisplayVersion" "` + buildinfo.Version + `"`} {
		if !strings.Contains(script, expected) {
			t.Fatalf("missing installer version: %s", expected)
		}
	}
	if strings.Count(script, `  File "/oname=`) != len(files) {
		t.Fatal("installer must include every manifest file exactly once")
	}
	for name := range files {
		remove := `Delete "$INSTDIR\` + nsisEscape(strings.ReplaceAll(name, "/", `\`)) + `"`
		if strings.Count(script, remove) != 1 {
			t.Errorf("uninstaller missing explicit path: %s", name)
		}
	}
	if strings.Contains(script, "RMDir /r") || strings.Contains(script, "Delete \"$INSTDIR\\*\"") {
		t.Fatal("uninstaller must not remove unrelated user files")
	}
	child := strings.Index(script, `RMDir "$INSTDIR\native\models"`)
	parent := strings.Index(script, `RMDir "$INSTDIR\native"`)
	if child < 0 || child >= parent {
		t.Fatal("uninstaller must remove children before parents")
	}
	for _, bad := range []string{"../other.txt", "/other.txt", "a/../../b", "a\\b", "a/*", "a\nb"} {
		_, err := installerScript("setup.exe", map[string]string{"foreverdubbed.exe": "app.exe", bad: "data"})
		if err == nil {
			t.Errorf("accepted invalid install path %q", bad)
		}
	}
}

// Runs in the Windows CI build on Linux, where NSIS is installed before go test.
func TestWindowsInstallerCompile(t *testing.T) {
	if _, err := exec.LookPath("makensis"); err != nil {
		t.Skip("NSIS/makensis is not installed")
	}
	root := t.TempDir()
	files := map[string]string{}
	for _, name := range []string{"foreverdubbed.exe", "native/models/model.onnx", "tts/voices.json", "docs/read me.txt"} {
		destination := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte("installer fixture: "+name), 0644); err != nil {
			t.Fatal(err)
		}
		files[name] = filepath.Join(root, filepath.FromSlash(name))
	}
	output := filepath.Join(root, "test-setup.exe")
	if err := writeWindowsInstaller(output, files); err != nil {
		t.Fatal(err)
	}
	installer, err := pe.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer installer.Close()
	if installer.FileHeader.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE == 0 {
		t.Fatal("installer is not an executable PE image")
	}
}
