package buildtool

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFileReplacesContentsAndPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permissions")
	}
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	for _, mode := range []os.FileMode{0755, 0644} {
		if err := os.WriteFile(source, []byte("new"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(source, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte("stale longer contents"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := CopyFile(source, destination); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(destination)
		if err != nil || string(got) != "new" {
			t.Fatalf("copied contents = %q, error = %v", got, err)
		}
		info, err := os.Stat(destination)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("copied permissions = %o, want %o", info.Mode().Perm(), mode)
		}
	}
}

func TestCopyFileRejectsSameFileWithoutTruncating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library")
	if err := os.WriteFile(path, []byte("library contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(path, path); err == nil {
		t.Fatal("accepted copying a file onto itself")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "library contents" {
		t.Fatalf("source changed: %q, error = %v", got, err)
	}
}
