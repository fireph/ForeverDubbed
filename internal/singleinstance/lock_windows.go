package singleinstance

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lock(f *os.File) error {
	// Keep the same byte offset across versions so all copies share one lock.
	ov := windows.Overlapped{Offset: 4096}
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ov)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errLocked
	}
	return err
}
