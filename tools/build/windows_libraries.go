package main

import (
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Library staging validates the runtime architecture and stages Windows loader names.
func addWindowsLibraries(bundle map[string]string, dir string) error {
	const name = "onnxruntime.dll"
	filename := filepath.Join(dir, name)
	image, err := pe.Open(filename)
	if err != nil {
		return fmt.Errorf("need Windows x64 native runtime at %s (prepare it with go run ./tools/native, or pass -native-dir): %w", dir, err)
	}
	machine := image.Machine
	image.Close()
	if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return fmt.Errorf("%s is not a Windows x64 library", filename)
	}
	bundle[name] = filename
	// ONNX Runtime must be beside the executable for Windows loader startup.
	// Include any supplied compiler runtime DLLs, but ignore the old PocketTTS bridge.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && !strings.EqualFold(entry.Name(), "foreverdubbed_tts.dll") && strings.EqualFold(filepath.Ext(entry.Name()), ".dll") {
			bundle[entry.Name()] = filepath.Join(dir, entry.Name())
		}
	}
	return nil
}
