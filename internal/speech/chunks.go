package speech

import (
	"strings"
)

// Chunks keeps words intact where possible and prefers sentence boundaries.
func Chunks(text string, maxRunes int) []string {
	if maxRunes < 1 {
		return nil
	}
	var chunks []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			chunks = append(chunks, string(current))
			current = nil
		}
	}
	for _, word := range strings.Fields(text) {
		runes := []rune(word)
		if len(current) > 0 && len(current)+1+len(runes) > maxRunes {
			flush()
		}
		for len(runes) > maxRunes {
			flush()
			chunks = append(chunks, string(runes[:maxRunes]))
			runes = runes[maxRunes:]
		}
		if len(current) > 0 {
			current = append(current, ' ')
		}
		current = append(current, runes...)
		if len(current) >= 40 && strings.ContainsAny(string(current[len(current)-1]), ".!?;") {
			flush()
		}
	}
	flush()
	return chunks
}
