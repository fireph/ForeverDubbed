//go:build windows

package update

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"time"
)

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}
func waitProcess(ctx context.Context, pid int) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err == windows.ERROR_INVALID_PARAMETER {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		status, err := windows.WaitForSingleObject(h, uint32((100*time.Millisecond)/time.Millisecond))
		if err != nil {
			return err
		}
		if status == windows.WAIT_OBJECT_0 {
			return nil
		}
		if status != uint32(windows.WAIT_TIMEOUT) {
			return fmt.Errorf("unexpected process wait result %d", status)
		}
	}
}
func verifyMac(oldApp, newApp string) error {
	return fmt.Errorf("macOS updates are not supported on Windows")
}
