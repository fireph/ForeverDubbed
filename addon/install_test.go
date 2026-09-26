package addon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallUpdateRepairAndPreserve(t *testing.T) {
	root := t.TempDir()
	check := func(version, want string) {
		t.Helper()
		got, err := Install(root, version)
		if err != nil || got != want {
			t.Fatalf("Install(%s) = %q, %v; want %q", version, got, err, want)
		}
	}
	check("0.8.3", Restart)
	dir := filepath.Join(root, "Interface", "AddOns", "ForeverDubbed")
	toc, _ := os.ReadFile(filepath.Join(dir, "ForeverDubbed.toc"))
	if tocVersion(toc) != "0.8.3" {
		t.Fatal("incorrect stamped version")
	}
	check("0.8.3", "")
	custom := filepath.Join(dir, "personal.txt")
	if err := os.WriteFile(custom, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}
	check("0.8.4", Reload)
	check("0.8.3", "") // Never downgrade.
	if data, _ := os.ReadFile(custom); string(data) != "keep me" {
		t.Fatal("lost custom file")
	}
	if err := os.Remove(filepath.Join(dir, "Codec.lua")); err != nil {
		t.Fatal(err)
	}
	check("0.8.4", Reload)
	check("0.8.4", "")
}

func TestGameDirectory(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "WowB.exe")
	if err := os.WriteFile(exe, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if dir, err := GameDirectory(exe, "windows"); err != nil || dir != root {
		t.Fatalf("%s %v", dir, err)
	}
	app := filepath.Join(root, "World of Warcraft Beta.app")
	if err := os.Mkdir(app, 0755); err != nil {
		t.Fatal(err)
	}
	if dir, err := GameDirectory(app, "darwin"); err != nil || dir != root {
		t.Fatalf("%s %v", dir, err)
	}
	if _, err := GameDirectory(root, "windows"); err == nil {
		t.Fatal("accepted arbitrary folder")
	}
	if _, err := GameDirectory(filepath.Join(root, "missing.exe"), "windows"); err == nil {
		t.Fatal("accepted missing file")
	}
}

func TestFailedInstallLeavesExistingAddon(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, "0.8.3"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "Interface", "AddOns", "ForeverDubbed")
	// An unsupported file in the existing addon makes staging fail safely.
	if err := os.Symlink("ForeverDubbed.toc", filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	if _, err := Install(root, "0.8.4"); err == nil {
		t.Fatal("accepted symlink")
	}
	toc, err := os.ReadFile(filepath.Join(dir, "ForeverDubbed.toc"))
	if err != nil || tocVersion(toc) != "0.8.3" {
		t.Fatalf("old install damaged: %v", err)
	}
}

func TestExistingAddonWithoutTOCNeedsReload(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Interface", "AddOns", "ForeverDubbed")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ForeverDubbed.lua"), []byte("-- old addon"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Install(root, "0.8.3")
	if err != nil || got != Reload {
		t.Fatalf("existing addon repair: %q, %v; want reload", got, err)
	}
}
