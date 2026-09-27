package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/console"
	"foreverdubbed/internal/updaterui"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		console.Prepare()
		fmt.Printf("ForeverDubbed updater %s\n", buildinfo.Version)
		return
	}
	if len(os.Args) != 2 {
		log.Fatal("expected the update plan filename")
	}
	if err := updaterui.Run(os.Args[1]); err != nil {
		log.Printf("Update failed: %v", err)
		message := fmt.Sprintf("The update could not finish.\n\n%v\n\nUpdate files and logs: %s", err, filepath.Dir(os.Args[1]))
		if dialogErr := updaterui.ShowError(message); dialogErr != nil {
			log.Printf("Show update error: %v", dialogErr)
		}
		os.Exit(1)
	}
}
