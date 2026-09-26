package speech

import (
	"regexp"
	"strings"
)

type speechSegment struct{ text, name string }

var narratorAside = regexp.MustCompile(`<([^<>]*)>`)

// splitNarration removes paired angle brackets from spoken text and assigns
// their contents to the narrator. Unmatched brackets remain ordinary text.
func splitNarration(segment speechSegment, narrator string) []speechSegment {
	var result []speechSegment
	appendText := func(text, voice string) {
		if text = strings.TrimSpace(text); text != "" {
			result = append(result, speechSegment{text, voice})
		}
	}
	start := 0
	for _, match := range narratorAside.FindAllStringSubmatchIndex(segment.text, -1) {
		appendText(segment.text[start:match[0]], segment.name)
		appendText(segment.text[match[2]:match[3]], narrator)
		start = match[1]
	}
	appendText(segment.text[start:], segment.name)
	return result
}
