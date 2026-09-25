package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"foreverdubbed/internal/buildinfo"
)

// Stamp a staged copy, keeping the source addon directly usable and avoiding
// tracked-file changes when a release tag overrides the local source version.
func stageAddon(dist string, bundle, addon map[string]string) error {
	const toc = "ForeverDubbed/ForeverDubbed.toc"
	if _, ok := addon[toc]; !ok {
		return fmt.Errorf("addon is missing %s", toc)
	}
	stage, err := os.MkdirTemp(dist, ".foreverdubbed-addon-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for name, source := range addon {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if name == toc {
			lines := strings.SplitAfter(string(data), "\n")
			count := 0
			for i, line := range lines {
				if !strings.HasPrefix(line, "## Version:") {
					continue
				}
				ending := ""
				if strings.HasSuffix(line, "\r\n") {
					ending = "\r\n"
				} else if strings.HasSuffix(line, "\n") {
					ending = "\n"
				}
				lines[i] = "## Version: " + buildinfo.Version + ending
				count++
			}
			if count != 1 {
				return fmt.Errorf("addon TOC must contain exactly one Version field")
			}
			data = []byte(strings.Join(lines, ""))
		}
		target := filepath.Join(stage, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(target, data, 0644); err != nil {
			return err
		}
	}
	destination := filepath.Join(dist, "addon")
	if err = os.RemoveAll(destination); err != nil {
		return err
	}
	if err = os.Rename(stage, destination); err != nil {
		return err
	}
	for name := range addon {
		source := filepath.Join(destination, filepath.FromSlash(name))
		addon[name] = source
		bundle["addon/"+name] = source
	}
	return nil
}
