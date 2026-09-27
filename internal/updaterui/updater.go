// Package updaterui keeps the standalone update helper independent of Fyne.
package updaterui

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"foreverdubbed/internal/update"
	"github.com/ncruces/zenity"
)

type progressDialog interface {
	Text(string) error
	Value(int) error
	Close() error
	Done() <-chan struct{}
}

type operations struct {
	ready   func(string) error
	wait    func(context.Context, update.Plan) error
	apply   func(string, update.Reporter) error
	restart func(string) error
}

// Run shows native progress while the existing update engine replaces the app.
// It returns only after replacement/rollback and closes progress before the
// caller displays an error. Closing the dialog never cancels file operations.
func Run(planPath string) error {
	plan, err := update.LoadPlan(planPath)
	if err != nil {
		return err
	}
	if plan.Install.OS != runtime.GOOS {
		return fmt.Errorf("update target does not match this operating system")
	}
	options := dialogOptions("Updating ForeverDubbed")
	options = append(options, zenity.NoCancel(), zenity.MaxValue(100))
	dialog, err := zenity.Progress(options...)
	if err != nil {
		return fmt.Errorf("open update progress: %w", err)
	}
	return run(planPath, plan, dialog, operations{update.Ready, update.WaitForApp, update.Apply, update.Restart})
}

func run(planPath string, plan update.Plan, dialog progressDialog, ops operations) error {
	defer dialog.Close()
	// Check that the window started before telling the parent app to quit.
	if err := dialog.Text("Preparing update…"); err != nil {
		return fmt.Errorf("show update progress: %w", err)
	}
	select {
	case <-dialog.Done():
		return fmt.Errorf("update progress closed before startup")
	default:
	}
	if err := ops.ready(planPath); err != nil {
		return err
	}
	hidden := false
	progress := func(p update.Progress) {
		// Some platforms allow closing a progress dialog even with NoCancel.
		// Once ready, the helper must finish or roll back regardless of UI state.
		if hidden {
			return
		}
		select {
		case <-dialog.Done():
			hidden = true
			return
		default:
		}
		if err := dialog.Text(p.Stage); err != nil {
			log.Printf("Update progress unavailable; continuing update: %v", err)
			hidden = true
			return
		}
		value := 0
		if p.Total > 0 {
			value = int(100 * float64(p.Done) / float64(p.Total))
			// Keep the native OK button disabled until the helper closes the window.
			value = max(0, min(99, value))
		}
		if err := dialog.Value(value); err != nil {
			log.Printf("Update progress unavailable; continuing update: %v", err)
			hidden = true
		}
	}
	progress(update.Progress{Stage: "Waiting for ForeverDubbed to close…"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := ops.wait(ctx, plan); err != nil {
		return fmt.Errorf("ForeverDubbed did not close: %w", err)
	}
	if err := ops.apply(planPath, progress); err != nil {
		return err
	}
	progress(update.Progress{Stage: "Restarting ForeverDubbed…", Done: 1, Total: 1})
	return ops.restart(planPath)
}

// ShowError uses the same application icon as the progress window.
func ShowError(message string) error {
	return zenity.Error(message, dialogOptions("ForeverDubbed update failed")...)
}

func dialogOptions(title string) []zenity.Option {
	return append([]zenity.Option{zenity.Title(title)}, windowIconOptions()...)
}
