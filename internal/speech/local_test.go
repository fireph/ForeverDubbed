package speech

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"foreverdubbed/internal/pocket"
	"foreverdubbed/internal/protocol"
)

type fakeEngine struct {
	generate func(context.Context, string, string, pocket.StreamOptions, func([]byte) error) error
}

func (e fakeEngine) Stream(c context.Context, t, v string, s pocket.StreamOptions, f func([]byte) error) error {
	return e.generate(c, t, v, s, f)
}
func (fakeEngine) Close() error { return nil }

func TestLocalVoiceChoices(t *testing.T) {
	c, err := Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	c.NPCOverrides["override"] = "orc_male"
	var choices map[string]string
	var selected, synthesized string
	var reads, plays int
	local := Local{
		Config: c,
		ChoiceSource: func() map[string]string {
			reads++
			return choices
		},
		OnVoice: func(_ protocol.Message, name string) { selected = name },
		Engine: fakeEngine{func(_ context.Context, _, voice string, _ pocket.StreamOptions, emit func([]byte) error) error {
			synthesized = filepath.Base(voice)
			return emit([]byte{0, 0})
		}},
		Play: func(_ context.Context, _ int, chunks <-chan []byte) error {
			plays++
			for range chunks {
			}
			return nil
		},
	}
	for _, tc := range []struct{ name, choice, override, npc, want string }{
		{"default", "", "", "", "human_female"},
		{"narrator", VoiceNarrator, "", "", "narrator_male"},
		{"none", VoiceNone, "", "", ""},
		{"none with voice override", VoiceNone, "orc_male", "", ""},
		{"none with npc override", VoiceNone, "", "override", ""},
		{"restore default", "", "", "", "human_female"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			choices = map[string]string{"human:female": tc.choice}
			local.Override = tc.override
			selected, synthesized, reads, plays = "", "", 0, 0
			if err := local.Speak(context.Background(), protocol.Message{Race: "Human", Gender: "female", NPCID: tc.npc, Text: "Hello."}); err != nil {
				t.Fatal(err)
			}
			if reads != 1 || selected != tc.want {
				t.Fatalf("choice reads=%d, selected=%q; want one read and %q", reads, selected, tc.want)
			}
			if tc.want == "" {
				if synthesized != "" || plays != 0 {
					t.Fatalf("muted voice reached synthesis/playback: %q, %d", synthesized, plays)
				}
			} else if synthesized != tc.want+".safetensors" || plays != 1 {
				t.Fatalf("synthesized=%q, plays=%d; want %q once", synthesized, plays, tc.want)
			}
		})
	}
}

func TestQuestSynthesisOmitsTitle(t *testing.T) {
	c, err := Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	local := Local{Config: c, Engine: fakeEngine{func(_ context.Context, text, _ string, _ pocket.StreamOptions, emit func([]byte) error) error {
		got += text
		return emit([]byte{0, 0})
	}}, Play: func(_ context.Context, _ int, chunks <-chan []byte) error {
		for range chunks {
		}
		return nil
	}}
	if err := local.Speak(context.Background(), protocol.Message{Kind: 2, Speaker: "Thrall", Title: "Quest title", Text: "Main dialogue.", Objectives: "Collect supplies."}); err != nil {
		t.Fatal(err)
	}
	if got != "Main dialogue." {
		t.Fatalf("synthesized %q", got)
	}
}
func TestNativeStreamingAndVoiceOptions(t *testing.T) {
	c, err := Load("../../tts/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	playing := make(chan struct{})
	profile := c.Profiles["undead_male"]
	profile.FadeInMS, profile.FadeOutMS = 50, 100
	c.Profiles["undead_male"] = profile
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	local := Local{Config: c, Engine: fakeEngine{func(ctx context.Context, text, voice string, options pocket.StreamOptions, emit func([]byte) error) error {
		if filepath.Base(voice) != "undead_male.safetensors" || options.DecodeSteps != c.Profiles["undead_male"].DecodeSteps {
			t.Errorf("voice=%s steps=%d", voice, options.DecodeSteps)
		}
		if options.FadeInMS != 50 || options.FadeOutMS != 100 {
			t.Errorf("fade options lost: %+v", options)
		}
		if err := emit([]byte{1, 0}); err != nil {
			return err
		}
		select {
		case <-playing:
		case <-ctx.Done():
			return ctx.Err()
		}
		return emit([]byte{2, 0})
	}}, Play: func(ctx context.Context, rate int, chunks <-chan []byte) error {
		n := 0
		for range chunks {
			n++
			if n == 1 {
				close(playing)
			}
		}
		if n != 2 || rate != 24000 {
			t.Errorf("chunks=%d rate=%d", n, rate)
		}
		return ctx.Err()
	}}
	if err := local.Speak(ctx, protocol.Message{Race: "Undead", Gender: "male", Text: "Welcome traveler."}); err != nil {
		t.Fatal(err)
	}
}
func TestNativeCancellationAndErrors(t *testing.T) {
	c, _ := Load("../../tts/voices.json")
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "failure"}[fail], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			boom := errors.New("native failed")
			local := Local{Config: c, Engine: fakeEngine{func(ctx context.Context, _, _ string, _ pocket.StreamOptions, emit func([]byte) error) error {
				if fail {
					return boom
				}
				if err := emit([]byte{1, 0}); err != nil {
					return err
				}
				<-ctx.Done()
				return ctx.Err()
			}}, Play: func(ctx context.Context, _ int, chunks <-chan []byte) error {
				select {
				case <-chunks:
					if !fail {
						cancel()
					}
				case <-ctx.Done():
				}
				<-ctx.Done()
				return ctx.Err()
			}}
			err := local.Speak(ctx, protocol.Message{Text: "Hello."})
			want := error(context.Canceled)
			if fail {
				want = boom
			}
			if !errors.Is(err, want) {
				t.Fatal(err)
			}
		})
	}
}
