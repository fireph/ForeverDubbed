//go:build gui

package main

import (
	"fmt"
	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/desktop"
	"log"
	"os"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		desktop.PrepareConsole()
		fmt.Printf("ForeverDubbed updater %s\n", buildinfo.Version)
		return
	}
	if len(os.Args) != 2 {
		log.Fatal("expected the update plan filename")
	}
	if err := desktop.RunUpdater(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
