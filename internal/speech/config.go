package speech

import (
	"encoding/json"
	"fmt"
	"foreverdubbed/internal/protocol"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type Profile struct {
	Voice       string  `json:"voice"`
	DecodeSteps int     `json:"decode_steps"`
	GainDB      float64 `json:"gain_db,omitempty"`
	FadeInMS    int     `json:"fade_in_ms,omitempty"`
	FadeOutMS   int     `json:"fade_out_ms,omitempty"`
}

type Config struct {
	BaseDir      string                       `json:"-"`
	Default      map[string]string            `json:"default"`
	Races        map[string]map[string]string `json:"races"`
	NPCOverrides map[string]string            `json:"npc_overrides"`
	Profiles     map[string]Profile           `json:"profiles"`
}

func DefaultConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "tts/voices.json"
	}
	for _, p := range []string{filepath.Join(filepath.Dir(exe), "..", "Resources", "tts", "voices.json"), filepath.Join(filepath.Dir(exe), "tts", "voices.json"), filepath.Join(filepath.Dir(exe), "..", "tts", "voices.json"), "tts/voices.json"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(filepath.Dir(exe), "tts", "voices.json")
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	c.BaseDir = filepath.Dir(abs)
	for id, profile := range c.Profiles {
		if profile.FadeInMS < 0 || profile.FadeInMS > 500 || profile.FadeOutMS < 0 || profile.FadeOutMS > 500 {
			return nil, fmt.Errorf("profile %q fade_in_ms and fade_out_ms must be 0..500", id)
		}
		if math.IsNaN(profile.GainDB) || math.IsInf(profile.GainDB, 0) || profile.GainDB < -24 || profile.GainDB > 12 {
			return nil, fmt.Errorf("profile %q gain_db must be between -24 and 12", id)
		}
		if profile.DecodeSteps < 0 || profile.DecodeSteps > 64 {
			return nil, fmt.Errorf("profile %q decode_steps must be 1..64", id)
		}
		if strings.TrimSpace(profile.Voice) == "" {
			return nil, fmt.Errorf("profile %q needs a Pocket TTS voice", id)
		}
	}
	check := func(voice string) error {
		if _, ok := c.Profiles[voice]; !ok {
			return fmt.Errorf("unknown voice profile %q", voice)
		}
		return nil
	}
	for _, gender := range []string{"male", "female", "unknown"} {
		if err := check(c.Default[gender]); err != nil {
			return nil, err
		}
	}
	for _, mapping := range c.Races {
		for _, voice := range mapping {
			if err := check(voice); err != nil {
				return nil, err
			}
		}
	}
	for _, voice := range c.NPCOverrides {
		if err := check(voice); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

func raceKey(race string) string {
	s := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, race)
	if s == "skyborneelf" || s == "skyborneelves" {
		return "skyborne"
	}
	if s == "scourge" {
		return "undead"
	}
	return s
}

func (c *Config) Voice(m protocol.Message, override string) (string, error) {
	voice := override
	if voice == "" {
		voice = c.NPCOverrides[m.NPCID]
	}
	gender := strings.ToLower(m.Gender)
	if gender != "male" && gender != "female" {
		gender = "unknown"
	}
	if voice == "" {
		mapping := c.Races[raceKey(m.Race)]
		voice = mapping[gender]
		if voice == "" {
			voice = mapping["unknown"]
		}
	}
	if voice == "" {
		voice = c.Default[gender]
	}
	if _, ok := c.Profiles[voice]; !ok {
		return "", fmt.Errorf("unknown voice %q", voice)
	}
	return voice, nil
}
