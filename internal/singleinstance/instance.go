// Package singleinstance keeps one companion running per user, across install paths.
package singleinstance

import (
	"errors"
	"os"
	"path/filepath"
)

var errLocked = errors.New("instance lock is held")

// Instance holds an OS lock until Close or process exit, including a crash.
type Instance struct {
	file *os.File
}

// Start returns nil, nil when another instance is already running.
func Start() (*Instance, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return start(filepath.Join(dir, "ForeverDubbed", "instance.lock"))
}

func start(path string) (*Instance, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lock(f); err != nil {
		f.Close()
		if errors.Is(err, errLocked) {
			return nil, nil
		}
		return nil, err
	}
	return &Instance{file: f}, nil
}

func (i *Instance) Close() {
	// Keep the file: removing it would let another process lock a different inode.
	i.file.Close()
}
