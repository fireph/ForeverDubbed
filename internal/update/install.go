package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Plan struct {
	Install    Installation
	Version    string
	ParentPID  int
	HelperPID  int
	Args       []string
	WorkingDir string
}

func jobParent(install Installation) string {
	if install.OS == "darwin" {
		return filepath.Dir(install.Root)
	}
	return install.Root
}
func Prepare(ctx context.Context, c *Client, r *Release, install Installation, progress Reporter) (string, error) {
	job, err := os.MkdirTemp(jobParent(install), ".foreverdubbed-update-")
	if err != nil {
		return "", fmt.Errorf("cannot write to the installation folder; move ForeverDubbed to a writable location: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(job)
		}
	}()
	archive := filepath.Join(job, "download.zip")
	if err = c.Download(ctx, r, archive, progress); err != nil {
		return "", err
	}
	payload := filepath.Join(job, "payload")
	if err = extract(ctx, archive, payload, progress); err != nil {
		return "", err
	}
	os.Remove(archive)
	manifestPath := filepath.Join(payload, ManifestName)
	if install.OS == "darwin" {
		manifestPath = filepath.Join(payload, "ForeverDubbed.app", "Contents", "Resources", ManifestName)
	}
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return "", fmt.Errorf("update manifest: %w", err)
	}
	if strings.TrimPrefix(r.Tag, "v") != manifest.Version {
		return "", fmt.Errorf("release tag and packaged version disagree")
	}
	if install.OS == "darwin" {
		report(progress, "Verifying app signature", 0, 0)
		if err = verifyMac(install.Root, filepath.Join(payload, "ForeverDubbed.app")); err != nil {
			return "", err
		}
	} else {
		for _, required := range []string{"foreverdubbed.exe", "foreverdubbed-updater.exe", "tts/voices.json"} {
			if manifest.Files[required] == "" {
				return "", fmt.Errorf("update is missing %s", required)
			}
		}
		for name, hash := range manifest.Files {
			if err = ctx.Err(); err != nil {
				return "", err
			}
			actual, err := fileHash(filepath.Join(payload, filepath.FromSlash(name)))
			if err != nil {
				return "", err
			}
			if actual != hash {
				return "", fmt.Errorf("update file checksum mismatch: %s", name)
			}
		}
	}
	helper := filepath.Join(job, filepath.Base(install.Helper))
	if err = copyFile(install.Helper, helper, 0755); err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	p := Plan{Install: install, Version: manifest.Version, ParentPID: os.Getpid(), Args: os.Args[1:], WorkingDir: cwd}
	planPath := filepath.Join(job, "plan.json")
	if err = writePlan(planPath, p); err != nil {
		return "", err
	}
	ok = true
	return planPath, nil
}
func writePlan(filename string, p Plan) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0600)
}
func LoadPlan(filename string) (Plan, error) {
	var p Plan
	data, err := os.ReadFile(filename)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	job := filepath.Dir(filename)
	if !filepath.IsAbs(filename) || !strings.HasPrefix(filepath.Base(job), ".foreverdubbed-update-") || filepath.Dir(job) != jobParent(p.Install) || p.ParentPID <= 0 || !filepath.IsAbs(p.Install.Root) || filepath.Dir(p.Install.Root) == p.Install.Root {
		return p, fmt.Errorf("invalid update plan")
	}
	expected := filepath.Join(p.Install.Root, "foreverdubbed.exe")
	if p.Install.OS == "darwin" {
		expected = filepath.Join(p.Install.Root, "Contents", "MacOS", "foreverdubbed")
	}
	if p.Install.Executable != expected {
		return p, fmt.Errorf("invalid update executable")
	}
	_, err = Version(p.Version)
	return p, err
}

// Start the independent helper and await its readiness before the app quits.
func Launch(ctx context.Context, planPath string) error {
	p, err := LoadPlan(planPath)
	if err != nil {
		return err
	}
	job := filepath.Dir(planPath)
	cmd := exec.Command(filepath.Join(job, filepath.Base(p.Install.Helper)), planPath)
	cmd.Dir = job
	detach(cmd)
	log, err := os.OpenFile(filepath.Join(job, "updater.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		log.Close()
		return err
	}
	log.Close()
	ready := filepath.Join(job, "ready")
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	for {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			<-exited
			return ctx.Err()
		case err := <-exited:
			return fmt.Errorf("update helper exited before it was ready: %v", err)
		case <-deadline.C:
			cmd.Process.Kill()
			<-exited
			return fmt.Errorf("update helper did not start; see %s", filepath.Join(job, "updater.log"))
		case <-ticker.C:
			if _, err = os.Stat(ready); err == nil {
				return nil
			}
		}
	}
}
func Ready(planPath string) error {
	p, err := LoadPlan(planPath)
	if err != nil {
		return err
	}
	p.HelperPID = os.Getpid()
	if err = writePlan(planPath, p); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Dir(planPath), "ready"), []byte("ready"), 0600)
}
func WaitForApp(ctx context.Context, p Plan) error { return waitProcess(ctx, p.ParentPID) }

// An entry is committed only after its previous copy has been moved into the
// rollback directory. Rollback runs in reverse order on any replacement error.
type replacement struct {
	target, backup    string
	hadOld, installed bool
}

func rollback(entries []replacement) error {
	var errs []error
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.installed {
			if err := os.RemoveAll(e.target); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		if e.hadOld {
			if err := os.Rename(e.backup, e.target); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
func Apply(planPath string, progress Reporter) error {
	p, err := LoadPlan(planPath)
	if err != nil {
		return err
	}
	job := filepath.Dir(planPath)
	payload := filepath.Join(job, "payload")
	previous := filepath.Join(job, "previous")
	if p.Install.OS == "darwin" {
		source := filepath.Join(payload, "ForeverDubbed.app")
		if err = verifyMac(p.Install.Root, source); err != nil {
			return err
		}
		report(progress, "Installing update", 0, 1)
		if err = os.Rename(p.Install.Root, previous); err != nil {
			return err
		}
		if err = os.Rename(source, p.Install.Root); err != nil {
			return errors.Join(err, os.Rename(previous, p.Install.Root))
		}
		report(progress, "Installing update", 1, 1)
		return nil
	}
	incoming, err := readManifest(filepath.Join(payload, ManifestName))
	if err != nil {
		return err
	}
	names := make([]string, 0, len(incoming.Files)+1)
	for name := range incoming.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	names = append(names, ManifestName)
	var entries []replacement
	fail := func(err error) error { return errors.Join(err, rollback(entries)) }
	for i, name := range names {
		report(progress, "Installing update", int64(i), int64(len(names)))
		destination := filepath.Join(p.Install.Root, filepath.FromSlash(name))
		// Existing symlink parents must not redirect writes outside the install.
		for dir := filepath.Dir(destination); dir != p.Install.Root; dir = filepath.Dir(dir) {
			info, statErr := os.Lstat(dir)
			if statErr == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
				return fail(fmt.Errorf("unsafe installed directory: %s", dir))
			}
			if statErr != nil && !os.IsNotExist(statErr) {
				return fail(statErr)
			}
		}
		info, statErr := os.Lstat(destination)
		if statErr != nil && !os.IsNotExist(statErr) {
			return fail(statErr)
		}
		if statErr == nil && !info.Mode().IsRegular() {
			return fail(fmt.Errorf("installed path is not a regular file: %s", name))
		}
		e := replacement{target: destination, backup: filepath.Join(previous, filepath.FromSlash(name)), hadOld: statErr == nil}
		if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return fail(err)
		}
		if e.hadOld {
			if err = os.MkdirAll(filepath.Dir(e.backup), 0755); err != nil {
				return fail(err)
			}
			if err = os.Rename(destination, e.backup); err != nil {
				return fail(err)
			}
		}
		entries = append(entries, e)
		if err = os.Rename(filepath.Join(payload, filepath.FromSlash(name)), destination); err != nil {
			return fail(err)
		}
		entries[len(entries)-1].installed = true
	}
	report(progress, "Installing update", int64(len(names)), int64(len(names)))
	return nil
}
func Restart(planPath string) error {
	p, err := LoadPlan(planPath)
	if err != nil {
		return err
	}
	// Keep backups if restarting fails, so recovery remains possible.
	if err = os.WriteFile(filepath.Join(filepath.Dir(planPath), "complete"), []byte("complete"), 0600); err != nil {
		return err
	}
	cmd := exec.Command(p.Install.Executable, p.Args...)
	cmd.Dir = p.WorkingDir
	if cmd.Dir == "" {
		cmd.Dir = filepath.Dir(p.Install.Executable)
	}
	detach(cmd)
	if err = cmd.Start(); err != nil {
		os.Remove(filepath.Join(filepath.Dir(planPath), "complete"))
		return err
	}
	go cmd.Wait()
	return nil
}

// Completed jobs are removed only after their helper exits. Failed jobs retain
// their logs/backups for recovery; unfinished jobs are never garbage-collected.
func CleanupCompleted(install Installation) {
	entries, err := os.ReadDir(jobParent(install))
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), ".foreverdubbed-update-") {
			continue
		}
		job := filepath.Join(jobParent(install), e.Name())
		if _, err = os.Stat(filepath.Join(job, "complete")); err != nil {
			continue
		}
		p, err := LoadPlan(filepath.Join(job, "plan.json"))
		if err != nil || p.Install.Root != install.Root || p.HelperPID <= 0 {
			continue
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if waitProcess(ctx, p.HelperPID) == nil {
				os.RemoveAll(job)
			}
		}()
	}
}
