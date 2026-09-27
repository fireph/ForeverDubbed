//go:build gui

package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"foreverdubbed/addon"
	"foreverdubbed/internal/appstate"
	"fyne.io/fyne/v2"
	"github.com/ncruces/zenity"
)

// UI callbacks own path selection; the single worker owns filesystem changes.
func manageAddon(ctx context.Context, a fyne.App, w fyne.Window, state *appstate.State, version string) func() {
	// Migrate old builds that persisted a reload/restart prompt. Files on disk
	// determine startup status; a previous prompt is not evidence of an update.
	a.Preferences().RemoveValue("addonPending")
	paths := make(chan string, 1)
	setBanner := func(text string) { state.Update(func(v *appstate.Snapshot) { v.AddonBanner = text }) }

	pickerOpen := false // Accessed only on the Fyne thread.
	choose := func() {
		if pickerOpen {
			return
		}
		pickerOpen = true
		options := wowPickerOptions(ctx, w, a.Preferences().String("wowExecutable"))
		go func() {
			selected, err := zenity.SelectFile(options...)
			fyne.Do(func() { pickerOpen = false })
			if ctx.Err() != nil || errors.Is(err, zenity.ErrCanceled) {
				return
			}
			if err != nil {
				setBanner(fmt.Sprintf("Could not open the file picker: %v. Click Locate WoW to retry.", err))
				return
			}
			if selected != "" {
				select {
				case paths <- selected:
				case <-ctx.Done():
				}
			}
		}()
	}

	// Always allow retrying a cancelled picker or choosing another installation.
	go func() {
		candidates := append([]string{a.Preferences().String("wowExecutable")}, addon.Candidates(runtime.GOOS, os.Getenv)...)
		path := ""
		for _, candidate := range candidates {
			if _, err := addon.GameDirectory(candidate, runtime.GOOS); err == nil {
				path = candidate
				break
			}
		}
		if path == "" {
			setBanner("Click Locate WoW to select your WoW Forever installation.")
			fyne.Do(choose)
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var installed, gameValid bool
		var pending string
		var observedSession uint32
		var observedVersion string
		attempt := func() {
			installed = false
			dir, err := addon.GameDirectory(path, runtime.GOOS)
			gameValid = err == nil
			if err == nil {
				a.Preferences().SetString("wowExecutable", path)
				state.Update(func(v *appstate.Snapshot) { v.WoWPath = path })
				var prompt string
				prompt, err = addon.Install(dir, version)
				if err == nil {
					installed = true

					pending = prompt
					setBanner(pending)
					if prompt != "" {
						state.Announce(pending)
					}

				}
			}
			if err != nil {
				if errors.Is(err, os.ErrPermission) {
					setBanner(addonPermissionHelp(dir, runtime.GOOS))
				} else {
					setBanner(fmt.Sprintf("Could not install ForeverDubbed: %v. Click Locate WoW to retry.", err))
				}
			}
		}
		if path != "" {
			attempt()
		}
		for {
			select {
			case <-ctx.Done():
				return
			case selected := <-paths:
				path, installed, pending = selected, false, ""
				if _, err := addon.GameDirectory(selected, runtime.GOOS); err == nil {
					state.SetCaptureTarget(selected)
				}
				observedSession, observedVersion = 0, ""
				attempt()
			case <-ticker.C:
				s := state.Snapshot()
				if !gameValid || !s.Tile || s.AddonSession == 0 {
					continue
				}
				// A decoded square proves the addon is already loaded, including
				// legacy versions without version metadata. A fresh-install notice
				// must not prevent us from asking for /reload instead.
				if next := reconcileAddonNotice(pending, s, version); next != pending {
					pending = next
					setBanner(next)
					if next != "" {
						state.Announce(next)
					}
				}
				if addonVersionCurrent(s.AddonVersion, version) {
					if !installed || pending != "" {
						installed = true
						pending = ""
						setBanner("")
					}
				} else if installed && pending == "" && (s.AddonSession != observedSession || s.AddonVersion != observedVersion) {
					// An old addon can remain loaded after files on disk were updated.
					attempt()
					if installed && pending == "" {
						pending = addon.Reload
						setBanner(addon.Reload)
						state.Announce(addon.Reload)
					}
				}
				observedSession, observedVersion = s.AddonSession, s.AddonVersion
			}
		}
	}()
	return choose
}

func addonPermissionHelp(gameDir, platform string) string {
	destination := filepath.Join(gameDir, "Interface", "AddOns")
	instruction := fmt.Sprintf("Copy the bundled ForeverDubbed addon folder here manually:\n%s", destination)
	if platform == "windows" {
		instruction = fmt.Sprintf("Run ForeverDubbed as administrator and retry, or manually copy the bundled ForeverDubbed addon folder here:\n%s", destination)
	}
	return "Permission denied. " + instruction
}

// Reconcile a pending install notice with evidence from the running addon.
func reconcileAddonNotice(notice string, observed appstate.Snapshot, expected string) string {
	if notice == "" || !observed.Tile || observed.AddonSession == 0 {
		return notice
	}
	if addonVersionCurrent(observed.AddonVersion, expected) {
		return ""
	}
	if notice == addon.Restart {
		return addon.Reload
	}
	return notice
}

// The version reported by the square is authoritative. Session IDs deduplicate
// dialogue; they must not impose an additional reload on a current addon.
func addonVersionCurrent(loaded, expected string) bool {
	loaded = strings.TrimPrefix(strings.TrimSpace(loaded), "v")
	expected = strings.TrimPrefix(strings.TrimSpace(expected), "v")
	return loaded != "" && (loaded == expected || addon.Newer(loaded, expected))
}
