// Package identity resolves observed NPC metadata before voice selection.
package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	racedata "foreverdubbed/data"
	"foreverdubbed/internal/protocol"
)

type Appearance struct {
	Race   string `json:"race"`
	Gender string `json:"gender,omitempty"`
}

// Observations document provenance without fixing an NPC type to one gender.
type NPC struct {
	Race                  string `json:"race"`
	Name                  string `json:"name,omitempty"`
	ObservedGender        string `json:"observed_gender,omitempty"`
	ObservedModelID       uint32 `json:"observed_model_id,omitempty"`
	ObservedDisplayStatus string `json:"observed_display_status,omitempty"`
}

type Custom struct {
	Source string                `json:"source,omitempty"`
	NPCs   map[string]NPC        `json:"npcs"`
	Models map[string]Appearance `json:"models"`
}

type Resolver struct {
	displays, models map[string]Appearance
	custom           Custom
}

// DefaultCustomPath finds an editable companion file, including macOS bundles.
// If none exists, Load uses the embedded copy.
func DefaultCustomPath() string {
	exe, _ := os.Executable()
	for _, p := range []string{
		filepath.Join(filepath.Dir(exe), "..", "Resources", "data", "custom-races.json"),
		filepath.Join(filepath.Dir(exe), "data", "custom-races.json"),
		"data/custom-races.json",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func validID(id string) bool {
	n, err := strconv.ParseUint(id, 10, 32)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == id
}

func Load(customPath string) (*Resolver, error) {
	r := &Resolver{}
	if err := json.Unmarshal(racedata.VoiceOver, &r.displays); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(racedata.Models, &r.models); err != nil {
		return nil, err
	}
	data := racedata.Custom
	if customPath != "" {
		var err error
		data, err = os.ReadFile(customPath)
		if err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal(data, &r.custom); err != nil {
		return nil, err
	}
	for id, npc := range r.custom.NPCs {
		if !validID(id) || strings.TrimSpace(npc.Race) == "" {
			return nil, fmt.Errorf("invalid custom NPC mapping %q", id)
		}
	}
	for id, model := range r.custom.Models {
		if !validID(id) || strings.TrimSpace(model.Race) == "" || (model.Gender != "" && model.Gender != "male" && model.Gender != "female") {
			return nil, fmt.Errorf("invalid custom model mapping %q", id)
		}
	}
	return r, nil
}

// Resolve never changes the source datasets. Overrides travel with each message,
// so clearing one in WoW takes effect on the next dialogue without desktop state.
func (r *Resolver) Resolve(m protocol.Message) protocol.Message {
	if m.IsControl() {
		return m
	}
	display := r.displays[m.DisplayID]
	model, ok := r.custom.Models[m.ModelID]
	modelSource := "custom model inference"
	if !ok {
		model, modelSource = r.models[m.ModelID], "model appearance"
	}
	switch {
	case m.RaceOverride != "":
		m.Race, m.RaceSource = m.RaceOverride, "saved NPC override"
	case r.custom.NPCs[m.NPCID].Race != "":
		m.Race, m.RaceSource = r.custom.NPCs[m.NPCID].Race, "custom NPC mapping"
	case m.Race != "":
		m.RaceSource = "addon race"
	case display.Race != "":
		m.Race, m.RaceSource = display.Race, "display lookup (VoiceOver)"
	case model.Race != "":
		m.Race, m.RaceSource = model.Race, modelSource
	default:
		m.RaceSource = "unavailable"
	}
	if m.Gender != "male" && m.Gender != "female" {
		m.Gender = display.Gender
		if m.Gender == "" {
			m.Gender = model.Gender
		}
	}
	return m
}
