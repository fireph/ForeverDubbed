package buildinfo

import "foreverdubbed/addon"

// Version is overridden by the release tag through -ldflags -X. For ordinary
// builds, initialization reads the embedded addon metadata instead.
var Version string

func init() {
	if Version == "" {
		Version = addon.Version()
	}
}
