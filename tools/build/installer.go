package main

import (
	_ "embed"
	"fmt"
	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/update"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed installer.nsi
var installerTemplate string

func releaseZIPName(targetOS, arch string) string {
	if targetOS == "windows" {
		return "ForeverDubbed-windows-" + arch + "-portable.zip"
	}
	return "ForeverDubbed-mac-" + arch + ".zip"
}

// Use the ZIP's exact manifest, including only configured voices. Explicit
// uninstall paths avoid recursively deleting unrelated files in the install dir.
func installerScript(output string, files map[string]string) (string, error) {
	if _, err := update.Version(buildinfo.Version); err != nil {
		return "", err
	}
	if _, ok := files["foreverdubbed.exe"]; !ok {
		return "", fmt.Errorf("Windows installer is missing its executable")
	}
	names := make([]string, 0, len(files))
	dirs := map[string]bool{}
	for name := range files {
		if name == "." || !filepath.IsLocal(name) || path.Clean(name) != name || strings.ContainsAny(name, "\\:*?\"<>|\r\n") {
			return "", fmt.Errorf("invalid installer path %q", name)
		}
		names = append(names, name)
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	sort.Strings(names)
	var install, remove strings.Builder
	for _, name := range names {
		source, err := filepath.Abs(files[name])
		if err != nil {
			return "", err
		}
		dir := "$INSTDIR"
		if path.Dir(name) != "." {
			dir += "\\" + nsisEscape(strings.ReplaceAll(path.Dir(name), "/", "\\"))
		}
		fmt.Fprintf(&install, "  SetOutPath \"%s\"\n  File %s %s\n", dir, nsisQuote("/oname="+path.Base(name)), nsisQuote(source))
		fmt.Fprintf(&remove, "  Delete \"$INSTDIR\\%s\"\n", nsisEscape(strings.ReplaceAll(name, "/", "\\")))
	}
	orderedDirs := make([]string, 0, len(dirs))
	for dir := range dirs {
		orderedDirs = append(orderedDirs, dir)
	}
	// Descendants sort after their parents; reverse order removes children first.
	sort.Sort(sort.Reverse(sort.StringSlice(orderedDirs)))
	for _, dir := range orderedDirs {
		fmt.Fprintf(&remove, "  RMDir \"$INSTDIR\\%s\"\n", nsisEscape(strings.ReplaceAll(dir, "/", "\\")))
	}
	return strings.NewReplacer("@VERSION@", buildinfo.Version, "@OUTPUT@", nsisQuote(output), "@INSTALL_FILES@", install.String(), "@REMOVE_FILES@", remove.String()).Replace(installerTemplate), nil
}

func nsisEscape(value string) string {
	return strings.NewReplacer("$", "$$", "\"", "$\\\"", "\r", "$\\r", "\n", "$\\n").Replace(value)
}

func nsisQuote(value string) string { return "\"" + nsisEscape(value) + "\"" }

func writeWindowsInstaller(destination string, files map[string]string) error {
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".foreverdubbed-installer-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	output, err := filepath.Abs(filepath.Join(stage, filepath.Base(destination)))
	if err != nil {
		return err
	}
	script, err := installerScript(output, files)
	if err != nil {
		return err
	}
	scriptPath := filepath.Join(stage, "installer.nsi")
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		return err
	}
	cmd := exec.Command("makensis", "-NOCD", "-V2", "-WX", scriptPath)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build Windows installer (requires NSIS/makensis): %w", err)
	}
	return os.Rename(output, destination)
}
