package main

import (
	"debug/macho"
	"fmt"
	"io"
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

// Extract the target slice before manifest hashing and app signing. Keep every
// loader alias as a regular file, as required by the update archive format.
func thinMacLibraries(bundle map[string]string, staging, arch string) error {
	want := macho.CpuArm64
	if arch == "amd64" {
		want = macho.CpuAmd64
	}
	for name, source := range bundle {
		if filepath.Ext(name) != ".dylib" {
			continue
		}
		if err := checkMachO(source, arch); err != nil {
			return err
		}
		fat, err := macho.OpenFat(source)
		if err == macho.ErrNotFat {
			continue
		}
		if err != nil {
			return err
		}
		var offset, size int64
		for _, slice := range fat.Arches {
			if slice.Cpu == want {
				offset, size = int64(slice.Offset), int64(slice.Size)
				break
			}
		}
		fat.Close()
		input, err := os.Open(source)
		if err != nil {
			return err
		}
		info, err := input.Stat()
		if err != nil {
			input.Close()
			return err
		}
		destination := filepath.Join(staging, filepath.Base(name))
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.CopyN(output, io.NewSectionReader(input, offset, size), size)
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := checkMachO(destination, arch); err != nil {
			return err
		}
		bundle[name] = destination
	}
	return nil
}
