package speech

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeSpeechText(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"GET OUT!", "Get out!"},
		{"That is AMAZING!", "That is amazing!"},
		{"T-That's", "That's"},
		{"W-What?", "What?"},
		{"W-W-What?", "What?"},
		{"t-That's AMAZING!", "That's amazing!"},
		{"T-THAT’S AMAZING! W-W-WHAT?", "That's amazing! What?"},
		{"SI:7, NPC, ID, HP, HQ, GM, PVE, PVP, SOS, XP, X-52", "SI:7, NPC, ID, HP, HQ, GM, PVE, PVP, SOS, XP, X-52"},
		{"I II III IV V VI VII VIII IX X XI XII XIII XIV XV XVI XVII XVIII XIX XX", "I II III IV V VI VII VIII IX X XI XII XIII XIV XV XVI XVII XVIII XIX XX"},
		{"Words: AI API CPU DNA EU FBI FYI GPS GPU MP NASA OK PC RPG TV UK USA USB.", "Words: ai api cpu dna eu fbi fyi gps gpu mp nasa ok pc rpg tv uk usa usb."},
		{"well-known mother-in-law T-shirt X-ray A-Team", "well-known mother-in-law T-shirt X-ray A-Team"},
		{"W-W, T-What, no-no, ha-ha, W-What-ever", "W-W, T-What, no-no, ha-ha, W-What-ever"},
		{"Thrall and McDonald agree. STOP! NOW? YES.", "Thrall and McDonald agree. Stop! Now? Yes."},
		{"\"GET OUT!\"\nDON'T RETURN.", "\"Get out!\"\nDon't return."},
		{"Well, I'M SURE I’LL GO.", "Well, I'm sure I'll go."},
		{"Stop!!! What??? Really?! Yes!?", "Stop! What? Really?! Yes!?"},
		{"What??!! No!!??", "What?! No!?"},
		{"Wait... wait… wait....", "Wait... wait... wait...."},
		{"‘Don’t,’ she said. “It’s Vol’jin!”", "'Don't,' she said. \"It's Vol'jin!\""},
		{"T-THAT‘S AMAZING!!! W-W-WHAT???", "That's amazing! What?"},
		{"Well-known—really–well-known; 3.14, SI:7, X-52.", "Well-known—really–well-known; 3.14, SI:7, X-52."},
		{"!!!???…", "!?..."},
		{"É-ÉCOUTE!", "Écoute!"},
		{"", ""},
		{" \t...!?", " \t...!?"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeSpeechText(tc.input)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if again := normalizeSpeechText(got); again != got {
				t.Fatalf("normalizing twice changed %q to %q", got, again)
			}
		})
	}
}

func TestNormalizeSpeechDialogue(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{
			"quoted contractions and stutters",
			"“T-T-THAT’S IT!!!” Vol’jin shouted. “I’M NOT LEAVING… W-W-WHAT??!!”",
			"\"That's it!\" Vol'jin shouted. \"I'm not leaving... What?!\"",
		},
		{
			"names ranks and identifiers",
			"Tell Vol’jin and Kael’thas: SI:7 NEEDS HELP!!! King Terenas II sent X-52 to HQ. I WON’T WAIT???",
			"Tell Vol'jin and Kael'thas: SI:7 needs help! King Terenas II sent X-52 to HQ. I won't wait?",
		},
		{
			"paragraph boundaries and dialogue punctuation",
			"  STOP!!!\r\n\t‘W-W-WAIT,’ he said; ‘DON’T GO.’\n(RETURN NOW???)  ",
			"  Stop!\r\n\t'Wait,' he said; 'don't go.'\n(Return now?)  ",
		},
		{
			"hesitation and mixed punctuation",
			"Wait... I… I can explain.... Really?!?! No!!?? Don't leave—please!",
			"Wait... I... I can explain.... Really?!?! No!? Don't leave—please!",
		},
		{
			"matching and nonmatching fragments",
			"w-W-What? T-That’s fine, but T-What, W-W, well-known, ha-ha, X-X52, W-What:7 and W-What-ever stay.",
			"What? That's fine, but T-What, W-W, well-known, ha-ha, X-X52, W-What:7 and W-What-ever stay.",
		},
		{
			"unicode words and multilingual text",
			"É-ÉCOUTE!!! Café, naïve, 中文 and Привет remain. “DON’T TOUCH Vol’jin’s staff!!!”",
			"Écoute! Café, naïve, 中文 and Привет remain. \"Don't touch Vol'jin's staff!\"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSpeechText(tc.input)
			if got != tc.want {
				t.Fatalf("input: %q\n  got: %q\n want: %q", tc.input, got, tc.want)
			}
			if again := normalizeSpeechText(got); again != got {
				t.Fatalf("second pass changed %q to %q", got, again)
			}
		})
	}
}

func TestNormalizeSpeechPreservesOrdinaryDialogue(t *testing.T) {
	for _, text := range []string{
		"I told King Terenas II that I would return at 3:15.",
		"Bring 1,000 coins, 3.14 ounces, and 50% of the supplies to SI:7.",
		"Vol'jin's well-known ally carries an X-52 and a T-shirt.",
		"No-no, ha-ha, I-I, W-W, and T-What aren't matching initial fragments.",
		"Wait... really?! Yes!? Well—perhaps–later. . . .",
		"\tCafé\u00a0and naïve adventurers.\r\n\n中文 Привет\t",
	} {
		if got := normalizeSpeechText(text); got != text {
			t.Errorf("ordinary dialogue changed:\n got: %q\nwant: %q", got, text)
		}
	}
}

// Exercise interactions between rules with arbitrary valid Unicode text, not
// just individual replacement examples. These properties should hold regardless
// of whether the input happens to be a recognizable word or sentence.
func FuzzNormalizeSpeechText(f *testing.F) {
	for _, seed := range []string{
		"", "T-T-THAT’S AMAZING!!! W-W-WHAT???", "SI:7 X-52 II III IV I",
		"“I’M READY…” she said. ‘DON’T GO!!??’", "É-ÉCOUTE! 中文 Привет",
		"... . . . ?!?! !!!!????", "\x00\r\n\t", "A-A'A a-a-a AAA",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip()
		}
		got := normalizeSpeechText(input)
		if !utf8.ValidString(got) {
			t.Fatalf("invalid UTF-8 output for %q: %q", input, got)
		}
		if strings.ContainsAny(got, "‘’“”…") || strings.Contains(got, "!!") || strings.Contains(got, "??") {
			t.Fatalf("punctuation was not normalized: %q -> %q", input, got)
		}
		if again := normalizeSpeechText(got); again != got {
			t.Fatalf("normalization is not idempotent: %q -> %q -> %q", input, got, again)
		}
	})
}
