package main

import (
	"fmt"
	"os"
	"os/exec"
)

func runWindowsSigner(script string, args ...string) error {
	command := append([]string{"-NoProfile", "-NonInteractive", "-File", script}, args...)
	cmd := exec.Command("pwsh", command...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Windows signing failed: %w", err)
	}
	return nil
}

// Sign before manifest hashing and ZIP/installer creation. Never replace
// third-party DLL publishers with our own identity.
func signWindowsPayload(files map[string]string, sign func(string) error) error {
	if sign == nil {
		return nil
	}
	for _, name := range []string{"foreverdubbed.exe", "foreverdubbed-updater.exe"} {
		filename := files[name]
		if filename == "" {
			return fmt.Errorf("missing signing input %s", name)
		}
		if err := sign(filename); err != nil {
			return err
		}
	}
	return nil
}
