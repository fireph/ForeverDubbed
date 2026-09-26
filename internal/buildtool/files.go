package buildtool

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RegularFile rejects directories and symlinks in release inputs.
func RegularFile(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("bundle source must be a regular file: %s", name)
	}
	return nil
}

// CopyFile copies a build artifact, preserving its executable permission bits.
func CopyFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if existing, err := os.Stat(destination); err == nil && os.SameFile(info, existing) {
		return fmt.Errorf("copy source and destination are the same file: %s", source)
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		// OpenFile applies the mode only when creating a file, and the umask
		// can remove executable bits even then.
		err = out.Chmod(info.Mode().Perm())
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
