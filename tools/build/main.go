// Build the Windows release on any Go-supported build host.
package main

import (
	"archive/zip"
	"debug/pe"
	"encoding/json"
	"flag"
	"fmt"
	"foreverdubbed/internal/buildtool"
	"foreverdubbed/internal/pocket"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Runtime support remains Windows/amd64. Host-independent packaging does not
// imply that screen capture or audio playback work on other operating systems.
const targetOS, targetArch = "windows", "amd64"

var runtimeFiles = []string{
	"README.md", "PROTOCOL.md", "CHANGELOG.md", "native/README.md", "Start-ForeverDubbed.cmd", "tts/voices.json",
}

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "Build failed:", err)
		os.Exit(1)
	}
}

func build() error {
	nativeDir := flag.String("native-dir", ".runtime/native", "Windows x64 ONNX Runtime, models, and presets directory")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("usage: go run ./tools/build [-native-dir path]")
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	cc, cxx, err := buildtool.WindowsCompilers()
	if err != nil {
		return err
	}
	bundle, addon, err := packageFiles(root)
	if err != nil {
		return err
	}
	if err := addNativeFiles(bundle, *nativeDir); err != nil {
		return err
	}
	hostEnv := buildEnv(os.Environ(), runtime.GOOS, runtime.GOARCH)
	if err := run(root, hostEnv, "test", "./..."); err != nil {
		return err
	}
	if err := run(root, hostEnv, "vet", "./..."); err != nil {
		return err
	}
	dist := filepath.Join(root, "dist")
	if err := os.MkdirAll(dist, 0755); err != nil {
		return err
	}
	// Keep the development executable runnable outside the ZIP too.
	for name, source := range bundle {
		if filepath.Ext(name) != ".dll" || filepath.Dir(name) != "." {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dist, name), data, 0644); err != nil {
			return err
		}
	}
	binary := filepath.Join(dist, "foreverdubbed.exe")
	nativeEnv := buildEnv(os.Environ(), targetOS, targetArch)
	nativeEnv[len(nativeEnv)-1] = "CGO_ENABLED=1"
	nativeEnv = append(nativeEnv, "CC="+cc, "CXX="+cxx)
	if err := run(root, nativeEnv, "build", "-tags", "pocket_native", "-buildvcs=false", "-trimpath", "-o", binary, "./cmd/foreverdubbed"); err != nil {
		return err
	}
	bundle["foreverdubbed.exe"] = binary
	if err := writeZIP(filepath.Join(dist, "ForeverDubbed-addon.zip"), addon); err != nil {
		return err
	}
	if err := writeZIP(filepath.Join(dist, "ForeverDubbed-"+targetOS+"-"+targetArch+".zip"), bundle); err != nil {
		return err
	}
	fmt.Println("Built dist/foreverdubbed.exe and addon/Windows ZIP packages.")
	return nil
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 2 && fields[0] == "module" && fields[1] == "foreverdubbed" {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run this build from the ForeverDubbed repository")
		}
		dir = parent
	}
}

func buildEnv(env []string, goos, goarch string) []string {
	result := make([]string, 0, len(env)+3)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "GOOS", "GOARCH", "CGO_ENABLED":
		default:
			result = append(result, entry)
		}
	}
	return append(result, "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
}

func run(root string, env []string, args ...string) error {
	fmt.Println("go", strings.Join(args, " "))
	cmd := exec.Command("go", args...)
	cmd.Dir, cmd.Env = root, env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// Maps ZIP names (always slash-separated) to source files. No staging tree or
// shell-specific archiver is needed, and no unreferenced voice files are copied.
func packageFiles(root string) (map[string]string, map[string]string, error) {
	bundle, addon := map[string]string{}, map[string]string{}
	for _, name := range runtimeFiles {
		source := filepath.Join(root, filepath.FromSlash(name))
		if err := regularFile(source); err != nil {
			return nil, nil, err
		}
		bundle[name] = source
	}
	for _, dir := range []string{"docs", "data", "addon/ForeverDubbed"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(source string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if err := regularFile(source); err != nil {
				return err
			}
			relative, err := filepath.Rel(root, source)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(relative)
			bundle[name] = source
			if strings.HasPrefix(name, "addon/") {
				addon[strings.TrimPrefix(name, "addon/")] = source
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	voices, err := voiceFiles(root)
	if err != nil {
		return nil, nil, err
	}
	for name, source := range voices {
		bundle[name] = source
	}
	return bundle, addon, nil
}

func regularFile(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("bundle source must be a regular file: %s", name)
	}
	return nil
}

func voiceFiles(root string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "tts", "voices.json"))
	if err != nil {
		return nil, err
	}
	var config struct {
		Profiles map[string]struct {
			Voice string `json:"voice"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	files := map[string]string{}
	custom := filepath.Join(root, "tts", "custom")
	for _, profile := range config.Profiles {
		// Parse Windows-style references the same way on every build host.
		name := path.Clean(strings.ReplaceAll(profile.Voice, "\\", "/"))
		ext := strings.ToLower(path.Ext(name))
		if ext != ".safetensors" && ext != ".wav" {
			continue
		}
		if !strings.HasPrefix(name, "custom/") || strings.Contains(name, ":") || strings.HasSuffix(strings.ToLower(name), ".pending.safetensors") {
			return nil, fmt.Errorf("bundle voice references must be finished files under tts/custom: %s", profile.Voice)
		}
		source := filepath.Join(root, "tts", filepath.FromSlash(name))
		// Resolve symlinks too: a lexical path inside custom/ can point outside.
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return nil, fmt.Errorf("voice %s: %w", profile.Voice, err)
		}
		realCustom, err := filepath.EvalSymlinks(custom)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(realCustom, resolved)
		if err != nil || !filepath.IsLocal(relative) {
			return nil, fmt.Errorf("voice escapes tts/custom: %s", profile.Voice)
		}
		if err := regularFile(resolved); err != nil {
			return nil, err
		}
		files["tts/"+name] = resolved
	}
	return files, nil
}

func writeZIP(destination string, files map[string]string) (err error) {
	output, err := os.CreateTemp(filepath.Dir(destination), ".foreverdubbed-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	defer output.Close()
	archive := zip.NewWriter(output)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := addFile(archive, name, files[name]); err != nil {
			archive.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Rename(output.Name(), destination)
}

func addFile(archive *zip.Writer, name, source string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name, header.Method = name, zip.Deflate
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, input)
	return err
}

func addNativeFiles(bundle map[string]string, dir string) error {
	for _, name := range []string{"onnxruntime.dll"} {
		filename := filepath.Join(dir, name)
		image, err := pe.Open(filename)
		if err != nil {
			return fmt.Errorf("need Windows x64 native runtime at %s (prepare it with go run ./tools/native, or pass -native-dir): %w", dir, err)
		}
		machine := image.Machine
		image.Close()
		if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return fmt.Errorf("%s is not a Windows x64 library", filename)
		}
		bundle[name] = filename
	}
	// ONNX Runtime must be beside the executable for Windows loader startup.
	// Include any supplied compiler runtime DLLs, but ignore the old PocketTTS bridge.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && !strings.EqualFold(entry.Name(), "foreverdubbed_tts.dll") && strings.EqualFold(filepath.Ext(entry.Name()), ".dll") {
			bundle[entry.Name()] = filepath.Join(dir, entry.Name())
		}
	}
	for _, asset := range pocket.Assets() {
		if err := pocket.VerifyAsset(dir, asset); err != nil {
			return fmt.Errorf("native assets: %w", err)
		}
		bundle["native/"+asset.Path] = filepath.Join(dir, filepath.FromSlash(asset.Path))
	}
	for _, name := range []string{"PocketTTS.cpp.txt", "ONNX-Runtime.txt", "SentencePiece.txt", "nlohmann-json.txt", "dr_libs.txt"} {
		source := filepath.Join(dir, "licenses", name)
		if err := regularFile(source); err != nil {
			return err
		}
		bundle["native/licenses/"+name] = source
	}
	return nil
}
