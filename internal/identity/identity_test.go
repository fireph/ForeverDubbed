package identity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	racedata "foreverdubbed/data"
	"foreverdubbed/internal/protocol"
	"foreverdubbed/internal/speech"
)

func TestResolutionAndVoiceSelection(t *testing.T) {
	r, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	voices, err := speech.Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                 string
		in                   protocol.Message
		race, gender, source string
	}{
		{"confirmed overrides appearance", protocol.Message{NPCID: "254100", DisplayID: "176", ModelID: "1100258", Gender: "male"}, "Skyborne", "male", "custom NPC mapping"},
		{"generic female", protocol.Message{NPCID: "254100", Gender: "female"}, "Skyborne", "female", "custom NPC mapping"},
		{"NPC does not fix gender", protocol.Message{NPCID: "254100"}, "Skyborne", "", "custom NPC mapping"},
		{"user overrides all", protocol.Message{NPCID: "254100", Race: "Orc", RaceOverride: "Goblin", Gender: "female", DisplayID: "176"}, "Goblin", "female", "saved NPC override"},
		{"API before display", protocol.Message{Race: "Orc", DisplayID: "176", Gender: "male"}, "Orc", "male", "addon race"},
		{"legacy VoiceOver", protocol.Message{DisplayID: "176", ModelID: "119376"}, "Human", "female", "display lookup (VoiceOver)"},
		{"old model fallback", protocol.Message{ModelID: "119376"}, "Goblin", "male", "model appearance"},
		{"adult inference", protocol.Message{NPCID: "999999", ModelID: "7478487"}, "Skyborne", "male", "custom model inference"},
		{"female inference", protocol.Message{ModelID: "7478494"}, "Skyborne", "female", "custom model inference"},
		{"unknown child model", protocol.Message{ModelID: "7865151"}, "", "", "unavailable"},
		{"confirmed child", protocol.Message{NPCID: "267936", ModelID: "7865151", Gender: "male"}, "Skyborne", "male", "custom NPC mapping"},
		{"IDs have distinct namespaces", protocol.Message{NPCID: "176", DisplayID: "7478487", ModelID: "254100"}, "", "", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Exercise the same raw transport -> desktop lookup -> voice path.
			frames, err := protocol.Encode(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			var assembler protocol.Assembler
			packet, err := protocol.Parse(frames[0])
			if err != nil {
				t.Fatal(err)
			}
			m, err := assembler.Add(packet, time.Now())
			if err != nil || m == nil {
				t.Fatalf("decode: %v", err)
			}
			got := r.Resolve(*m)
			if got.Race != tc.race || got.Gender != tc.gender || got.RaceSource != tc.source {
				t.Fatalf("got %+v", got)
			}
			if got.Race == "Skyborne" {
				voice, err := voices.Voice(got, "")
				want := "skyborne_male"
				if got.Gender == "female" {
					want = "skyborne_female"
				}
				if err != nil || voice != want {
					t.Fatalf("voice=%s error=%v", voice, err)
				}
			}
		})
	}
	// A previous override does not survive a subsequent cleared message.
	r.Resolve(protocol.Message{NPCID: "254100", RaceOverride: "Goblin"})
	if r.Resolve(protocol.Message{NPCID: "254100"}).Race != "Skyborne" {
		t.Fatal("stale override")
	}
	control := protocol.Message{Kind: protocol.KindSkip}
	if r.Resolve(control) != control {
		t.Fatal("changed control packet")
	}
}

func TestDataSeparationAndCustomConfig(t *testing.T) {
	r, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.displays) != 15444 || len(r.custom.NPCs) != 51 {
		t.Fatal("lost imported records")
	}
	if r.displays["115"].Race != "Dwarf" || r.displays["6882"].Race != "Goblin" {
		t.Fatal("changed upstream identities")
	}
	for _, entry := range r.displays {
		if entry.Race == "Skyborne" {
			t.Fatal("modified VoiceOver data")
		}
	}
	before := string(racedata.VoiceOver)
	path := filepath.Join(t.TempDir(), "custom.json")
	config := Custom{NPCs: map[string]NPC{"254100": {Race: "Troll"}}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	edited, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Resolve(protocol.Message{NPCID: "254100"}).Race != "Troll" {
		t.Fatal("custom file ignored")
	}
	if edited.Resolve(protocol.Message{NPCID: "254100", RaceOverride: "Skyborne"}).Race != "Skyborne" {
		t.Fatal("custom file overrode slash command")
	}
	if string(racedata.VoiceOver) != before {
		t.Fatal("mutated source")
	}
	for _, bad := range []string{`{`, `{"npcs":{"bad":{"race":"Human"}}}`, `{"models":{"7478487":{"race":"Skyborne","gender":"invalid"}}}`} {
		os.WriteFile(path, []byte(bad), 0600)
		if _, err := Load(path); err == nil {
			t.Fatal("accepted invalid config", bad)
		}
	}
	if _, err := Load(path + "missing"); err == nil {
		t.Fatal("ignored explicit missing config")
	}
}
