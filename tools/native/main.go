// Prepare dependencies for the Windows release, or the host's native speech tools.
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
	target := flag.String("target", "windows", "dependency target: windows (x64 release) or host (native speech tools)")
	flag.Parse()
	if flag.NArg() != 0 || (*target != "windows" && *target != "host") {
		return fmt.Errorf("usage: go run ./tools/native [-target windows|host] [-out path]")
	}
	targetOS, targetArch := runtime.GOOS, runtime.GOARCH
	env := os.Environ()
	if *target == "windows" {
		targetOS, targetArch = "windows", "amd64"
		cc, cxx, err := buildtool.WindowsCompilers()
		if err != nil {
			return err
		}
		env = append(env, "CC="+cc, "CXX="+cxx)
	}
	dir, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	// Separate caches by host and target; a WSL checkout can also be used from
	// Windows, and old Linux/MSVC caches cannot be reused for cross-compilation.
	build := filepath.Join(dir, "build-"+runtime.GOOS+"-"+runtime.GOARCH+"-"+targetOS+"-"+targetArch)
	configure := []string{"-S", "native", "-B", build, "-DCMAKE_BUILD_TYPE=Release", "-DCMAKE_INSTALL_PREFIX=" + dir}
	if targetOS == "windows" {
		configure = append(configure, "-DCMAKE_SYSTEM_NAME=Windows", "-DCMAKE_SYSTEM_PROCESSOR=AMD64")
	}
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
