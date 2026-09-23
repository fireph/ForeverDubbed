// Prepare dependencies for the embedded speech engine for the current host. Requires CMake and C++17.
package main

import (
	"flag"
	"fmt"
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
	flag.Parse()
	dir, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	build := filepath.Join(dir, "build")
	configure := []string{"-S", "native", "-B", build, "-DCMAKE_BUILD_TYPE=Release", "-DCMAKE_INSTALL_PREFIX=" + dir}
	if runtime.GOOS == "windows" {
		// CMake otherwise defaults to MSVC, which is incompatible with cgo's GCC ABI.
		configure = append(configure, "-G", "MinGW Makefiles")
	}
	for _, args := range [][]string{configure, {"--build", build, "--config", "Release", "--target", "sentencepiece-static", "--parallel", "2"}, {"--install", build, "--config", "Release", "--component", "Native"}} {
		fmt.Println("cmake", args)
		cmd := exec.Command("cmake", args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}
