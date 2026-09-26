package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"foreverdubbed/internal/identity"
	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/protocol"
)

func decodeImages(files string, identities *identity.Resolver, destination io.Writer) error {
	var assembler protocol.Assembler
	output := json.NewEncoder(destination)
	completed := 0
	for _, name := range strings.Split(files, ",") {
		f, err := os.Open(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		im, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return err
		}
		_, p, err := protocol.Find(im)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		m, err := assembler.Add(p, time.Now())
		if err != nil {
			return err
		}
		if m != nil {
			*m = identities.Resolve(*m)
			if err := output.Encode(m); err != nil {
				return err
			}
			completed++
		}
	}
	if completed == 0 {
		return fmt.Errorf("valid pages read, but no complete message; supply screenshots for every page")
	}
	return nil
}

func saveSnapshot(ctx context.Context, path string) error {
	log.Print("Taking a game-window snapshot in 3 seconds. Keep the game and square visible.")
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	im, err := platform.Capture(platform.Desktop())
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(f, im)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	log.Printf("Saved capture PNG %s (%d × %d).", path, im.Bounds().Dx(), im.Bounds().Dy())
	l, p, err := protocol.Find(im)
	if err != nil {
		return err
	}
	log.Printf("Valid tile at (%d,%d), %dpx cells; page %d/%d.", l.X, l.Y, l.Cell, p.Index+1, p.Count)
	return nil
}
