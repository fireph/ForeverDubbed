package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/identity"
	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/protocol"
)

type captureSettings struct {
	poll, scan     time.Duration
	emitJSON, mute bool
}

func captureLoop(ctx context.Context, settings captureSettings, state *appstate.State, identities *identity.Resolver, speak func(context.Context, protocol.Message) error) error {
	var assembler protocol.Assembler
	output := json.NewEncoder(os.Stdout)
	// A single worker applies the selected queue/interrupt policy.
	speech := make(chan protocol.Message, 1)
	done := make(chan struct{})
	speechCtx, cancelSpeech := context.WithCancel(ctx)
	go func() { defer close(done); speakLoop(speechCtx, speech, speak, state) }()
	defer func() { cancelSpeech(); <-done }()
	log.Printf("ForeverDubbed %s (FDB5, 16 colors). Searching; in WoW: /fdb unlock. Ctrl+C to quit.", version)
	var location *protocol.Location
	failures := 0
	var lastError time.Time
	for ctx.Err() == nil {
		delay := settings.poll
		var p protocol.Packet
		windowOK, tileOK := false, false
		var err error
		if location == nil {
			delay = settings.scan
			im, captureErr := platform.Capture(platform.Desktop())
			if captureErr != nil {
				err = captureErr
			} else {
				windowOK = true
				l, packet, findErr := protocol.Find(im)
				if findErr == nil {
					tileOK = true
					location = &l
					p = packet
					failures = 0
					delay = settings.poll
					log.Printf("Found tile at (%d, %d), %dpx cells, %d bytes/page.", l.X, l.Y, l.Cell, protocol.PayloadBytes)
				} else {
					err = findErr
				}
			}
		} else {
			im, captureErr := platform.Capture(location.Rect())
			if captureErr != nil {
				err = captureErr
			} else {
				windowOK = true
				p, err = protocol.Decode(im, *location)
				tileOK = err == nil
			}
			if err != nil {
				failures++
				if failures >= 4 {
					location = nil
					log.Print("Tile lost; searching again.")
				}
			} else {
				failures = 0
			}
		}
		state.Capture(windowOK, tileOK, err)
		if err == nil {
			m, assemblyErr := assembler.Add(p, time.Now())
			if assemblyErr != nil {
				log.Printf("Discarding message: %v", assemblyErr)
			}
			if m != nil {
				*m = identities.Resolve(*m)
				if !m.IsControl() {
					state.Received(*m)
				}
				if settings.emitJSON {
					if err := output.Encode(m); err != nil {
						return err
					}
				}
				if !settings.mute {
					select {
					case speech <- *m:
					case <-ctx.Done():
						return nil
					}
				}
			}
		} else if time.Since(lastError) > 15*time.Second {
			log.Printf("Waiting: %v", err)
			lastError = time.Now()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return nil
}
