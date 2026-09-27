//go:build gui

package desktop

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/speech"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// Opt-in capture of the real desktop widgets with reproducible example state.
// This needs neither a running game nor the native speech/capture backends.
func TestWebsiteScreenshots(t *testing.T) {
	directory := os.Getenv("FDB_WEBSITE_SCREENSHOTS_DIR")
	if directory == "" {
		t.Skip("set FDB_WEBSITE_SCREENSHOTS_DIR to refresh the website screenshots")
	}
	config, err := speech.Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	var races []string
	for race := range config.Races {
		races = append(races, race)
	}
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(companionTheme{theme.DefaultTheme()})
	state := appstate.New("WowB.exe", "pocket", false)
	state.SetQueueSpeech(true)
	state.Update(func(s *appstate.Snapshot) {
		s.Ready, s.Window, s.Tile = true, true, true
		s.Audio = "Idle"
		s.AddonVersion = buildinfo.Version
		s.WoWPath = `C:\Program Files (x86)\World of Warcraft\_classic_beta_\WowB.exe`
		s.DetectedExecutable = s.WoWPath
	})
	d := newDashboard(buildinfo.Version, func() {}, func() {}, state.StopAudio, state.SkipAudio,
		state.SetQueueSpeech, state.SetSpeechFilters, races, nil, state.SetVoiceChoices)
	w := a.NewWindow("ForeverDubbed")
	defer w.Close()
	w.SetContent(d.root)
	w.Resize(defaultWindowSize)
	d.render(state.Snapshot())
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	capture := func(name string) {
		t.Helper()
		file, err := os.Create(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		encodeErr := png.Encode(file, w.Canvas().Capture())
		closeErr := file.Close()
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	capture("settings.png")
	d.tabs.SelectIndex(1)
	capture("voices.png")
	d.tabs.SelectIndex(2)
	capture("about.png")
}
