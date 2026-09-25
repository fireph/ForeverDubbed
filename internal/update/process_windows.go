//go:build windows

package update

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os/exec"
	"path/filepath"
	"strings"
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

// Portable copies must never change another installation's Windows metadata.
func recordInstalledVersion(install Installation, version string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\ForeverDubbed`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	defer key.Close()
	location, _, err := key.GetStringValue("InstallLocation")
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(location), filepath.Clean(install.Root)) {
		return nil
	}
	return key.SetStringValue("DisplayVersion", version)
}
