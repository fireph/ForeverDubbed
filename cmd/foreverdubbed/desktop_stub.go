//go:build !gui

package main

import (
	"context"
	"foreverdubbed/internal/appstate"
)

func runDesktop(_ context.Context, _ context.CancelFunc, _ *appstate.State, _ []string, work func() error) error {
	return work()
}

const desktopEnabled = false

func prepareConsole() {}
