//go:build gui && windows && !ci

package desktop

import (
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var isIconic = user32.NewProc("IsIconic")
var showWindow = user32.NewProc("ShowWindow")

func isMinimized(w fyne.Window) (minimized bool) {
	if native, ok := w.(driver.NativeWindow); ok {
		native.RunNative(func(c any) {
			if win, ok := c.(driver.WindowsWindowContext); ok && win.HWND != 0 {
				result, _, _ := isIconic.Call(win.HWND)
				minimized = result != 0
			}
		})
	}
	return
}
func restoreMinimized(w fyne.Window) {
	if native, ok := w.(driver.NativeWindow); ok {
		native.RunNative(func(c any) {
			if win, ok := c.(driver.WindowsWindowContext); ok && win.HWND != 0 {
				result, _, _ := isIconic.Call(win.HWND)
				if result != 0 {
					showWindow.Call(win.HWND, 9)
				}
			}
		})
	}
}
