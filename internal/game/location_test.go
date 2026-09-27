package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectory(t *testing.T) {
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
			dir, err := Directory(client, tc.platform)
			if tc.wantValid {
				if err != nil || dir != filepath.Dir(client) {
					t.Fatalf("Directory = %q, %v", dir, err)
				}
			} else if err == nil {
				t.Fatalf("accepted invalid client: %s", client)
			}
			if _, err := Directory(filepath.Join(root, "missing.exe"), "windows"); err == nil {
				t.Fatal("accepted missing file")
			}
		})
	}
}
