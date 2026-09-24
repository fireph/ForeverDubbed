// Package racedata embeds the desktop's independent identity datasets.
package racedata

import _ "embed"

// VoiceOver contains the unchanged identities from the pinned upstream snapshot.
//
//go:embed voiceover-display.json
var VoiceOver []byte

//go:embed model-races.json
var Models []byte

//go:embed custom-races.json
var Custom []byte
