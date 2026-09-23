package platform

import (
	"fmt"
	"image"
	"path"
	"strings"
	"sync"
	"time"
)

// Window titles are intentionally not used for identity: unrelated apps can
// show a page or document named "World of Warcraft".
type captureWindow struct {
	ID                      uint64
	PID                     int32
	App, Bundle, Executable string
	Width, Height           float64
	Layer                   int
	OnScreen                bool
}

// Resolve the app bundle from the process's actual executable, not its window
// title or display name. This distinguishes Beta from other installed clients.
func executableBundle(executable string) string {
	if !path.IsAbs(executable) {
		return ""
	}
	for dir := path.Dir(executable); ; dir = path.Dir(dir) {
		if strings.HasSuffix(strings.ToLower(dir), ".app") {
			return dir
		}
		if path.Dir(dir) == dir {
			return ""
		}
	}
}

func matchesCaptureApp(w captureWindow, app string) bool {
	if strings.HasSuffix(strings.ToLower(app), ".exe") {
		// Windows paths must compare identically in Linux-hosted tests too.
		executable := strings.ReplaceAll(w.Executable, "\\", "/")
		selector := strings.ReplaceAll(app, "\\", "/")
		if executable == "" {
			return false
		}
		if strings.Contains(selector, "/") {
			return strings.EqualFold(path.Clean(executable), path.Clean(selector))
		}
		return strings.EqualFold(path.Base(executable), selector)
	}
	bundle := executableBundle(w.Executable)
	if path.IsAbs(app) {
		return path.Clean(app) == w.Executable || (bundle != "" && path.Clean(app) == bundle)
	}
	if strings.HasSuffix(strings.ToLower(app), ".app") {
		return bundle != "" && strings.EqualFold(path.Base(bundle), app)
	}
	return strings.EqualFold(w.App, app) || strings.EqualFold(w.Bundle, app)
}

func chooseCaptureWindow(windows []captureWindow, app string, previous captureWindow) (captureWindow, error) {
	app = strings.TrimSpace(app)
	if app == "" {
		return captureWindow{}, fmt.Errorf("capture application must not be empty")
	}
	var best captureWindow
	for _, w := range windows {
		if !matchesCaptureApp(w, app) {
			continue
		}
		if w.ID == 0 || w.PID <= 0 || !w.OnScreen || w.Layer != 0 || w.Width < 100 || w.Height < 100 {
			continue
		}
		if best.ID != 0 && best.PID != w.PID {
			return captureWindow{}, fmt.Errorf("multiple processes match %q; close the other game instance or use -capture-app with the game's exact executable/app path or bundle identifier", app)
		}
		// Keep the current window if it is still available, so opening another game
		// window doesn't silently switch the capture target.
		if best.ID != 0 && best.ID == previous.ID && best.PID == previous.PID {
			continue
		}
		if best.ID == 0 || (w.ID == previous.ID && w.PID == previous.PID) || w.Width*w.Height > best.Width*best.Height || (w.Width*w.Height == best.Width*best.Height && w.ID < best.ID) {
			best = w
		}
	}
	if best.ID == 0 {
		return captureWindow{}, fmt.Errorf("waiting for a visible game window owned by %q (use -capture-app for a different executable, app bundle name, path, or identifier)", app)
	}
	return best, nil
}

type windowCaptureDriver interface {
	Windows() ([]captureWindow, error)
	Select(captureWindow) (image.Rectangle, error)
	Capture(image.Rectangle) (*image.RGBA, error)
}

// There is deliberately no desktop/display capture method in this driver.
// Enumeration reads window metadata; pixel capture starts only after selection.
type windowCapture struct {
	sync.Mutex
	driver    windowCaptureDriver
	app       string
	selected  captureWindow
	bounds    image.Rectangle
	checkedAt time.Time
	err       error
}

func (c *windowCapture) refresh() {
	c.checkedAt = time.Now()
	windows, err := c.driver.Windows()
	var selected captureWindow
	var bounds image.Rectangle
	if err == nil {
		selected, err = chooseCaptureWindow(windows, c.app, c.selected)
	}
	if err == nil {
		bounds, err = c.driver.Select(selected)
	}
	if err == nil && (bounds.Empty() || int64(bounds.Dx())*int64(bounds.Dy()) > 100_000_000) {
		err = fmt.Errorf("invalid game window bounds %v", bounds)
	}
	c.err = err
	if err != nil {
		if driver, ok := c.driver.(interface{ Reset() }); ok {
			driver.Reset()
		}
		c.selected = captureWindow{}
		c.bounds = image.Rectangle{}
		return
	}
	c.selected, c.bounds = selected, bounds
}

func (c *windowCapture) Bounds() image.Rectangle {
	c.Lock()
	defer c.Unlock()
	c.refresh()
	return c.bounds
}

func (c *windowCapture) Capture(rect image.Rectangle) (*image.RGBA, error) {
	c.Lock()
	defer c.Unlock()
	if time.Since(c.checkedAt) >= time.Second {
		previous, bounds := c.selected, c.bounds
		c.refresh()
		if c.err == nil && (previous.ID != c.selected.ID || previous.PID != c.selected.PID || bounds != c.bounds) {
			return nil, fmt.Errorf("game window changed; searching for the tile again")
		}
	}
	if c.err != nil {
		return nil, c.err
	}
	if rect.Empty() || !rect.In(c.bounds) || int64(rect.Dx())*int64(rect.Dy()) > 100_000_000 {
		return nil, fmt.Errorf("invalid game capture rectangle %v", rect)
	}
	im, err := c.driver.Capture(rect)
	if err != nil {
		// Re-enumerate before capturing again after a closed/replaced window or
		// display/size change. Never substitute a display or unrelated window.
		c.err = err
		c.checkedAt = time.Time{}
	}
	return im, err
}
