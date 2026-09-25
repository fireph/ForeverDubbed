package main

import (
	"debug/macho"
	"fmt"
	"os"
	"path/filepath"
)

// Library staging validates the runtime architecture and includes every dylib alias.
// Go's ZIP writer dereferences dylib symlinks, so include every loader name.
func addMacLibraries(bundle map[string]string, dir, arch string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.dylib"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "libonnxruntime.dylib")); err != nil {
		return fmt.Errorf("macOS runtime: %w (prepare with tools/native -target darwin)", err)
	}
	for _, source := range matches {
		if err := checkMachO(source, arch); err != nil {
			return err
		}
		bundle["native/"+filepath.Base(source)] = source
	}
	return nil
}
func checkMachO(filename, arch string) error {
	want := macho.CpuArm64
	if arch == "amd64" {
		want = macho.CpuAmd64
	}
	fat, err := macho.OpenFat(filename)
	if err == nil {
		defer fat.Close()
		for _, image := range fat.Arches {
			if image.Cpu == want {
				return nil
			}
		}
	} else {
		image, err := macho.Open(filename)
		if err != nil {
			return fmt.Errorf("macOS library %s: %w", filename, err)
		}
		defer image.Close()
		if image.Cpu == want {
			return nil
		}
	}
	return fmt.Errorf("%s does not contain macOS %s code", filename, arch)
}
