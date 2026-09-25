package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const ManifestName = "release-manifest.json"

type Manifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}
type Installation struct{ Root, Executable, Helper, OS, Arch string }

// Development binaries deliberately do not update a source checkout. Only a
// complete release (including its independently runnable helper) can update.
func Locate() (Installation, error) {
	exe, err := os.Executable()
	if err != nil {
		return Installation{}, err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return Installation{}, err
	}
	install := Installation{Executable: exe, OS: runtime.GOOS, Arch: runtime.GOARCH}
	switch runtime.GOOS {
	case "windows":
		install.Root = filepath.Dir(exe)
		install.Helper = filepath.Join(install.Root, "foreverdubbed-updater.exe")
		if _, err = os.Stat(filepath.Join(install.Root, "native", "models")); err != nil {
			return install, fmt.Errorf("automatic updates require an extracted or installed release")
		}
	case "darwin":
		install.Root = filepath.Dir(filepath.Dir(filepath.Dir(exe)))
		if !strings.HasSuffix(install.Root, ".app") || filepath.Dir(exe) != filepath.Join(install.Root, "Contents", "MacOS") {
			return install, fmt.Errorf("automatic updates require an installed .app")
		}
		install.Helper = filepath.Join(install.Root, "Contents", "MacOS", "foreverdubbed-updater")
	default:
		return install, fmt.Errorf("automatic updates are supported on Windows and macOS")
	}
	if _, err = os.Stat(install.Helper); err != nil {
		return install, fmt.Errorf("update helper is missing: %w", err)
	}
	return install, nil
}

// Files are extracted into a new, private directory. Reject links, device
// files, duplicate/case-colliding names, traversal, and excessive expansion.
func extract(ctx context.Context, archive, destination string, progress Reporter) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) > 10000 {
		return fmt.Errorf("too many files in update")
	}
	seen := map[string]bool{}
	var total uint64
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		if !safePath(name) || seen[strings.ToLower(name)] || f.Mode()&os.ModeSymlink != 0 || (!f.FileInfo().IsDir() && !f.Mode().IsRegular()) {
			return fmt.Errorf("unsafe update archive path %q", f.Name)
		}
		seen[strings.ToLower(name)] = true
		if f.UncompressedSize64 > 4<<30 || total > 4<<30-f.UncompressedSize64 {
			return fmt.Errorf("update archive is too large")
		}
		total += f.UncompressedSize64
	}
	var count int64
	for _, f := range z.File {
		if err = ctx.Err(); err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if f.Mode()&0111 != 0 {
			mode = 0755
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			src.Close()
			return err
		}
		n, copyErr := io.Copy(dst, contextReader{ctx, src})
		src.Close()
		closeErr := dst.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		count += n
		report(progress, "Preparing update", count, int64(total))
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
func safePath(name string) bool {
	if name == "" || name == "." || !filepath.IsLocal(name) || path.Clean(name) != name || strings.ContainsAny(name, "\\:*?\"<>|\r\n\x00") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimRight(part, ". ") != part {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}
func fileHash(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func copyFile(source, destination string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	closeErr := dst.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func readManifest(filename string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(filename)
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if _, err = Version(m.Version); err != nil {
		return m, err
	}
	if len(m.Files) == 0 {
		return m, fmt.Errorf("empty release manifest")
	}
	for name, hash := range m.Files {
		b, err := hex.DecodeString(hash)
		if !safePath(name) || err != nil || len(b) != sha256.Size {
			return m, fmt.Errorf("invalid release manifest entry")
		}
	}
	return m, nil
}
