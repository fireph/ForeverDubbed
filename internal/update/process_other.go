//go:build !windows

package update

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func waitProcess(ctx context.Context, pid int) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func verifyMac(oldApp, newApp string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/codesign", "-d", "-r-", oldApp).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot read installed app signature: %w", err)
	}
	requirement, err := designatedRequirement(string(output))
	if err != nil {
		return err
	}
	output, err = exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", "-R="+requirement, newApp).CombinedOutput()
	if err != nil {
		return fmt.Errorf("update signature verification failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func designatedRequirement(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		if requirement, ok := strings.CutPrefix(line, "designated => "); ok && strings.TrimSpace(requirement) != "" {
			return requirement, nil
		}
	}
	return "", fmt.Errorf("installed app has no signing identity")
}
