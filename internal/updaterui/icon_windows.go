package updaterui

import (
	"log"
	"os"

	_ "foreverdubbed/internal/appicon/windowsresource"

	"github.com/ncruces/zenity"
)

func windowIconOptions() []zenity.Option {
	// The updater is copied to a staging folder before launch. Read its own
	// embedded icon so the dialog never depends on files being replaced.
	executable, err := os.Executable()
	if err != nil {
		log.Printf("Locate updater icon: %v", err)
		return nil
	}
	return []zenity.Option{zenity.WindowIcon(executable)}
}
