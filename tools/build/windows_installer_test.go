package main

import (
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		record := nsisEscape("F" + strings.ReplaceAll(name, "/", `\`))
		if strings.Count(script, record) != 1 {
			t.Errorf("inventory missing file: %s", name)
		}
	}
	for _, expected := range []string{
		`"UninstallString" '"$INSTDIR\Uninstall.exe"'`,
		`ReadINIStr $0 "$INSTDIR\uninstall-files.ini" "inventory" "$InventoryIndex"`,
		`Delete "$InventoryPath"`, `RMDir "$InventoryPath"`,
		`FileWriteUTF16LE /BOM`, `Call un.ValidateInventoryPath`,
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("missing inventory uninstall step: %s", expected)
		}
	}
	if strings.Contains(script, "ExecWait") || strings.Contains(script, "-uninstall") || strings.Contains(script, "-remove-installed-files") {
		t.Fatal("uninstall must not delegate to the updater")
	}
	if strings.Contains(script, "RMDir /r") || strings.Contains(script, "Delete \"$INSTDIR\\*\"") {
		t.Fatal("uninstaller must preserve unrelated user files")
	}
	child := strings.Index(script, nsisEscape(`Dnative\models`))
	parent := strings.Index(script, nsisEscape(`Dnative`)+`$\"`)
	if child < 0 || child >= parent {
		t.Fatal("inventory must remove child directories before parents")
	}
	for _, bad := range []string{"../other.txt", "/other.txt", "a/../../b", "a\\b", "a/*", "a\nb"} {
		_, err := installerScript("setup.exe", map[string]string{"foreverdubbed.exe": "app.exe", bad: "data"})
		if err == nil {
			t.Errorf("accepted invalid install path %q", bad)
		}
	}
}

// Runs on Windows CI, and on Linux when NSIS is installed.
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

// Exercise NSIS's compile-time callback without an Azure account. The stand-in
// validates that NSIS has produced a PE uninstaller and propagates failures.
func TestUninstallerSigningHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell stand-in for pwsh")
	}
	if _, err := exec.LookPath("makensis"); err != nil {
		t.Skip("NSIS/makensis is not installed")
	}
	root := t.TempDir()
	putFile(t, root, "app.exe", "fixture")
	putFile(t, root, "pwsh", "#!/bin/sh\nfor arg; do last=\"$arg\"; done\n[ \"$(head -c 2 \"$last\")\" = MZ ] || exit 9\nprintf called > \"$FDB_TEST_SIGN_LOG\"\nexit \"$FDB_TEST_SIGN_EXIT\"\n")
	if err := os.Chmod(filepath.Join(root, "pwsh"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FDB_TEST_SIGN_LOG", filepath.Join(root, "called"))
	files := map[string]string{"foreverdubbed.exe": filepath.Join(root, "app.exe")}
	for _, code := range []string{"0", "7"} {
		t.Setenv("FDB_TEST_SIGN_EXIT", code)
		output := filepath.Join(root, "setup-"+code+".exe")
		err := writeWindowsInstallerWithSigner(output, files, filepath.Join(root, "sign script.ps1"))
		if code == "0" && err != nil {
			t.Fatal(err)
		}
		if code != "0" {
			if err == nil {
				t.Fatal("ignored uninstaller signing failure")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("published installer after signing failure")
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "called")); err != nil || string(data) != "called" {
		t.Fatalf("uninstaller signer was not called: %s, %v", data, err)
	}
}
