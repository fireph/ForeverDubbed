package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"foreverdubbed/internal/buildtool"
)

// Maps ZIP names (always slash-separated) to source files. No staging tree or
// shell-specific archiver is needed, and no unreferenced voice files are copied.
func packageFiles(root string) (map[string]string, map[string]string, error) {
	bundle, addon := map[string]string{}, map[string]string{}
	for _, name := range runtimeFiles {
		source := filepath.Join(root, filepath.FromSlash(name))
		if err := buildtool.RegularFile(source); err != nil {
			return nil, nil, err
		}
		bundle[name] = source
	}
	for _, dir := range []string{"docs", "data", "addon/ForeverDubbed"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(source string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if dir == "data" && strings.HasSuffix(entry.Name(), ".go") {
				return nil // Embed declarations are build source, not runtime data.
			}
			if err := buildtool.RegularFile(source); err != nil {
				return err
			}
			relative, err := filepath.Rel(root, source)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(relative)
			bundle[name] = source
			if strings.HasPrefix(name, "addon/") {
				addon[strings.TrimPrefix(name, "addon/")] = source
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	voices, err := voiceFiles(root)
	if err != nil {
		return nil, nil, err
	}
	for name, source := range voices {
		bundle[name] = source
	}
	return bundle, addon, nil
}

func voiceFiles(root string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "tts", "voices.json"))
	if err != nil {
		return nil, err
	}
	var config struct {
		Profiles map[string]struct {
			Voice string `json:"voice"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	files := map[string]string{}
	custom := filepath.Join(root, "tts", "custom")
	for _, profile := range config.Profiles {
		// Parse Windows-style references the same way on every build host.
		name := path.Clean(strings.ReplaceAll(profile.Voice, "\\", "/"))
		ext := strings.ToLower(path.Ext(name))
		if ext != ".safetensors" && ext != ".wav" {
			continue
		}
		if !strings.HasPrefix(name, "custom/") || strings.Contains(name, ":") || strings.HasSuffix(strings.ToLower(name), ".pending.safetensors") {
			return nil, fmt.Errorf("bundle voice references must be finished files under tts/custom: %s", profile.Voice)
		}
		source := filepath.Join(root, "tts", filepath.FromSlash(name))
		// Resolve symlinks too: a lexical path inside custom/ can point outside.
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return nil, fmt.Errorf("voice %s: %w", profile.Voice, err)
		}
		realCustom, err := filepath.EvalSymlinks(custom)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(realCustom, resolved)
		if err != nil || !filepath.IsLocal(relative) {
			return nil, fmt.Errorf("voice escapes tts/custom: %s", profile.Voice)
		}
		if err := buildtool.RegularFile(resolved); err != nil {
			return nil, err
		}
		files["tts/"+name] = resolved
	}
	return files, nil
}
