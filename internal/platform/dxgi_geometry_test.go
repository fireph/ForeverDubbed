package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDesktopCapturePolicyAndGeometry(t *testing.T) {
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Skip("C++ compiler required for native desktop capture tests")
	}
	binary := filepath.Join(t.TempDir(), "dxgi-geometry-test.exe")
	if output, err := exec.Command(compiler, "-std=c++17", "-O2", "-I.", "testdata/dxgi_geometry.cpp", "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile desktop capture tests: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("desktop capture policy/geometry: %v\n%s", err, output)
	}
}
