//go:build gui

package desktop

import (
	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/game"
	"fyne.io/fyne/v2"
)

// gameSelection owns validation, persistence and capture selection. It does
// not install addons or open dialogs; callers decide how to report failures.
type gameSelection struct {
	preferences fyne.Preferences
	state       *appstate.State
	platform    string
}

// discover restores a valid saved path before trying the usual locations.
// An empty result tells the caller to offer the picker.
func (g gameSelection) discover(candidates []string) string {
	paths := append([]string{g.preferences.String("wowExecutable")}, candidates...)
	for _, path := range paths {
		if _, err := g.selectPath(path, false); err == nil {
			return path
		}
	}
	return ""
}

// Explicit CLI selectors win at startup. A manual picker selection can replace
// them for this run. Invalid selections leave the current installation intact.
func (g gameSelection) selectPath(path string, manual bool) (string, error) {
	dir, err := game.Directory(path, g.platform)
	if err != nil {
		return "", err
	}
	g.preferences.SetString("wowExecutable", path)
	g.state.Update(func(s *appstate.Snapshot) { s.WoWPath = path })
	s := g.state.Snapshot()
	if (manual || !s.CaptureTargetExplicit) && s.Target != path {
		g.state.SetCaptureTarget(path)
	}
	return dir, nil
}
