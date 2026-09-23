package platform

import (
	"errors"
	"image"
	"testing"
	"time"
)

func gameWindow(id uint32, pid int32) captureWindow {
	return captureWindow{ID: uint64(id), PID: pid, App: "World of Warcraft", Bundle: "com.blizzard.worldofwarcraft", Executable: "/Applications/World of Warcraft/_classic_beta_/World of Warcraft Beta.app/Contents/MacOS/World of Warcraft", Width: 1280, Height: 720, OnScreen: true}
}

func TestChooseCaptureWindow(t *testing.T) {
	game := gameWindow(1, 42)
	browser := captureWindow{ID: 2, PID: 43, App: "Safari", Bundle: "com.apple.Safari", Width: 3840, Height: 2160, OnScreen: true}
	hidden := game
	hidden.OnScreen = false
	popup := game
	popup.Layer = 1
	small := game
	small.Width = 30
	otherGame := gameWindow(3, 44)
	for _, tc := range []struct {
		name, app string
		windows   []captureWindow
		want      uint64
	}{
		{"only game owner", "World of Warcraft", []captureWindow{browser, game}, 1},
		{"exact bundle", "com.blizzard.worldofwarcraft", []captureWindow{game}, 1},
		{"case insensitive", "world of warcraft", []captureWindow{game}, 1},
		{"no title or desktop fallback", "World of Warcraft", []captureWindow{browser}, 0},
		{"no substring match", "Warcraft", []captureWindow{game}, 0},
		{"closed", "World of Warcraft", nil, 0},
		{"minimized", "World of Warcraft", []captureWindow{hidden}, 0},
		{"popup", "World of Warcraft", []captureWindow{popup}, 0},
		{"tiny window", "World of Warcraft", []captureWindow{small}, 0},
		{"ambiguous processes", "World of Warcraft", []captureWindow{game, otherGame}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := chooseCaptureWindow(tc.windows, tc.app, captureWindow{})
			if got.ID != tc.want || (err == nil) != (tc.want != 0) {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestChooseWindowKeepsCurrentWithinProcess(t *testing.T) {
	small, big := gameWindow(1, 42), gameWindow(2, 42)
	big.Width *= 2
	for _, windows := range [][]captureWindow{{small, big}, {big, small}} {
		got, err := chooseCaptureWindow(windows, small.App, captureWindow{})
		if err != nil || got.ID != big.ID {
			t.Fatalf("expected largest game window: %+v, %v", got, err)
		}
		got, err = chooseCaptureWindow(windows, small.App, small)
		if err != nil || got.ID != small.ID {
			t.Fatalf("switched away from current window: %+v, %v", got, err)
		}
	}
}

type fakeWindowDriver struct {
	resets                   int
	windows                  []captureWindow
	bounds                   image.Rectangle
	discoveryErr, captureErr error
	selected                 captureWindow
	captures                 []image.Rectangle
}

func (d *fakeWindowDriver) Reset() { d.resets++ }

func (d *fakeWindowDriver) Windows() ([]captureWindow, error) { return d.windows, d.discoveryErr }
func (d *fakeWindowDriver) Select(w captureWindow) (image.Rectangle, error) {
	d.selected = w
	return d.bounds, nil
}
func (d *fakeWindowDriver) Capture(r image.Rectangle) (*image.RGBA, error) {
	d.captures = append(d.captures, r)
	if d.captureErr != nil {
		return nil, d.captureErr
	}
	return image.NewRGBA(r), nil
}
func expireWindowCheck(c *windowCapture) { c.checkedAt = time.Now().Add(-2 * time.Second) }

func TestWindowCaptureWaitsForGameAndReopens(t *testing.T) {
	d := &fakeWindowDriver{bounds: image.Rect(0, 0, 1280, 720)}
	c := &windowCapture{app: "World of Warcraft", driver: d}
	if bounds := c.Bounds(); !bounds.Empty() {
		t.Fatal("missing game has a capture surface")
	}
	if _, err := c.Capture(image.Rect(0, 0, 100, 100)); err == nil {
		t.Fatal("captured without a game")
	}
	if len(d.captures) != 0 {
		t.Fatal("requested pixels before game selection")
	}
	d.windows = []captureWindow{gameWindow(1, 42)}
	bounds := c.Bounds()
	if _, err := c.Capture(bounds); err != nil {
		t.Fatal(err)
	}
	tile := image.Rect(120, 80, 240, 160)
	if im, err := c.Capture(tile); err != nil || im.Bounds() != tile {
		t.Fatalf("tile coordinates: %v, %v", im, err)
	}
	// Closing the game must never turn the old rectangle into desktop capture.
	d.windows = nil
	expireWindowCheck(c)
	if _, err := c.Capture(tile); err == nil {
		t.Fatal("captured a closed game")
	}
	if len(d.captures) != 2 {
		t.Fatal("captured after game closed")
	}
	if d.resets < 2 {
		t.Fatal("did not release the native session when the game disappeared")
	}
	d.windows = []captureWindow{gameWindow(3, 55)}
	bounds = c.Bounds()
	if _, err := c.Capture(bounds); err != nil {
		t.Fatal(err)
	}
	if d.selected.ID != 3 || d.selected.PID != 55 {
		t.Fatal("did not reacquire reopened game")
	}
}

func TestWindowCaptureInvalidatesChangedGeometryAndErrors(t *testing.T) {
	d := &fakeWindowDriver{windows: []captureWindow{gameWindow(1, 42)}, bounds: image.Rect(0, 0, 1280, 720)}
	c := &windowCapture{app: "World of Warcraft", driver: d}
	old := c.Bounds()
	d.bounds = image.Rect(0, 0, 1920, 1080)
	expireWindowCheck(c)
	if _, err := c.Capture(old); err == nil {
		t.Fatal("used stale window geometry")
	}
	if len(d.captures) != 0 {
		t.Fatal("captured stale geometry")
	}
	if _, err := c.Capture(c.Bounds()); err != nil {
		t.Fatal(err)
	}
	d.discoveryErr = errors.New("permission revoked")
	expireWindowCheck(c)
	if _, err := c.Capture(old); !errors.Is(err, d.discoveryErr) {
		t.Fatal(err)
	}
	if len(d.captures) != 1 {
		t.Fatal("captured after discovery failed")
	}
	d.discoveryErr = nil
	bounds := c.Bounds()
	if _, err := c.Capture(image.Rect(-1, 0, 10, 10)); err == nil {
		t.Fatal("accepted out-of-window rectangle")
	}
	d.captureErr = errors.New("window disappeared during screenshot")
	if _, err := c.Capture(bounds); !errors.Is(err, d.captureErr) {
		t.Fatal(err)
	}
	d.windows = nil
	if _, err := c.Capture(bounds); err == nil {
		t.Fatal("retried stale window after capture error")
	}
	if len(d.captures) != 2 {
		t.Fatal("captured after game disappeared")
	}
}

func TestCaptureAppUsesRealBetaBundle(t *testing.T) {
	beta := gameWindow(1, 42)
	retail := gameWindow(2, 43)
	retail.Executable = "/Applications/World of Warcraft/_retail_/World of Warcraft.app/Contents/MacOS/World of Warcraft"
	spoof := gameWindow(3, 44)
	spoof.App = "World of Warcraft Beta.app"
	spoof.Executable = "/Applications/Safari.app/Contents/MacOS/Safari"
	for _, selector := range []string{
		"World of Warcraft Beta.app",
		"/Applications/World of Warcraft/_classic_beta_/World of Warcraft Beta.app",
		beta.Executable,
	} {
		got, err := chooseCaptureWindow([]captureWindow{retail, spoof, beta}, selector, captureWindow{})
		if err != nil || got.ID != beta.ID {
			t.Fatalf("%s: %+v, %v", selector, got, err)
		}
		if _, err := chooseCaptureWindow([]captureWindow{retail, spoof}, selector, captureWindow{}); err == nil {
			t.Fatalf("%s: fell back to another app", selector)
		}
	}
	beta.Executable = ""
	if _, err := chooseCaptureWindow([]captureWindow{beta}, "World of Warcraft Beta.app", captureWindow{}); err == nil {
		t.Fatal("matched app bundle without executable identity")
	}
	if _, err := chooseCaptureWindow([]captureWindow{beta}, "", captureWindow{}); err == nil {
		t.Fatal("accepted empty selector")
	}
}

func TestWindowsExecutableSelection(t *testing.T) {
	game := captureWindow{ID: 0x123456789, PID: 100, Executable: `C:\Games\World of Warcraft\_classic_beta_\WoWB.exe`, Width: 1920, Height: 1080, OnScreen: true}
	retail := game
	retail.ID, retail.PID, retail.Executable = 2, 200, `C:\Games\World of Warcraft\_retail_\WoW.exe`
	browser := game
	browser.ID, browser.PID, browser.App, browser.Executable = 3, 300, "WoWB.exe", `C:\Browser\browser.exe`
	for _, selector := range []string{"WoWB.exe", "wowb.EXE", game.Executable, "c:/games/World of Warcraft/_classic_beta_/wowb.exe"} {
		got, err := chooseCaptureWindow([]captureWindow{retail, browser, game}, selector, captureWindow{})
		if err != nil || got.ID != game.ID {
			t.Fatalf("%s: %+v, %v", selector, got, err)
		}
		if _, err := chooseCaptureWindow([]captureWindow{retail, browser}, selector, captureWindow{}); err == nil {
			t.Fatalf("%s: matched another executable", selector)
		}
	}
	second := game
	second.ID, second.PID, second.Executable = 4, 400, `D:\Other WoW\WoWB.exe`
	if _, err := chooseCaptureWindow([]captureWindow{game, second}, "WoWB.exe", captureWindow{}); err == nil {
		t.Fatal("accepted ambiguous executable instances")
	}
	got, err := chooseCaptureWindow([]captureWindow{second, game}, game.Executable, captureWindow{})
	if err != nil || got.ID != game.ID {
		t.Fatalf("full executable path did not disambiguate: %+v, %v", got, err)
	}
}
