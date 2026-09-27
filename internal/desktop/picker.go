//go:build gui

package desktop

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"github.com/ncruces/zenity"
)

func wowPickerOptions(ctx context.Context, w fyne.Window, previous string) []zenity.Option {
	options := []zenity.Option{zenity.Context(ctx), zenity.Title("Locate WoW Forever — select your WoW executable"),
		zenity.FileFilter{Name: "WoW Forever (WowB.exe)", Patterns: []string{"WowB.exe"}, CaseFold: true},
		zenity.FileFilter{Name: "All executables (*.exe)", Patterns: []string{"*.exe"}, CaseFold: true}}
	if runtime.GOOS == "darwin" {
		options = []zenity.Option{zenity.Context(ctx), zenity.Title("Locate WoW Forever — select your WoW application"),
			zenity.FileFilter{Name: "Applications", Patterns: []string{"*.app"}}}
		// chooseFile treats application bundles as files, not navigable folders.
	} else if native, ok := w.(driver.NativeWindow); ok {
		native.RunNative(func(context any) {
			if win, ok := context.(driver.WindowsWindowContext); ok && win.HWND != 0 {
				options = append(options, zenity.Attach(win.HWND), zenity.Modal())
			}
		})
	}
	if previous != "" {
		// Start outside the .app bundle even though it is a directory on disk.
		directory := filepath.Dir(previous)
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			options = append(options, zenity.Filename(directory+string(os.PathSeparator)))
		}
	} else if runtime.GOOS == "darwin" {
		options = append(options, zenity.Filename("/Applications/"))
	}
	return options
}
