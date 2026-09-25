// Package addon exposes the addon's metadata to the desktop build.
package addon

import (
	_ "embed"
	"strings"
)

//go:embed ForeverDubbed/ForeverDubbed.toc
var metadata string

// Version is the single source version shared by local desktop and addon builds.
func Version() string {
	for _, line := range strings.Split(metadata, "\n") {
		if value, ok := strings.CutPrefix(line, "## Version:"); ok {
			return strings.TrimSpace(value)
		}
	}
	panic("ForeverDubbed.toc is missing its Version metadata")
}
