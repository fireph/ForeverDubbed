//go:build gui

package desktop

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/update"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The opaque frame covers the popup background Fyne draws behind the content.
// The body takes the stretch area of a border layout so wrapped text can never
// displace the heading or the action row when the dialog size is fixed.
func updateDialog(title string, body fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	top := container.NewVBox(heading, questRule())
	var bottom fyne.CanvasObject
	if len(actions) > 0 {
		row := make([]fyne.CanvasObject, 0, len(actions)+1)
		row = append(row, layout.NewSpacer())
		row = append(row, actions...)
		bottom = container.NewHBox(row...)
	}
	return questFrame(parchment(container.NewBorder(top, bottom, nil, nil, inset(5, body))))
}

type updateProgress struct {
	root  fyne.CanvasObject
	label *widget.Label
	bar   *widget.ProgressBar
}

func newUpdateProgress() *updateProgress {
	// Stage lines stay single-line so the label keeps a constant minimum
	// height in fixed-size dialogs.
	label := widget.NewLabel("Preparing update…")
	bar := widget.NewProgressBar()
	return &updateProgress{inset(5, container.NewVBox(label, bar)), label, bar}
}
func (v *updateProgress) render(p update.Progress) {
	text := p.Stage
	if p.Total > 0 {
		v.bar.SetValue(float64(p.Done) / float64(p.Total))
		if p.Stage == "Downloading update" {
			text = fmt.Sprintf("%s: %.1f / %.1f MB", p.Stage, float64(p.Done)/(1<<20), float64(p.Total)/(1<<20))
		}
	} else {
		v.bar.SetValue(0)
	}
	v.label.SetText(text)
}

func checkForUpdates(ctx context.Context, version string, w fyne.Window, stop context.CancelFunc) {
	install, err := update.Locate()
	if err != nil {
		log.Printf("Update check skipped: %v", err)
		return
	}
	update.CleanupCompleted(install)
	client := update.NewClient()
	go func() {
		checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		release, err := client.Check(checkCtx, version, install.OS, install.Arch)
		if err != nil {
			log.Printf("Update check: %v", err)
			return
		}
		if release == nil || ctx.Err() != nil {
			return
		}
		fyne.Do(func() {
			if ctx.Err() != nil {
				return
			}
			prompt := updatePrompt(w, release.Tag, version, func() {
				downloadUpdate(ctx, w, stop, client, release, install)
			})
			prompt.Show()
		})
	}()
}
func downloadUpdate(ctx context.Context, w fyne.Window, stop context.CancelFunc, client *update.Client, release *update.Release, install update.Installation) {
	downloadCtx, cancel := context.WithCancel(ctx)
	view := newUpdateProgress()
	progress := widget.NewModalPopUp(container.NewVBox(), w.Canvas())
	cancelButton := widget.NewButton("Cancel", func() { cancel(); progress.Hide() })
	progress.Content = updateDialog("Updating ForeverDubbed", view.root, questButtonWidget(cancelButton))
	progress.Resize(fyne.NewSize(460, 240))
	progress.Show()
	go func() {
		defer cancel()
		path, err := update.Prepare(downloadCtx, client, release, install, func(p update.Progress) {
			fyne.Do(func() {
				if downloadCtx.Err() == nil {
					view.render(p)
				}
			})
		})
		if err != nil {
			if downloadCtx.Err() == nil {
				fyne.Do(func() {
					progress.Hide()
					showUpdateError(fmt.Errorf("The update could not be prepared. Your current installation has not changed.\n\n%w", err), w)
				})
			}
			return
		}
		if downloadCtx.Err() != nil {
			os.RemoveAll(filepath.Dir(path))
			return
		}
		fyne.DoAndWait(func() { cancelButton.Disable(); view.render(update.Progress{Stage: "Starting installer…"}) })
		if err = update.Launch(downloadCtx, path); err != nil {
			fyne.Do(func() { progress.Hide(); showUpdateError(err, w) })
			return
		}
		// The helper now owns the staged update. It keeps progress visible while
		// waiting for this process to release its executable, runtime DLLs and audio.
		stop()
	}()
}

// RunUpdater runs in a separate, self-contained executable without PocketTTS
// DLL dependencies, so it can replace the full app after the parent exits.
func RunUpdater(planPath string) error {
	plan, err := update.LoadPlan(planPath)
	if err != nil {
		return err
	}
	if plan.Install.OS != runtime.GOOS {
		return fmt.Errorf("update target does not match this operating system")
	}
	app.SetMetadata(fyne.AppMetadata{ID: "io.foreverdubbed.updater", Name: "ForeverDubbed updater", Version: buildinfo.Version})
	a := app.NewWithID("io.foreverdubbed.updater")
	a.SetIcon(Icon)
	a.Settings().SetTheme(companionTheme{theme.DefaultTheme()})
	w := a.NewWindow("Updating ForeverDubbed")
	view := newUpdateProgress()
	w.SetContent(updateDialog("Updating ForeverDubbed", view.root))
	w.Resize(fyne.NewSize(480, 240))
	w.CenterOnScreen()
	// Replacement must finish or roll back before this window can be closed.
	w.SetCloseIntercept(func() {})
	w.Show()
	go func() {
		progress := func(p update.Progress) { fyne.Do(func() { view.render(p) }) }
		run := func() error {
			if err := update.Ready(planPath); err != nil {
				return err
			}
			progress(update.Progress{Stage: "Waiting for ForeverDubbed to close…"})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := update.WaitForApp(ctx, plan); err != nil {
				return fmt.Errorf("ForeverDubbed did not close: %w", err)
			}
			if err := update.Apply(planPath, progress); err != nil {
				return err
			}
			progress(update.Progress{Stage: "Restarting ForeverDubbed…", Done: 1, Total: 1})
			return update.Restart(planPath)
		}
		if err := run(); err != nil {
			log.Printf("Update failed: %v", err)
			fyne.Do(func() {
				view.label.SetText("The update could not finish.")
				w.SetCloseIntercept(a.Quit)
				message := widget.NewLabel(fmt.Sprintf("%v\n\nUpdate files and logs: %s", err, filepath.Dir(planPath)))
				message.Wrapping = fyne.TextWrapWord
				w.SetContent(updateDialog("Update failed", message, questButton("Close", a.Quit)))
				w.Resize(fyne.NewSize(560, 340))
			})
			return
		}
		fyne.Do(a.Quit)
	}()
	w.ShowAndRun()
	return nil
}

func updatePrompt(w fyne.Window, available, current string, accept func()) *widget.PopUp {
	text := widget.NewLabel(fmt.Sprintf("ForeverDubbed %s is available (you have %s).\n\nPress OK to download and install it. The app will restart when the update is ready.", available, current))
	text.Wrapping = fyne.TextWrapWord
	popup := widget.NewModalPopUp(container.NewVBox(), w.Canvas())
	popup.Content = updateDialog("Update available", text,
		questButton("Later", popup.Hide),
		questButton("OK", func() { popup.Hide(); accept() }))
	popup.Resize(fyne.NewSize(460, 280))
	return popup
}
func showUpdateError(err error, w fyne.Window) {
	label := widget.NewLabel(err.Error())
	label.Wrapping = fyne.TextWrapWord
	popup := widget.NewModalPopUp(container.NewVBox(), w.Canvas())
	popup.Content = updateDialog("Update failed", label, questButton("OK", popup.Hide))
	popup.Resize(fyne.NewSize(500, 300))
	popup.Show()
}
