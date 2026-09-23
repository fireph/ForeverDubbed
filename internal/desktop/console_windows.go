//go:build gui && windows

package desktop

import (
	"os"
	"strings"
	"syscall"
)

// Windows releases use the GUI subsystem to avoid a console on double-click.
// Explicit terminal operations attach to the parent's console, preserving any
// redirected stdout/stderr handles supplied by the caller.
func PrepareConsole() {
	terminal := false
	for _, arg := range os.Args[1:] {
		name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		switch name {
		case "headless", "version", "voices", "speak-test", "snapshot", "image", "help", "h":
			terminal = true
		}
	}
	if !terminal {
		return
	}
	stdout, errout := os.Stdout, os.Stderr
	_, outErr := stdout.Stat()
	_, errErr := errout.Stat()
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	attach.Call(^uintptr(0)) // ATTACH_PARENT_PROCESS
	if outErr != nil {
		if h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE); err == nil {
			os.Stdout = os.NewFile(uintptr(h), "stdout")
		}
	}
	if errErr != nil {
		if h, err := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE); err == nil {
			os.Stderr = os.NewFile(uintptr(h), "stderr")
		}
	}
}
