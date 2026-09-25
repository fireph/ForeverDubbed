package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/update"
)

func releaseVersion() (string, error) {
	version := buildinfo.Version
	if os.Getenv("GITHUB_REF_TYPE") == "tag" {
		tag := os.Getenv("GITHUB_REF_NAME")
		if !strings.HasPrefix(tag, "v") {
			return "", fmt.Errorf("release tag must start with v")
		}
		version = strings.TrimPrefix(tag, "v")
	}
	if _, err := update.Version(version); err != nil {
		return "", err
	}
	return version, nil
}
func addReleaseManifest(dist string, files map[string]string) error {
	manifest := update.Manifest{Version: buildinfo.Version, Files: map[string]string{}}
	for name, source := range files {
		f, err := os.Open(source)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		manifest.Files[name] = hex.EncodeToString(h.Sum(nil))
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(dist, update.ManifestName)
	if err = os.WriteFile(target, append(data, '\n'), 0644); err != nil {
		return err
	}
	files[update.ManifestName] = target
	return nil
}

func releaseZIPName(targetOS, arch string) string {
	if targetOS == "windows" {
		return "ForeverDubbed-windows-" + arch + "-portable.zip"
	}
	return "ForeverDubbed-mac-" + arch + ".zip"
}
