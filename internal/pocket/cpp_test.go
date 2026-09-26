package pocket

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSentenceFadeEnvelope(t *testing.T) {
	runCPPTest(t, "sentence_fade")
}

func TestSpeechRules(t *testing.T) {
	runCPPTest(t, "speech_rules")
}

func runCPPTest(t *testing.T, name string) {
	t.Helper()
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Skip("C++ compiler required for native unit tests")
	}
	binary := filepath.Join(t.TempDir(), name+"-test")
	if output, err := exec.Command(compiler, "-std=c++17", "-O2", "-DPOCKET_TTS_HELPERS_ONLY", "-I.", filepath.Join("testdata", name+".cpp"), "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile %s: %v\n%s", name, err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, output)
	}
}
