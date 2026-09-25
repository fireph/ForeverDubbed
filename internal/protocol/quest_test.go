package protocol

import (
	"testing"
	"time"
)

func TestQuestContentSelection(t *testing.T) {
	for _, kind := range []byte{2, 3, 4} {
		m := Message{Kind: kind, Speaker: "Thrall", Title: "Quest title", Text: "Main dialogue.", Objectives: "Collect ten supplies."}
		if m.DialogueText(false, false) != "Main dialogue." || m.Speech() != "Main dialogue." {
			t.Fatal("quest title, speaker or objectives leaked into default speech")
		}
		if m.DialogueText(false, true) != "Main dialogue.\n\nCollect ten supplies." {
			t.Fatal("missing optional objectives")
		}
		if m.DialogueText(true, false) != "Quest title.\n\nMain dialogue." {
			t.Fatal("title-only option changed dialogue or included objectives")
		}
		if m.DialogueText(true, true) != "Quest title.\n\nMain dialogue.\n\nCollect ten supplies." {
			t.Fatal("quest parts are missing or out of order")
		}
		if m.Title != "Quest title" || m.Objectives != "Collect ten supplies." {
			t.Fatal("raw metadata changed")
		}
	}
	m := Message{Kind: 1, Speaker: "Thrall", Title: "Greeting", Text: "Hello.", Objectives: "Ignore."}
	if m.DialogueText(false, true) != "Greeting. Hello." || m.Speech() != "Thrall. Greeting. Hello." {
		t.Fatal("non-quest speech changed")
	}
	if got := (Message{Kind: 2, Title: "A quest!"}).DialogueText(true, false); got != "A quest!" {
		t.Fatalf("title-only quest or punctuation changed: %q", got)
	}
}

func TestObjectivesWithoutIdentity(t *testing.T) {
	want := Message{Kind: 2, Title: "Quest", Objectives: "Find the captain."}
	var a Assembler
	got, err := a.Add(packetFor(t, framesFor(t, want)[0]), time.Now())
	if err != nil || got == nil || *got != want {
		t.Fatalf("objectives-only round trip: %+v %v", got, err)
	}
	if got.DialogueText(false, false) != "" || got.DialogueText(false, true) != want.Objectives {
		t.Fatal("empty dialogue selection")
	}
}
