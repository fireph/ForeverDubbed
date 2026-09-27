package speech

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Speech text processing, in order:
//   - local.go selects dialogue/title text; narration.go routes <asides>.
//   - normalizeSpeechText below normalizes punctuation, removes stutter prefixes,
//     and normalizes capitals, in that order.
//   - chunks.go folds whitespace and splits text for streaming.
//   - pocket/pocket_tts.hpp prepare_text removes double quotes/backticks and edge
//     apostrophes, trims whitespace, capitalizes the first letter, fixes terminal
//     punctuation, and pads short utterances for the model.
//
// Keep application pronunciation replacements here. They affect only the copy
// sent to synthesis, never the original protocol message or displayed dialogue.

// Keep identifiers (SI:7, X-52) and hyphenated words together so a replacement
// cannot accidentally modify just part of one. Apostrophes preserve contractions.
var speechWord = regexp.MustCompile(`[\p{L}\p{N}]+(?:[':\-][\p{L}\p{N}]+)*`)

// Typographic punctuation -> plain equivalents. Keep apostrophes in names and
// contractions, and keep ellipses as pauses rather than replacing them with a
// single period. Dashes/hyphens retain their original meaning.
var speechPunctuation = strings.NewReplacer(
	"‘", "'",
	"’", "'",
	"“", `"`,
	"”", `"`,
	"…", "...",
)

// Collapse only identical adjacent marks: !!! -> !, ??? -> ?. Mixed punctuation
// such as ?! and !? keeps its order and both marks; periods are untouched.
var repeatedSpeechPunctuation = regexp.MustCompile(`!{2,}|\?{2,}`)

// Capitalization alone cannot distinguish acronyms from shouted words. Keep
// exceptions scoped to WoW dialogue/game terminology and Roman numerals used in
// names and ranks. Add new exceptions when encountered in WoW Forever dialogue;
// mixed-case names and single letters (including I, V, X) are always kept.
var speechAcronyms = map[string]bool{
	// Game terminology and abbreviations used in orders or dialogue.
	"GM": true, "HP": true, "HQ": true, "ID": true, "NPC": true,
	"PVE": true, "PVP": true, "SI": true, "SOS": true, "XP": true,

	// Roman numerals through XX; single-letter numerals need no exception.
	"II": true, "III": true, "IV": true, "VI": true, "VII": true,
	"VIII": true, "IX": true, "XI": true, "XII": true, "XIII": true,
	"XIV": true, "XV": true, "XVI": true, "XVII": true, "XVIII": true,
	"XIX": true, "XX": true,
}

func normalizeSpeechText(text string) string {
	text = speechPunctuation.Replace(text)
	text = repeatedSpeechPunctuation.ReplaceAllStringFunc(text, func(marks string) string {
		return marks[:1]
	})
	var out strings.Builder
	start, sentenceStart := 0, true
	for _, match := range speechWord.FindAllStringIndex(text, -1) {
		gap := text[start:match[0]]
		out.WriteString(gap)
		if strings.ContainsAny(gap, ".!?\n\r") {
			sentenceStart = true
		}
		word := removeStutter(text[match[0]:match[1]])
		if isUppercaseWord(word) && !speechAcronyms[word] {
			word = strings.ToLower(word)
			// Preserve the pronoun in uppercase contractions: I'M -> I'm.
			if sentenceStart || strings.HasPrefix(word, "i'") {
				first, size := utf8.DecodeRuneInString(word)
				word = string(unicode.ToUpper(first)) + word[size:]
			}
		}
		out.WriteString(word)
		sentenceStart = false
		start = match[1]
	}
	out.WriteString(text[start:])
	return out.String()
}

// T-That's -> That's; W-W-What -> What. Only matching single-letter prefixes
// qualify. Leave ordinary compounds, identifiers, and whole-word repeats intact.
func removeStutter(word string) string {
	parts := strings.Split(word, "-")
	if len(parts) < 2 {
		return word
	}
	last := parts[len(parts)-1]
	first, size := utf8.DecodeRuneInString(last)
	if !unicode.IsLetter(first) || len(last) == size || strings.ContainsAny(last, "0123456789:") {
		return word
	}
	for _, prefix := range parts[:len(parts)-1] {
		if utf8.RuneCountInString(prefix) != 1 || !strings.EqualFold(prefix, string(first)) {
			return word
		}
	}
	return last
}

func isUppercaseWord(word string) bool {
	letters := 0
	for _, r := range word {
		if r == '\'' {
			continue
		}
		if !unicode.IsUpper(r) {
			return false
		}
		letters++
	}
	return letters > 1 // Preserve I and standalone letters.
}
