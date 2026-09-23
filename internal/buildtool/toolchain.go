// Package buildtool shares toolchain selection between dependency setup and packaging.
package buildtool

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// WindowsCompilers selects the same Windows x64 C/C++ compilers for CMake and
// cgo. Explicit CC/CXX values may name wrapper commands and take precedence.
func WindowsCompilers() (string, string, error) {
	return windowsCompilers(runtime.GOOS, os.Getenv, exec.LookPath)
}

func windowsCompilers(host string, getenv func(string) string, lookPath func(string) (string, error)) (string, string, error) {
	cc, cxx := getenv("CC"), getenv("CXX")
	var pairs [][2]string
	if host == "windows" {
		pairs = [][2]string{{"gcc", "g++"}}
	} else {
		// Ubuntu's POSIX variant supports the C++ threads used by PocketTTS.
		pairs = [][2]string{
			{"x86_64-w64-mingw32-gcc-posix", "x86_64-w64-mingw32-g++-posix"},
			{"x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-g++"},
		}
	}
	for _, pair := range pairs {
		c, cpp := cc, cxx
		var errC, errCPP error
		if c == "" {
			c, errC = lookPath(pair[0])
		}
		if cpp == "" {
			cpp, errCPP = lookPath(pair[1])
		}
		if errC == nil && errCPP == nil {
			return c, cpp, nil
		}
	}
	return "", "", fmt.Errorf("Windows x64 C/C++ compilers not found; on Ubuntu/WSL install them with: sudo apt-get install g++-mingw-w64-x86-64-posix (or set CC and CXX to Windows x64 compiler commands)")
}
