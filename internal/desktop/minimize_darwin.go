//go:build gui && darwin && !ci

package desktop

/*
#cgo LDFLAGS: -framework AppKit
#include <stdint.h>
int fdb_gui_minimized(uintptr_t window, int restore);
*/
import "C"
import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

func macMinimized(w fyne.Window, restore bool) (minimized bool) {
	if native, ok := w.(driver.NativeWindow); ok {
		native.RunNative(func(c any) {
			if win, ok := c.(driver.MacWindowContext); ok && win.NSWindow != 0 {
				var r C.int
				if restore {
					r = 1
				}
				minimized = C.fdb_gui_minimized(C.uintptr_t(win.NSWindow), r) != 0
			}
		})
	}
	return
}
func isMinimized(w fyne.Window) bool { return macMinimized(w, false) }
func restoreMinimized(w fyne.Window) { macMinimized(w, true) }
