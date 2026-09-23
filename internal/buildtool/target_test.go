package buildtool

import "testing"

func TestTargetDefaultsAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, arch, host, hostArch string
		want                       Target
		valid                      bool
	}{
		{"windows", "", "linux", "arm64", Target{"windows", "amd64"}, true},
		{"darwin", "", "linux", "amd64", Target{"darwin", "arm64"}, true},
		{"darwin", "amd64", "linux", "arm64", Target{"darwin", "amd64"}, true},
		{"darwin", "", "darwin", "amd64", Target{"darwin", "amd64"}, true},
		{"host", "", "linux", "arm64", Target{"linux", "arm64"}, true},
		{"windows", "arm64", "linux", "amd64", Target{}, false},
		{"darwin", "386", "linux", "amd64", Target{}, false},
		{"unknown", "", "linux", "amd64", Target{}, false},
	} {
		got, err := resolveTarget(tc.name, tc.arch, tc.host, tc.hostArch)
		if got != tc.want || (err == nil) != tc.valid {
			t.Errorf("%+v: got %+v, %v", tc, got, err)
		}
	}
}
