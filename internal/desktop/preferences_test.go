//go:build gui

package desktop

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"foreverdubbed/internal/appstate"
	"fyne.io/fyne/v2/test"
)

func TestSpeechFilterPreferences(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	p := a.Preferences()
	if got := loadSpeechFilters(p); got != (appstate.SpeechFilters{Quests: true, Conversations: true, NPCSpeech: true}) {
		t.Fatal("missing preferences must enable all categories", got)
	}
	want := appstate.SpeechFilters{Conversations: true, QuestObjectives: true, QuestTitle: true}
	saveSpeechFilters(p, want)
	if got := loadSpeechFilters(p); got != want {
		t.Fatalf("saved=%+v loaded=%+v", want, got)
	}
	// An older install with only one stored key preserves enabled defaults.
	p.RemoveValue("speakNPCSpeech")
	if got := loadSpeechFilters(p); !got.NPCSpeech || got.Quests {
		t.Fatal("incorrect preference migration", got)
	}
	p.RemoveValue("speakQuestObjectives")
	p.RemoveValue("speakQuestTitle")
	if got := loadSpeechFilters(p); got.QuestObjectives || got.QuestTitle {
		t.Fatal("quest extras must default off for older installs")
	}
}

func TestVoiceChoicePreferences(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	p := a.Preferences()
	if got := loadVoiceChoices(p); len(got) != 0 {
		t.Fatal("missing preferences must select defaults", got)
	}
	want := map[string]string{"orc:male": "none", "human:female": "narrator"}
	saveVoiceChoices(p, want)
	if got := loadVoiceChoices(p); !maps.Equal(got, want) {
		t.Fatalf("saved=%v loaded=%v", want, got)
	}
	// Corrupt or legacy values fall back to defaults instead of breaking speech.
	p.SetString("voiceChoices", "{not-json")
	if got := loadVoiceChoices(p); len(got) != 0 {
		t.Fatal("corrupt JSON must select defaults", got)
	}
	p.SetString("voiceChoices", "null")
	if got := loadVoiceChoices(p); len(got) != 0 {
		t.Fatal("null JSON must select defaults", got)
	}
	p.SetString("voiceChoices", `{"orc:male":"none","human:female":"obsolete","gnome:male":"default"}`)
	if got := loadVoiceChoices(p); !maps.Equal(got, map[string]string{"orc:male": "none"}) {
		t.Fatal("invalid selections must fall back to defaults", got)
	}
}

func TestCaptureTargetPreferences(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	p := a.Preferences()
	root := t.TempDir()
	game := filepath.Join(root, "WowB.exe")
	if err := os.WriteFile(game, nil, 0600); err != nil {
		t.Fatal(err)
	}
	p.SetString("wowExecutable", game)
	state := appstate.New("WowB.exe", "pocket", false)
	restoreCaptureTarget(p, state, "windows")
	if state.Snapshot().Target != game {
		t.Fatal("saved game was not selected")
	}
	state.SetCaptureTarget("explicit.exe")
	state.Update(func(s *appstate.Snapshot) { s.CaptureTargetExplicit = true })
	restoreCaptureTarget(p, state, "windows")
	if state.Snapshot().Target != "explicit.exe" {
		t.Fatal("saved selection replaced explicit CLI target")
	}
	state.Update(func(s *appstate.Snapshot) { s.CaptureTargetExplicit = false })
	if err := os.Remove(game); err != nil {
		t.Fatal(err)
	}
	restoreCaptureTarget(p, state, "windows")
	if state.Snapshot().Target != "explicit.exe" {
		t.Fatal("missing saved executable was selected")
	}
}
