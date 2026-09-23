// Prepare native dependencies for Windows, macOS, or the host.
package main

import (
	"flag"
	"fmt"
	"foreverdubbed/internal/buildtool"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	out := flag.String("out", ".runtime/native", "native runtime directory")
	target := flag.String("target", "windows", "dependency target: windows, darwin, or host")
	arch := flag.String("arch", "", "target architecture: amd64 or arm64")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}
	targetSpec, err := buildtool.ResolveTarget(*target, *arch)
	if err != nil {
		return err
	}
	targetOS, targetArch := targetSpec.OS, targetSpec.Arch
	cc, cxx, err := targetSpec.Compilers()
	if err != nil {
		return err
	}
	env := append(os.Environ(), "CC="+cc, "CXX="+cxx)
	dir, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	// Separate caches by host and target; a WSL checkout can also be used from
	// Windows, and old Linux/MSVC caches cannot be reused for cross-compilation.
	build := filepath.Join(dir, "build-"+runtime.GOOS+"-"+runtime.GOARCH+"-"+targetOS+"-"+targetArch)
	configure := []string{"-S", "native", "-B", build, "-DCMAKE_BUILD_TYPE=Release", "-DCMAKE_INSTALL_PREFIX=" + dir}
	configure = append(configure, targetSpec.CMakeArgs()...)
	if runtime.GOOS == "windows" {
		// CMake otherwise defaults to MSVC, which is incompatible with cgo's GCC ABI.
		configure = append(configure, "-G", "MinGW Makefiles")
	}
	for _, args := range [][]string{configure, {"--build", build, "--config", "Release", "--target", "sentencepiece-static", "--parallel", "2"}, {"--install", build, "--config", "Release", "--component", "Native"}} {
		fmt.Println("cmake", args)
		cmd := exec.Command("cmake", args...)
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}
