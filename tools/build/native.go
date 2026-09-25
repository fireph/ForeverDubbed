package main

import (
	"fmt"
	"path/filepath"

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

	for _, asset := range pocket.Assets() {
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
