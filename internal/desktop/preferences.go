//go:build gui

package desktop

import (
	"encoding/json"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/speech"
	"fyne.io/fyne/v2"
)

func loadSpeechFilters(p fyne.Preferences) appstate.SpeechFilters {
	return appstate.SpeechFilters{
		Quests:          p.BoolWithFallback("speakQuests", true),
		Conversations:   p.BoolWithFallback("speakConversations", true),
		NPCSpeech:       p.BoolWithFallback("speakNPCSpeech", true),
		QuestObjectives: p.Bool("speakQuestObjectives"),
		QuestTitle:      p.Bool("speakQuestTitle"),
	}
}

func saveSpeechFilters(p fyne.Preferences, f appstate.SpeechFilters) {
	p.SetBool("speakQuests", f.Quests)
	p.SetBool("speakConversations", f.Conversations)
	p.SetBool("speakNPCSpeech", f.NPCSpeech)
	p.SetBool("speakQuestObjectives", f.QuestObjectives)
	p.SetBool("speakQuestTitle", f.QuestTitle)
}

// Voice choices are stored as one JSON map of "race:gender" → "narrator"/"none";
// only non-default selections are persisted.
func loadVoiceChoices(p fyne.Preferences) map[string]string {
	choices := map[string]string{}
	if raw := p.String("voiceChoices"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &choices); err != nil || choices == nil {
			return map[string]string{}
		}
	}
	for key, value := range choices {
		if value != speech.VoiceNarrator && value != speech.VoiceNone {
			delete(choices, key)
		}
	}
	return choices
}

func saveVoiceChoices(p fyne.Preferences, choices map[string]string) {
	data, err := json.Marshal(choices)
	if err != nil {
		return
	}
	p.SetString("voiceChoices", string(data))
}
