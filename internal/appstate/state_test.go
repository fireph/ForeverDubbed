package appstate

import (
	"errors"
	"foreverdubbed/internal/protocol"
	"sync"
	"testing"
)

func TestCaptureRecoveryAndShutdown(t *testing.T) {
	s := New("WoWB.exe", "pocket", false)
	s.Capture(true, true, nil)
	s.Capture(false, true, errors.New("game closed"))
	if v := s.Snapshot(); v.Window || v.Tile || v.CaptureError == "" {
		t.Fatal(v)
	}
	s.Capture(true, false, nil)
	if v := s.Snapshot(); !v.Window || v.Tile || v.CaptureError != "" {
		t.Fatal(v)
	}
	s.Audio("Playing audio")
	s.Finished(errors.New("failed"))
	if v := s.Snapshot(); v.Window || v.Tile || !v.Stopped || v.Audio != "Stopped" || v.FatalError != "failed" {
		t.Fatal(v)
	}
}
func TestConcurrentStatusKeepsLastDialogue(t *testing.T) {
	s := New("game", "pocket", false)
	m := protocol.Message{Speaker: "Thrall", Text: "Welcome, traveler."}
	s.Received(m)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				s.Capture(true, true, nil)
				s.Audio("Playing audio")
				_ = s.Snapshot()
			}
		}()
	}
	wg.Wait()
	if v := s.Snapshot(); v.Message != m || v.Received.IsZero() {
		t.Fatal(v)
	}
}

func TestSpeechCategoryMapping(t *testing.T) {
	for _, tc := range []struct {
		filters SpeechFilters
		allowed []byte
	}{
		{SpeechFilters{}, []byte{0, 6, protocol.KindStop, protocol.KindSkip}},
		{SpeechFilters{Quests: true}, []byte{0, 2, 3, 4, 6, 7, 8}},
		{SpeechFilters{Conversations: true}, []byte{0, 1, 6, 7, 8}},
		{SpeechFilters{NPCSpeech: true}, []byte{0, 5, 6, 7, 8}},
	} {
		for kind := byte(0); kind <= 8; kind++ {
			want := false
			for _, k := range tc.allowed {
				if k == kind {
					want = true
				}
			}
			if tc.filters.Allows(kind) != want {
				t.Fatalf("filters=%+v kind=%d", tc.filters, kind)
			}
		}
	}
	defaults := New("game", "pocket", false).Snapshot().Filters
	for kind := byte(0); kind <= 8; kind++ {
		if !defaults.Allows(kind) {
			t.Fatalf("default muted kind %d", kind)
		}
	}
}
