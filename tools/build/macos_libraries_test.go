package main

import (
	"debug/macho"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeMachOLibrary(t *testing.T, filename string, cpu macho.Cpu) {
	t.Helper()
	// A minimal little-endian 64-bit dylib header is enough to test architecture
	// validation without storing platform binaries in the repository.
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	words := []uint32{macho.Magic64, uint32(cpu), 0, uint32(macho.TypeDylib), 0, 0, 0, 0}
	if err := binary.Write(f, binary.LittleEndian, words); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMacLibraryArchitectureAndAliases(t *testing.T) {
	dir := t.TempDir()
	library := filepath.Join(dir, "libonnxruntime.dylib")
	writeMachOLibrary(t, library, macho.CpuArm64)
	if err := checkMachO(library, "amd64"); err == nil {
		t.Fatal("accepted ARM library for Intel release")
	}
	versioned := filepath.Join(dir, "libonnxruntime.1.23.2.dylib")
	writeMachOLibrary(t, versioned, macho.CpuArm64)
	bundle := map[string]string{}
	if err := addMacLibraries(bundle, dir, "arm64"); err != nil {
		t.Fatal(err)
	}
	if bundle["native/libonnxruntime.dylib"] != library || bundle["native/libonnxruntime.1.23.2.dylib"] != versioned {
		t.Fatalf("lost loader names: %v", bundle)
	}
	if err := os.WriteFile(library, []byte("not a library"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := addMacLibraries(map[string]string{}, dir, "arm64"); err == nil {
		t.Fatal("accepted invalid dylib")
	}
}

func TestThinMacLibraries(t *testing.T) {
	dir := t.TempDir()
	arm := filepath.Join(dir, "arm.dylib")
	intel := filepath.Join(dir, "intel.dylib")
	writeMachOLibrary(t, arm, macho.CpuArm64)
	writeMachOLibrary(t, intel, macho.CpuAmd64)
	armBytes, _ := os.ReadFile(arm)
	intelBytes, _ := os.ReadFile(intel)
	// Two aligned slices in a universal Mach-O file.
	source := filepath.Join(dir, "universal.dylib")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	words := []uint32{macho.MagicFat, 2,
		uint32(macho.CpuArm64), 0, 4096, uint32(len(armBytes)), 12,
		uint32(macho.CpuAmd64), 0, 8192, uint32(len(intelBytes)), 12}
	if err := binary.Write(f, binary.BigEndian, words); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(armBytes, 4096); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(intelBytes, 8192); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"arm64", "amd64"} {
		t.Run(arch, func(t *testing.T) {
			bundle := map[string]string{"native/libonnxruntime.dylib": source, "native/libonnxruntime.1.dylib": source}
			if err := thinMacLibraries(bundle, t.TempDir(), arch); err != nil {
				t.Fatal(err)
			}
			for _, filename := range bundle {
				if filename == source {
					t.Fatal("universal input retained")
				}
				if err := checkMachO(filename, arch); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(filename)
				if err != nil || info.Size() != 32 {
					t.Fatalf("unexpected slice size: %v %v", info, err)
				}
			}
			if err := thinMacLibraries(bundle, t.TempDir(), arch); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := macho.OpenFat(source); err != nil {
		t.Fatalf("source was modified: %v", err)
	}
}
