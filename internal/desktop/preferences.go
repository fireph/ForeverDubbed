//go:build gui

package desktop

import (
	"foreverdubbed/internal/appstate"
	"fyne.io/fyne/v2"
)

func loadSpeechFilters(p fyne.Preferences) appstate.SpeechFilters {
	return appstate.SpeechFilters{
		Quests:        p.BoolWithFallback("speakQuests", true),
		Conversations: p.BoolWithFallback("speakConversations", true),
		NPCSpeech:     p.BoolWithFallback("speakNPCSpeech", true),
	}
}

func saveSpeechFilters(p fyne.Preferences, f appstate.SpeechFilters) {
	p.SetBool("speakQuests", f.Quests)
	p.SetBool("speakConversations", f.Conversations)
	p.SetBool("speakNPCSpeech", f.NPCSpeech)
}
