//go:build gui

package desktop

import (
	"context"
	"time"

	"foreverdubbed/internal/appicon"
	"foreverdubbed/internal/appstate"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
)

// Run keeps Fyne's event loop on the main goroutine; capture and synthesis run
// in the worker. Status is copied under a lock and rendered only via fyne.Do.
func Run(ctx context.Context, stop context.CancelFunc, state *appstate.State, version string, races []string, work func() error) error {
	app.SetMetadata(fyne.AppMetadata{ID: "io.foreverdubbed.companion", Name: "ForeverDubbed", Version: version, Migrations: map[string]bool{"fyneDo": true}})
	a := app.NewWithID("io.foreverdubbed.companion")
	a.SetIcon(Icon)
	a.Settings().SetTheme(companionTheme{theme.DefaultTheme()})
	w := a.NewWindow("ForeverDubbed")
	w.Resize(fyne.NewSize(760, 800))
	w.CenterOnScreen()
	quit := func() { stop() }
	show := func() { restoreMinimized(w); w.Show(); w.RequestFocus() }
	tray, hasTray := a.(desktop.App)
	hide := func() {
		if hasTray {
			w.Hide()
		}
	}
	state.SetQueueSpeech(a.Preferences().Bool("queueSpeech"))
	state.SetSpeechFilters(loadSpeechFilters(a.Preferences()))
	state.SetVoiceChoices(loadVoiceChoices(a.Preferences()))
	d := newDashboard(version, hide, quit, state.StopAudio, func(enabled bool) {
		state.SetQueueSpeech(enabled)
		a.Preferences().SetBool("queueSpeech", enabled)
	}, func(filters appstate.SpeechFilters) {
		state.SetSpeechFilters(filters)
		saveSpeechFilters(a.Preferences(), filters)
	}, races, state.VoiceChoices(), func(choices map[string]string) {
		state.SetVoiceChoices(choices)
		saveVoiceChoices(a.Preferences(), choices)
	})
	w.SetContent(d.root)
	d.render(state.Snapshot())
	if hasTray {
		exit := fyne.NewMenuItem("Quit", quit)
		exit.IsQuit = true
		tray.SetSystemTrayIcon(Icon)
		tray.SetSystemTrayMenu(fyne.NewMenu("ForeverDubbed", fyne.NewMenuItem("Show ForeverDubbed", show), fyne.NewMenuItemSeparator(), exit))
		w.SetCloseIntercept(hide)
	} else {
		w.SetCloseIntercept(quit)
	}
	// OS-level Quit (including Cmd+Q) must cancel the worker too.
	a.Lifecycle().SetOnStopped(stop)
	done := make(chan struct{})
	var workErr error
	go func() {
		defer close(done)
		workErr = work()
		state.Finished(workErr)
	}()
	uiDone := make(chan struct{})
	defer close(uiDone)
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		var previous appstate.Snapshot
		for {
			select {
			case <-uiDone:
				return
			case <-ctx.Done():
				// Release capture and drain/cancel speech before shutting down the UI.
				<-done
				fyne.Do(a.Quit)
				return
			case <-ticker.C:
				current := state.Snapshot()
				changed := current != previous
				previous = current
				fyne.Do(func() {
					if hasTray && isMinimized(w) {
						w.Hide()
					}
					if changed {
						d.render(current)
					}
				})
			}
		}
	}()
	checkForUpdates(ctx, version, w, stop)
	w.ShowAndRun()
	stop()
	<-done
	return workErr
}

var Icon = fyne.NewStaticResource("foreverdubbed.png", appicon.PNG(256))
var Banner = fyne.NewStaticResource("forever-dubbed-banner.png", appicon.Banner)
