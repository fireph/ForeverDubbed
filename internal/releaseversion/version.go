// Package releaseversion parses and compares stable MAJOR.MINOR.PATCH versions.
package releaseversion

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse parses stable release versions only. Prereleases must never replace a
// stable install, and integer comparison avoids treating 0.10 as older than 0.9.
func Parse(s string) ([3]uint64, error) {
	var result [3]uint64
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("invalid release version %q", s)
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return result, fmt.Errorf("invalid release version %q", s)
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return result, fmt.Errorf("invalid release version %q", s)
			}
		}
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return result, err
		}
		result[i] = v
	}
	return result, nil
}

// Newer returns false if either version is invalid.
func Newer(candidate, current string) bool {
	a, err := Parse(candidate)
	if err != nil {
		return false
	}
	b, err := Parse(current)
	if err != nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
