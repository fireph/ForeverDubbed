package buildtool

import (
	"fmt"
	"os"
	"runtime"
)

// Target keeps dependency preparation and application packaging consistent.
type Target struct{ OS, Arch string }

func ResolveTarget(name, arch string) (Target, error) {
	return resolveTarget(name, arch, runtime.GOOS, runtime.GOARCH)
}
func resolveTarget(name, arch, hostOS, hostArch string) (Target, error) {
	if name == "host" {
		name = hostOS
		if arch == "" {
			arch = hostArch
		}
	}
	if arch == "" {
		arch = "amd64"
		if name == "darwin" {
			arch = "arm64"
			if hostOS == "darwin" {
				arch = hostArch
			}
		}
	}
	if (name == "windows" && arch == "amd64") || ((name == "darwin" || name == "linux") && (arch == "amd64" || arch == "arm64")) {
		return Target{name, arch}, nil
	}
	return Target{}, fmt.Errorf("unsupported target %s/%s", name, arch)
}
func (t Target) Compilers() (string, string, error) {
	if t.OS == "windows" {
		return WindowsCompilers()
	}
	cc, cxx := os.Getenv("CC"), os.Getenv("CXX")
	if t.OS != runtime.GOOS || t.Arch != runtime.GOARCH {
		if cc == "" || cxx == "" {
			return "", "", fmt.Errorf("cross-compiling %s/%s requires CC and CXX set to matching cross-compilers (for macOS use OSXCross)", t.OS, t.Arch)
		}
	}
	if cc == "" {
		cc = "cc"
		if t.OS == "darwin" {
			cc = "clang"
		}
	}
	if cxx == "" {
		cxx = "c++"
		if t.OS == "darwin" {
			cxx = "clang++"
		}
	}
	return cc, cxx, nil
}
func (t Target) CMakeArgs() []string {
	switch t.OS {
	case "windows":
		return []string{"-DCMAKE_SYSTEM_NAME=Windows", "-DCMAKE_SYSTEM_PROCESSOR=AMD64"}
	case "darwin":
		arch := "arm64"
		if t.Arch == "amd64" {
			arch = "x86_64"
		}
		args := []string{"-DCMAKE_SYSTEM_NAME=Darwin", "-DCMAKE_SYSTEM_PROCESSOR=" + arch, "-DCMAKE_OSX_ARCHITECTURES=" + arch, "-DCMAKE_OSX_DEPLOYMENT_TARGET=14.0"}
		if sdk := os.Getenv("MACOS_SDK"); sdk != "" {
			args = append(args, "-DCMAKE_OSX_SYSROOT="+sdk)
		}
		return args
	}
	return nil
}
