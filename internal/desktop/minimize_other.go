//go:build gui && ((!windows && !darwin) || ci)

package desktop

import "fyne.io/fyne/v2"

func isMinimized(fyne.Window) bool { return false }
func restoreMinimized(fyne.Window) {}
