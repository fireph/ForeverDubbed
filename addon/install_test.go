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
	for _, tc := range []struct {
		name, client, platform string
		directories, files     []string
		bundle, wantValid      bool
	}{
		{name: "default Windows", client: "WowB.exe", platform: "windows", directories: []string{"Data"}, wantValid: true},
		{name: "renamed Windows", client: "CustomClient.EXE", platform: "windows", directories: []string{"Interface", "WTF"}, wantValid: true},
		{name: "fresh shared data", client: "_classic_beta_/Custom.exe", platform: "windows", directories: []string{"Data"}, files: []string{".build.info"}, wantValid: true},
		{name: "default Mac", client: "World of Warcraft Beta.app", platform: "darwin", directories: []string{"Data"}, bundle: true, wantValid: true},
		{name: "renamed Mac", client: "_classic_beta_/Custom WoW.app", platform: "darwin", directories: []string{"Data"}, files: []string{".build.info"}, bundle: true, wantValid: true},
		{name: "unrelated executable", client: "Other.exe", platform: "windows"},
		{name: "expected name outside install", client: "WowB.exe", platform: "windows"},
		{name: "unrelated Mac app", client: "Other.app", platform: "darwin", bundle: true},
		{name: "wrong extension", client: "Wow.txt", platform: "windows", directories: []string{"Data"}},
		{name: "exe directory", client: "Wow.exe", platform: "windows", directories: []string{"Data"}, bundle: true},
		{name: "app file", client: "WoW.app", platform: "darwin", directories: []string{"Data"}},
		{name: "data file", client: "Wow.exe", platform: "windows", files: []string{"Data"}},
		{name: "incomplete shared layout", client: "_classic_beta_/Wow.exe", platform: "windows", directories: []string{"Data"}},
		{name: "metadata directory", client: "_classic_beta_/Wow.exe", platform: "windows", directories: []string{"Data", ".build.info"}},
		{name: "unsupported platform", client: "Wow.exe", platform: "linux", directories: []string{"Data"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			client := filepath.Join(root, tc.client)
			mkdir := func(path string) {
				t.Helper()
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path string) {
				t.Helper()
				if err := os.WriteFile(path, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			mkdir(filepath.Dir(client))
			for _, path := range tc.directories {
				mkdir(filepath.Join(root, path))
			}
			for _, path := range tc.files {
				write(filepath.Join(root, path))
			}
			if tc.bundle {
				mkdir(client)
			} else {
				write(client)
			}
			dir, err := GameDirectory(client, tc.platform)
			if tc.wantValid {
				if err != nil || dir != filepath.Dir(client) {
					t.Fatalf("GameDirectory = %q, %v", dir, err)
				}
			} else if err == nil {
				t.Fatalf("accepted invalid client: %s", client)
			}
			if _, err := GameDirectory(filepath.Join(root, "missing.exe"), "windows"); err == nil {
				t.Fatal("accepted missing file")
			}
		})
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
