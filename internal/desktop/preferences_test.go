//go:build gui

package desktop

import (
	"foreverdubbed/internal/appstate"
	"fyne.io/fyne/v2/test"
	"testing"
)

func TestSpeechFilterPreferences(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	p := a.Preferences()
	if got := loadSpeechFilters(p); got != (appstate.SpeechFilters{Quests: true, Conversations: true, NPCSpeech: true}) {
		t.Fatal("missing preferences must enable all categories", got)
	}
	want := appstate.SpeechFilters{Conversations: true}
	saveSpeechFilters(p, want)
	if got := loadSpeechFilters(p); got != want {
		t.Fatalf("saved=%+v loaded=%+v", want, got)
	}
	// An older install with only one stored key preserves enabled defaults.
	p.RemoveValue("speakNPCSpeech")
	if got := loadSpeechFilters(p); !got.NPCSpeech || got.Quests {
		t.Fatal("incorrect preference migration", got)
	}
}
