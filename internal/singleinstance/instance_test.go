package singleinstance

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestExclusionAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	first, err := start(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := start(path)
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		second.Close()
		t.Fatal("second instance acquired lock")
	}
	first.Close()
	next, err := start(path)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("lock not released")
	}
	next.Close()
}

func TestProcessHelper(t *testing.T) {
	path := os.Getenv("FDB_INSTANCE_TEST_PATH")
	if path == "" {
		return
	}
	inst, err := start(path)
	if err != nil {
		t.Fatal(err)
	}
	if inst == nil {
		fmt.Println("already running")
		return
	}
	defer inst.Close()
	fmt.Println("ready")
	// Let the parent kill this process with its lock still held.
	time.Sleep(time.Minute)
}

func TestCrossProcessExclusionAndCrashRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := func() *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessHelper$")
		cmd.Env = append(os.Environ(), "FDB_INSTANCE_TEST_PATH="+path)
		cmd.Stderr = os.Stderr
		return cmd
	}
	owner := command()
	output, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { owner.Process.Kill(); owner.Wait() }()
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatal("owner failed to acquire lock")
	}
	contender := command()
	result, err := contender.Output()
	if err != nil {
		t.Fatalf("contender: %v: %s", err, result)
	}
	if string(result) != "already running\nPASS\n" {
		t.Fatalf("unexpected contender output: %s", result)
	}
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	owner.Wait()
	recovered, err := start(path)
	if err != nil {
		t.Fatal(err)
	}
	if recovered == nil {
		t.Fatal("crashed owner kept lock")
	}
	recovered.Close()
}

func TestExistingLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	if err := os.WriteFile(path, []byte("leftover lock file"), 0600); err != nil {
		t.Fatal(err)
	}
	inst, err := start(path)
	if err != nil {
		t.Fatal(err)
	}
	if inst == nil {
		t.Fatal("leftover lock file blocked startup")
	}
	inst.Close()
}
