//go:build !windows

package updaterui

import "github.com/ncruces/zenity"

func windowIconOptions() []zenity.Option { return nil }
