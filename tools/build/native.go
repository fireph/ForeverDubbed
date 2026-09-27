package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"foreverdubbed/internal/buildtool"
	"foreverdubbed/internal/pocket"
)

func addTargetNativeFiles(bundle map[string]string, dir string, target buildtool.Target) error {
	if target.OS == "darwin" {
		if err := addMacLibraries(bundle, dir, target.Arch); err != nil {
			return err
		}
	} else if err := addWindowsLibraries(bundle, dir); err != nil {
		return err
	}

	assets, err := releaseAssets(bundle["tts/voices.json"])
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if err := pocket.VerifyAsset(dir, asset); err != nil {
			return fmt.Errorf("native assets: %w", err)
		}
		bundle["native/"+asset.Path] = filepath.Join(dir, filepath.FromSlash(asset.Path))
	}
	for _, name := range []string{"PocketTTS.cpp.txt", "ONNX-Runtime.txt", "SentencePiece.txt", "nlohmann-json.txt", "dr_libs.txt"} {
		source := filepath.Join(dir, "licenses", name)
		if err := buildtool.RegularFile(source); err != nil {
			return err
		}
		bundle["native/licenses/"+name] = source
	}
	return nil
}

// Models are always required; presets are shipped only when configured.
func releaseAssets(configPath string) ([]pocket.Asset, error) {
	profiles, err := voiceProfiles(configPath)
	if err != nil {
		return nil, err
	}
	presets := map[string]bool{}
	for _, profile := range profiles {
		voice := profile.Voice
		if filepath.Ext(voice) != "" {
			continue
		}
		if voice == "" || voice == "." || voice == ".." || strings.ContainsAny(voice, "/\\:") {
			return nil, fmt.Errorf("invalid preset %q", voice)
		}
		presets["presets/"+voice+".safetensors"] = true
	}
	var selected []pocket.Asset
	for _, asset := range pocket.Assets() {
		if strings.HasPrefix(asset.Path, "presets/") && !presets[asset.Path] {
			continue
		}
		selected = append(selected, asset)
		delete(presets, asset.Path)
	}
	for name := range presets {
		return nil, fmt.Errorf("unknown bundled preset %q", name)
	}
	return selected, nil
}
