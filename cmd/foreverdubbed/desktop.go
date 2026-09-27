//go:build gui

package main

import (
	"context"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/console"
	"foreverdubbed/internal/desktop"
)

func runDesktop(ctx context.Context, stop context.CancelFunc, state *appstate.State, races []string, work func() error) error {
	return desktop.Run(ctx, stop, state, version, races, work)
}

const desktopEnabled = true

func prepareConsole() { console.Prepare() }
