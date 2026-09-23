package speech

import (
	"foreverdubbed/internal/protocol"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestVoiceSelection(t *testing.T) {
	c, err := Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	c.NPCOverrides["4949"] = "orc_male"
	for _, tc := range []struct{ race, gender, npc, override, want string }{
		{"Human", "male", "", "", "human_male"},
		{"Skyborne", "male", "254100", "", "skyborne_male"},
		{"Skyborne Elf", "female", "", "", "skyborne_female"},
		{"Goblin", "male", "", "", "goblin_male"},
		{"Goblin", "female", "", "", "goblin_female"},
		{"Blood Elf", "female", "", "", "bloodelf_female"},
		{"Draenei", "male", "", "", "draenei_male"},
		{"Night Elf", "female", "", "", "nightelf_female"},
		{"Scourge", "male", "", "", "undead_male"},
		{"Orc", "", "", "", "orc_male"},
		{"Dragon", "female", "", "", "narrator_male"},
		{"", "", "", "", "narrator_male"},
		{"", "male", "", "", "narrator_male"},
		{"", "female", "", "", "narrator_male"},
		{"   ", "female", "", "", "narrator_male"},
		{"Human", "female", "", "", "human_female"},
		{"", "female", "4949", "", "orc_male"},
		{"Human", "female", "4949", "", "orc_male"},
		{"Human", "female", "4949", "gnome_female", "gnome_female"},
	} {
		got, err := c.Voice(protocol.Message{Race: tc.race, Gender: tc.gender, NPCID: tc.npc}, tc.override)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: %s %v", tc, got, err)
		}
	}
	if _, err := c.Voice(protocol.Message{}, "missing"); err == nil {
		t.Fatal("accepted unknown voice")
	}
}

func TestChunksUnicodeAndLimit(t *testing.T) {
	text := "Welcome, traveler! " + strings.Repeat("Café 世界 ", 60)
	chunks := Chunks(text, 180)
	if strings.Join(chunks, " ") != strings.Join(strings.Fields(text), " ") {
		t.Fatal("text lost")
	}
	for _, s := range chunks {
		if !utf8.ValidString(s) || utf8.RuneCountInString(s) > 180 {
			t.Fatal("invalid chunk")
		}
	}
	if got := Chunks(strings.Repeat("世", 401), 180); len(got) != 3 || strings.Join(got, "") != strings.Repeat("世", 401) {
		t.Fatal("long word lost")
	}
}
