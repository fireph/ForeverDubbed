package pocket

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSentenceFadeEnvelope(t *testing.T) {
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Skip("C++ compiler required for native fade envelope test")
	}
	binary := filepath.Join(t.TempDir(), "fade-test")
	if output, err := exec.Command(compiler, "-std=c++17", "-O2", "-I.", "testdata/sentence_fade.cpp", "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile fade test: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("fade envelope: %v\n%s", err, output)
	}
}
