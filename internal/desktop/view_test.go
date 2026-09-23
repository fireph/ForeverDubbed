//go:build gui

package desktop

import (
	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/protocol"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestDashboardLiveStates(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(companionTheme{theme.DefaultTheme()})
	d := newDashboard("0.5.0", func() {}, func() {})
	w := a.NewWindow("ForeverDubbed")
	defer w.Close()
	w.SetContent(d.root)
	w.Resize(fyne.NewSize(840, 660))
	state := appstate.New("World of Warcraft Beta.app", "pocket", false)
	d.render(state.Snapshot())
	if d.headline.Text != "Starting your companion" {
		t.Fatal(d.headline.Text)
	}
	state.Update(func(v *appstate.Snapshot) { v.Ready = true })
	state.Capture(true, false, nil)
	d.render(state.Snapshot())
	if d.window.value.Text != "Detected" || d.tile.value.Text != "Searching" {
		t.Fatal("window and tile states conflated")
	}
	state.Capture(true, true, nil)
	state.Update(func(v *appstate.Snapshot) {
		v.Voice = "orc_male"
		v.Received = time.Date(2026, 9, 23, 14, 32, 8, 0, time.UTC)
		v.Message = protocol.Message{Speaker: "Thrall", Title: "A call to adventure", Text: "Welcome, traveler. There is much to be done, and Azeroth needs your strength. Take a moment to prepare yourself, then meet me at the gates."}
	})
	state.Audio("Playing audio")
	d.render(state.Snapshot())
	if d.audio.value.Text != "Playing audio" || d.speaker.Text != "Thrall" {
		t.Fatal("missing playback/dialogue status")
	}
	if name := os.Getenv("FDB_UI_SCREENSHOT"); name != "" {
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	state.Finished(nil)
	d.render(state.Snapshot())
	if d.window.value.Text != "Stopped" || d.audio.value.Text != "Stopped" {
		t.Fatal("stale running status after stop")
	}
}
