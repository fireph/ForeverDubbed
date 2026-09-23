//go:build windows && cgo

package platform

import (
	"os"
	"testing"
)

// The real D3D/WinRT pipeline requires a Windows desktop and a running game.
func TestWindowsGameWindowCapture(t *testing.T) {
	app := os.Getenv("FDB_TEST_CAPTURE_APP")
	if app == "" {
		t.Skip("set FDB_TEST_CAPTURE_APP=WoWB.exe with the game open to test native capture")
	}
	if err := Init(app); err != nil {
		t.Fatal(err)
	}
	defer CloseCapture()
	bounds := Desktop()
	pixels, err := Capture(bounds)
	if err != nil {
		t.Fatal(err)
	}
	if bounds.Empty() || pixels.Bounds() != bounds {
		t.Fatalf("invalid window capture: %v", bounds)
	}
	region := bounds.Inset(10)
	tile, err := Capture(region)
	if err != nil {
		t.Fatal(err)
	}
	if tile.Bounds() != region {
		t.Fatalf("incorrect crop: %v", tile.Bounds())
	}
}
