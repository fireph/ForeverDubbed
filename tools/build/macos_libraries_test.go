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
