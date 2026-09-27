// Package game locates and validates WoW installations independently of addon installation.
package game

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Directory accepts the Windows executable or the macOS application bundle.
func Directory(path, platform string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(path)
	valid := platform == "windows" && strings.EqualFold(ext, ".exe") && info.Mode().IsRegular()
	valid = valid || platform == "darwin" && strings.EqualFold(ext, ".app") && info.IsDir()
	if !valid {
		return "", fmt.Errorf("select the WoW executable (.exe) or application bundle (.app)")
	}
	dir := filepath.Dir(path)
	isDir := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}
	// Older clients keep Data beside the executable; launched clients also
	// have Interface and WTF. Modern installations share Data and .build.info
	// in the parent directory, even before their first launch.
	if isDir(filepath.Join(dir, "Data")) ||
		(isDir(filepath.Join(dir, "Interface")) && isDir(filepath.Join(dir, "WTF"))) {
		return dir, nil
	}
	parent := filepath.Dir(dir)
	build, err := os.Stat(filepath.Join(parent, ".build.info"))
	if err == nil && build.Mode().IsRegular() && isDir(filepath.Join(parent, "Data")) {
		return dir, nil
	}
	return "", fmt.Errorf("no WoW installation found beside %s; select the game client in its installation folder", filepath.Base(path))
}

func Candidates(platform string, getenv func(string) string) []string {
	if platform == "darwin" {
		return []string{"/Applications/World of Warcraft/_classic_beta_/World of Warcraft Beta.app"}
	}
	if platform != "windows" {
		return nil
	}
	roots := []string{getenv("ProgramFiles(x86)"), getenv("ProgramFiles"), `C:\Program Files (x86)`, `C:\Program Files`, `C:\Games`, `C:\`}
	var paths []string
	for _, root := range roots {
		if root != "" {
			paths = append(paths, filepath.Join(root, "World of Warcraft", "_classic_beta_", "WowB.exe"))
		}
	}
	return paths
}
