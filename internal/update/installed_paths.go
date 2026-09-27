package update

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Validate the root and every parent before touching an installed file. This
// applies equally to obsolete files and replacements.
func safeInstalledPath(root, name string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root || !safePath(name) {
		return fmt.Errorf("invalid installed path")
	}
	for dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(name))); ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("unsafe installed directory: %s", dir)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if dir == root {
			break
		}
	}
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name)))
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("installed path is not a regular file: %s", name)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func removeEmptyParents(root string, names []string) {
	dirs := map[string]bool{}
	for _, name := range names {
		for dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(name))); dir != root; dir = filepath.Dir(dir) {
			dirs[dir] = true
		}
	}
	ordered := make([]string, 0, len(dirs))
	for dir := range dirs {
		ordered = append(ordered, dir)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ordered)))
	for _, dir := range ordered {
		// Remove only empty directories; unrelated files must survive.
		_ = os.Remove(dir)
	}
}
