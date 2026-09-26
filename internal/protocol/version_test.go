package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAddonVersion(t *testing.T) {
	for _, m := range []Message{
		{Kind: KindVersion, AddonVersion: "v0.8.3"},
		{Kind: 2, Text: strings.Repeat("dialogue", 500), Objectives: "objective", AddonVersion: "v0.8.3"},
		{Kind: KindStop, AddonVersion: "v0.8.3"},
		{Kind: 1, Text: "legacy"},
	} {
		var assembler Assembler
		var got *Message
		for _, frame := range framesFor(t, m) {
			packet, err := Parse(frame)
			if err != nil {
				t.Fatal(err)
			}
			got, err = assembler.Add(packet, time.Now())
			if err != nil {
				t.Fatal(err)
			}
		}
		if got == nil || *got != m {
			t.Fatalf("got %+v want %+v", got, m)
		}
	}
	data, err := json.Marshal(Message{AddonVersion: "v0.8.3"})
	if err != nil || !strings.Contains(string(data), `"v":"v0.8.3"`) {
		t.Fatalf("JSON: %s %v", data, err)
	}
	if _, err := Encode(Message{AddonVersion: "bad\x00version"}); err == nil {
		t.Fatal("accepted NUL")
	}
}
