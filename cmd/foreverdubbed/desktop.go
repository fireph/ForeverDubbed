//go:build gui

package main

import (
	"context"
	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/desktop"
)

func runDesktop(ctx context.Context, stop context.CancelFunc, state *appstate.State, work func() error) error {
	return desktop.Run(ctx, stop, state, version, work)
}

const desktopEnabled = true

func prepareConsole() { desktop.PrepareConsole() }
