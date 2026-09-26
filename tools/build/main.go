// Build Windows and macOS releases, including with Linux cross-compilers.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/buildtool"
)

var runtimeFiles = []string{
	"README.md", "PROTOCOL.md", "CHANGELOG.md", "native/README.md", "tts/voices.json",
}

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "Build failed:", err)
		os.Exit(1)
	}
}

func build() error {
	nativeDir := flag.String("native-dir", ".runtime/native", "target ONNX Runtime, models, and presets directory")
	target := flag.String("target", "windows", "release target: windows or darwin")
	windowsInstaller := flag.Bool("windows-installer", false, "also build a Windows amd64 installer (requires NSIS/makensis)")
	macUnsigned := flag.Bool("mac-unsigned", false, "explicitly skip macOS certificate signing (test builds only)")
	arch := flag.String("arch", "", "target architecture: amd64 or arm64")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("usage: go run ./tools/build [-target windows|darwin] [-arch amd64|arm64] [-native-dir path] [-windows-installer]")
	}
	release, err := releaseVersion()
	if err != nil {
		return err
	}
	buildinfo.Version = release
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	spec, err := buildtool.ResolveTarget(*target, *arch)
	if err != nil {
		return err
	}
	if spec.OS != "windows" && spec.OS != "darwin" {
		return fmt.Errorf("release target must be windows or darwin")
	}
	targetOS, targetArch := spec.OS, spec.Arch
	if *windowsInstaller {
		if targetOS != "windows" || targetArch != "amd64" {
			return fmt.Errorf("-windows-installer requires -target windows -arch amd64")
		}
		if _, err := exec.LookPath("makensis"); err != nil {
			return fmt.Errorf("-windows-installer requires NSIS/makensis: %w", err)
		}
	}
	var signMac func(string) error
	if targetOS == "darwin" {
		signMac, err = macSigner(root, *macUnsigned)
		if err != nil {
			return err
		}
	}
	cc, cxx, err := spec.Compilers()
	if err != nil {
		return err
	}
	bundle, addon, err := packageFiles(root)
	if err != nil {
		return err
	}
	if err := addTargetNativeFiles(bundle, *nativeDir, spec); err != nil {
		return err
	}
	hostCGO := runtime.GOOS == "darwin" && targetOS == "darwin" && targetArch == runtime.GOARCH
	hostEnv := buildEnv(os.Environ(), runtime.GOOS, runtime.GOARCH, hostCGO)
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
	if err := stageAddon(dist, bundle, addon); err != nil {
		return err
	}
	// Keep the development executable runnable outside the ZIP too.
	for name, source := range bundle {
		ext := filepath.Ext(name)
		if ext != ".dll" && ext != ".dylib" {
			continue
		}
		destination := filepath.Join(dist, filepath.FromSlash(name))
		if err := buildtool.CopyFile(source, destination); err != nil {
			return err
		}
	}
	binaryName := "foreverdubbed"
	if targetOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(dist, binaryName)
	nativeEnv := buildEnv(os.Environ(), targetOS, targetArch, true)
	nativeEnv = append(nativeEnv, "CC="+cc, "CXX="+cxx)
	versionFlag := "-X foreverdubbed/internal/buildinfo.Version=" + release
	helperName := "foreverdubbed-updater"
	helperFlags := versionFlag + " -s -w"
	if targetOS == "windows" {
		helperName += ".exe"
		helperFlags += " -H=windowsgui"
	}
	helperPath := filepath.Join(dist, helperName)
	if err := run(root, nativeEnv, "build", "-tags", "gui", "-buildvcs=false", "-trimpath", "-ldflags", helperFlags, "-o", helperPath, "./cmd/foreverdubbed-updater"); err != nil {
		return err
	}
	bundle[helperName] = helperPath
	buildArgs := []string{"build", "-tags", "pocket_native,gui", "-buildvcs=false", "-trimpath"}
	if targetOS == "darwin" {
		// cgo source directives reject @-prefixed rpaths. Pass these deliberate
		// release loader paths through the Go external linker instead.
		buildArgs = append(buildArgs, "-ldflags", versionFlag+" -s -w -extldflags=-Wl,-rpath,@executable_path/../Resources/native,-rpath,@executable_path/native,-rpath,@executable_path/../.runtime/native")
	}
	if targetOS == "windows" {
		buildArgs = append(buildArgs, "-ldflags", versionFlag+" -H=windowsgui")
	}
	buildArgs = append(buildArgs, "-o", binary, "./cmd/foreverdubbed")
	if err := run(root, nativeEnv, buildArgs...); err != nil {
		return err
	}
	bundle[binaryName] = binary
	if err := addReleaseManifest(dist, bundle); err != nil {
		return err
	}
	if targetOS == "darwin" {
		bundle, err = macApp(dist, bundle, signMac)
		if err != nil {
			return err
		}
	}
	if err := writeZIP(filepath.Join(dist, "ForeverDubbed-addon.zip"), addon); err != nil {
		return err
	}
	if err := writeZIP(filepath.Join(dist, releaseZIPName(targetOS, targetArch)), bundle); err != nil {
		return err
	}
	if *windowsInstaller {
		installer := filepath.Join(dist, "ForeverDubbed-windows-"+targetArch+"-setup.exe")
		if err := writeWindowsInstaller(installer, bundle); err != nil {
			return err
		}
		fmt.Println("Built", installer)
	}
	fmt.Printf("Built %s and addon/%s-%s ZIP packages.\n", binary, targetOS, targetArch)
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

func buildEnv(env []string, goos, goarch string, cgo bool) []string {
	result := make([]string, 0, len(env)+3)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "GOOS", "GOARCH", "CGO_ENABLED":
		default:
			result = append(result, entry)
		}
	}
	enabled := "0"
	if cgo {
		enabled = "1"
	}
	return append(result, "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED="+enabled)
}

func run(root string, env []string, args ...string) error {
	fmt.Println("go", strings.Join(args, " "))
	cmd := exec.Command("go", args...)
	cmd.Dir, cmd.Env = root, env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
