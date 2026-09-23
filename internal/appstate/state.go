// Package appstate shares runtime status without coupling capture or speech to a GUI.
package appstate

import (
	"sync"
	"time"

	"foreverdubbed/internal/protocol"
)

type Snapshot struct {
	Target, Backend                       string
	Ready, Stopped                        bool
	Window, Tile                          bool
	CaptureError, SpeechError, FatalError string
	Audio                                 string
	Voice                                 string
	Message                               protocol.Message
	Received                              time.Time
}

type State struct {
	mu    sync.RWMutex
	value Snapshot
}

func New(target, backend string, muted bool) *State {
	audio := "Starting"
	if muted {
		audio = "Muted"
	}
	return &State{value: Snapshot{Target: target, Backend: backend, Audio: audio}}
}
func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}
func (s *State) Update(fn func(*Snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.value)
}
func (s *State) Capture(window, tile bool, err error) {
	s.Update(func(v *Snapshot) {
		v.Window, v.Tile = window, window && tile
		v.CaptureError = ""
		if err != nil {
			v.CaptureError = err.Error()
		}
	})
}
func (s *State) Received(m protocol.Message) {
	s.Update(func(v *Snapshot) { v.Message, v.Received = m, time.Now() })
}
func (s *State) Audio(phase string) {
	s.Update(func(v *Snapshot) { v.Audio = phase })
}
func (s *State) Finished(err error) {
	s.Update(func(v *Snapshot) {
		v.Stopped = true
		v.Ready, v.Window, v.Tile = false, false, false
		v.Audio = "Stopped"
		if err != nil {
			v.FatalError = err.Error()
		}
	})
}
