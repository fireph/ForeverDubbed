package addon

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"foreverdubbed/internal/releaseversion"
)

// Files travel inside every desktop executable, including portable builds.
//
//go:embed ForeverDubbed/*
var Files embed.FS

const Reload = "Type slash reload to update Forever Dubbed addon"
const Restart = "Restart World of Warcraft to install Forever Dubbed addon"

func tocVersion(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "## Version:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Install stages a complete copy next to the destination before replacing it.
// Unknown files are preserved; a failed swap restores the previous directory.
// The returned prompt is empty when the installed files already match.
func Install(gameDir, version string) (string, error) {
	if _, err := releaseversion.Parse(version); err != nil {
		return "", err
	}
	parent := filepath.Join(gameDir, "Interface", "AddOns")
	destination := filepath.Join(parent, "ForeverDubbed")
	old, err := os.ReadFile(filepath.Join(destination, "ForeverDubbed.toc"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if releaseversion.Newer(tocVersion(old), version) {
		return "", nil
	}
	contents := map[string][]byte{}
	entries, err := fs.ReadDir(Files, "ForeverDubbed")
	if err != nil {
		return "", err
	}
	changed := false
	for _, entry := range entries {
		if entry.IsDir() {
			return "", fmt.Errorf("unexpected addon directory %s", entry.Name())
		}
		data, err := Files.ReadFile("ForeverDubbed/" + entry.Name())
		if err != nil {
			return "", err
		}
		if entry.Name() == "ForeverDubbed.toc" {
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if strings.HasPrefix(line, "## Version:") {
					lines[i] = "## Version: " + version
				}
			}
			data = []byte(strings.Join(lines, "\n"))
		}
		contents[entry.Name()] = data
		current, err := os.ReadFile(filepath.Join(destination, entry.Name()))
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if !bytes.Equal(current, data) {
			changed = true
		}
	}
	if !changed {
		return "", nil
	}
	if err := os.MkdirAll(parent, 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(parent, ".foreverdubbed-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	exists := false
	if info, err := os.Lstat(destination); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("addon destination must be a directory")
		}
		exists = true
		if err := filepath.WalkDir(destination, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("addon contains a symbolic link: %s", path)
			}
			return nil
		}); err != nil {
			return "", err
		}
		if err := os.CopyFS(stage, os.DirFS(destination)); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	for name, data := range contents {
		if err := os.WriteFile(filepath.Join(stage, name), data, 0644); err != nil {
			return "", err
		}
	}
	backup := stage + "-previous"
	if exists {
		if err := os.Rename(destination, backup); err != nil {
			return "", err
		}
	}
	if err := os.Rename(stage, destination); err != nil {
		if exists {
			if restore := os.Rename(backup, destination); restore != nil {
				return "", fmt.Errorf("install: %v; restore: %v (backup: %s)", err, restore, backup)
			}
		}
		return "", err
	}
	if exists {
		_ = os.RemoveAll(backup)
	}
	if !exists {
		return Restart, nil
	}
	return Reload, nil
}
