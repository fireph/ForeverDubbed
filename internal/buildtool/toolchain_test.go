package buildtool

import (
	"fmt"
	"strings"
	"testing"
)

func TestWindowsCompilers(t *testing.T) {
	for _, tc := range []struct {
		name, host, cc, cxx string
		available           []string
		wantCC, wantCXX     string
	}{
		{
			name: "Ubuntu prefers POSIX threads", host: "linux",
			available: []string{"gcc", "g++", "x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-g++", "x86_64-w64-mingw32-gcc-posix", "x86_64-w64-mingw32-g++-posix"},
			wantCC:    "x86_64-w64-mingw32-gcc-posix", wantCXX: "x86_64-w64-mingw32-g++-posix",
		},
		{
			name: "generic MinGW", host: "darwin",
			available: []string{"x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-g++"},
			wantCC:    "x86_64-w64-mingw32-gcc", wantCXX: "x86_64-w64-mingw32-g++",
		},
		{
			name: "Windows host", host: "windows", available: []string{"gcc", "g++"},
			wantCC: "gcc", wantCXX: "g++",
		},
		{
			name: "explicit wrappers", host: "linux", cc: "zig cc -target x86_64-windows-gnu", cxx: "zig c++ -target x86_64-windows-gnu",
			wantCC: "zig cc -target x86_64-windows-gnu", wantCXX: "zig c++ -target x86_64-windows-gnu",
		},
		{
			name: "host compilers cannot produce Windows", host: "linux", available: []string{"gcc", "g++"},
		},
		{
			name: "missing C++ compiler", host: "linux", available: []string{"x86_64-w64-mingw32-gcc-posix"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string {
				return map[string]string{"CC": tc.cc, "CXX": tc.cxx}[key]
			}
			lookup := func(name string) (string, error) {
				for _, available := range tc.available {
					if name == available {
						return name, nil
					}
				}
				return "", fmt.Errorf("missing %s", name)
			}
			cc, cxx, err := windowsCompilers(tc.host, getenv, lookup)
			if tc.wantCC == "" {
				if err == nil || !strings.Contains(err.Error(), "sudo apt-get install") {
					t.Fatalf("expected actionable missing-toolchain error, got %v", err)
				}
				return
			}
			if err != nil || cc != tc.wantCC || cxx != tc.wantCXX {
				t.Fatalf("got (%q, %q, %v), want (%q, %q)", cc, cxx, err, tc.wantCC, tc.wantCXX)
			}
		})
	}
}
