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
	QueueSpeech                           bool
	Queued                                int
	PlaybackID                            uint64
	Played, Duration                      time.Duration
	DurationKnown                         bool
	PlayingSpeaker                        string
	Voice                                 string
	Message                               protocol.Message
	Received                              time.Time
}

type State struct {
	mu           sync.RWMutex
	value        Snapshot
	stopAudio    chan uint64
	queueChanges chan struct{}
}

func New(target, backend string, muted bool) *State {
	audio := "Starting"
	if muted {
		audio = "Muted"
	}
	return &State{stopAudio: make(chan uint64, 1), queueChanges: make(chan struct{}, 1), value: Snapshot{Target: target, Backend: backend, Audio: audio}}
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
		v.PlaybackID = 0
		if err != nil {
			v.FatalError = err.Error()
		}
	})
}

// StopAudio requests cancellation of the utterance visible when clicked.
// IDs prevent a delayed click from interrupting its replacement.
func (s *State) StopAudio() {
	id := s.Snapshot().PlaybackID
	if id == 0 {
		return
	}
	select {
	case s.stopAudio <- id:
	default:
	}
}
func (s *State) AudioStops() <-chan uint64 { return s.stopAudio }
func (s *State) ResetPlayback() {
	s.Update(func(v *Snapshot) {
		v.Audio = "Idle"
		v.PlaybackID = 0
		v.Played, v.Duration = 0, 0
		v.DurationKnown = false
		v.PlayingSpeaker = ""
	})
}

// SetQueueSpeech wakes the speech worker so mode changes apply immediately.
func (s *State) SetQueueSpeech(enabled bool) {
	s.Update(func(v *Snapshot) { v.QueueSpeech = enabled })
	select {
	case s.queueChanges <- struct{}{}:
	default:
	}
}
func (s *State) QueueChanges() <-chan struct{} { return s.queueChanges }
