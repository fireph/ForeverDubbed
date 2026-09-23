//go:build darwin && cgo

package platform

import (
	"os"
	"testing"
)

// Opt-in live regression for CLI CoreGraphics initialization. Mock window tests
// cannot reproduce CGS_REQUIRE_INIT inside initWithDesktopIndependentWindow.
// Requires Screen Recording permission and the selected game window to be open.
func TestMacWindowCaptureStartup(t *testing.T) {
	app := os.Getenv("FDB_TEST_CAPTURE_APP")
	if app == "" {
		t.Skip("set FDB_TEST_CAPTURE_APP to run the live macOS window capture test")
	}
	if err := Init(app); err != nil {
		t.Fatal(err)
	}
	bounds := Desktop()
	pixels, err := Capture(bounds)
	if err != nil {
		t.Fatal(err)
	}
	if bounds.Empty() || pixels.Bounds() != bounds {
		t.Fatalf("invalid game capture bounds: requested %v, got %v", bounds, pixels.Bounds())
	}
	// Keep the image only in memory, like the normal reader.
}
