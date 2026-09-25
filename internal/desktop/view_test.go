//go:build gui

package desktop

import (
	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/protocol"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestDashboardLiveStates(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(companionTheme{theme.DefaultTheme()})
	stops := 0
	queued := false
	var filters appstate.SpeechFilters
	d := newDashboard("0.5.0", func() {}, func() {}, func() { stops++ }, func(enabled bool) { queued = enabled }, func(f appstate.SpeechFilters) { filters = f })
	w := a.NewWindow("ForeverDubbed")
	defer w.Close()
	w.SetContent(d.root)
	w.Resize(fyne.NewSize(760, 640))
	state := appstate.New("World of Warcraft Beta.app", "pocket", false)
	d.render(state.Snapshot())
	if d.skipControl.Visible() || !d.skip.Disabled() {
		t.Fatal("skip available without queue mode")
	}
	if d.headline.Text != "Starting your companion" {
		t.Fatal(d.headline.Text)
	}
	if !d.quests.Checked || !d.conversations.Checked || !d.npcSpeech.Checked {
		t.Fatal("filters should default on")
	}
	if d.questObjectives.Checked || d.questTitle.Checked {
		t.Fatal("quest extras should default off")
	}
	test.Tap(d.questObjectives)
	if !filters.QuestObjectives || filters.QuestTitle || !filters.Quests || !filters.Conversations || !filters.NPCSpeech {
		t.Fatal("objective checkbox changed wrong filters", filters)
	}
	test.Tap(d.questObjectives)
	test.Tap(d.questTitle)
	if !filters.QuestTitle || filters.QuestObjectives || !filters.Quests || !filters.Conversations || !filters.NPCSpeech {
		t.Fatal("title checkbox changed wrong filters", filters)
	}
	test.Tap(d.questTitle)
	test.Tap(d.quests)
	if filters.Quests || !filters.Conversations || !filters.NPCSpeech {
		t.Fatal("quest checkbox changed wrong filters", filters)
	}
	test.Tap(d.conversations)
	test.Tap(d.npcSpeech)
	if filters != (appstate.SpeechFilters{}) {
		t.Fatal("filters not independently selectable", filters)
	}
	d.render(state.Snapshot())
	state.Update(func(v *appstate.Snapshot) { v.Ready = true })
	state.Capture(true, false, nil)
	d.render(state.Snapshot())
	if d.window.value.Text != "Detected" || d.tile.value.Text != "Searching" {
		t.Fatal("window and tile states conflated")
	}
	test.Tap(d.queue)
	if !queued {
		t.Fatal("checkbox did not enable queue mode")
	}
	test.Tap(d.queue)
	if queued {
		t.Fatal("checkbox did not disable queue mode")
	}
	state.SetQueueSpeech(true)
	d.render(state.Snapshot())
	if !d.queue.Checked {
		t.Fatal("saved queue setting not reflected")
	}
	state.Capture(true, true, nil)
	state.Update(func(v *appstate.Snapshot) {
		v.Voice = "orc_male"
		v.PlaybackID = 1
		v.PlayingSpeaker = "Thrall"
		v.Played, v.Duration, v.DurationKnown = 7*time.Second, 20*time.Second, true
		v.Received = time.Date(2026, 9, 23, 14, 32, 8, 0, time.UTC)
		v.Message = protocol.Message{Speaker: "Thrall", Title: "A call to adventure", Text: "Welcome, traveler. There is much to be done, and Azeroth needs your strength. Take a moment to prepare yourself, then meet me at the gates."}
	})
	state.Audio("Playing audio")
	d.render(state.Snapshot())
	if d.audio.value.Text != "Playing audio" {
		t.Fatal("missing playback/dialogue status")
	}
	if d.stop.Disabled() {
		t.Fatal("stop disabled during playback")
	}
	test.Tap(d.stop)
	if stops != 1 {
		t.Fatal("stop callback missing")
	}
	if !d.skipControl.Visible() || d.skip.Disabled() {
		t.Fatal("skip unavailable during queued playback")
	}
	test.Tap(d.skip)
	if stops != 2 {
		t.Fatal("skip did not interrupt current speech")
	}
	state.SetQueueSpeech(false)
	d.render(state.Snapshot())
	if d.skipControl.Visible() || !d.skip.Disabled() {
		t.Fatal("skip available after queue disabled")
	}
	state.SetQueueSpeech(true)
	d.render(state.Snapshot())
	if os.Getenv("FDB_UI_SCREENSHOT_IDLE") != "" {
		state.ResetPlayback()
		d.render(state.Snapshot())
	}
	if name := os.Getenv("FDB_UI_SCREENSHOT"); name != "" {
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	for _, phase := range []string{"Preparing speech", "Idle", "Muted", "Starting", "Stopped"} {
		state.Audio(phase)
		d.render(state.Snapshot())
		if !d.stop.Disabled() || !d.skip.Disabled() {
			t.Fatalf("stop enabled during %s", phase)
		}
		test.Tap(d.stop)
		test.Tap(d.skip)
	}
	if stops != 2 {
		t.Fatal("disabled stop invoked callback")
	}
	state.Update(func(v *appstate.Snapshot) { v.PlaybackID = 2 })
	state.Audio("Speaking (system voice)")
	d.render(state.Snapshot())
	if d.stop.Disabled() {
		t.Fatal("stop disabled for system voice")
	}
	state.ResetPlayback()
	d.render(state.Snapshot())
	if !d.stop.Disabled() {
		t.Fatal("idle stop enabled")
	}
	state.Finished(nil)
	d.render(state.Snapshot())
	if d.window.value.Text != "Stopped" || d.audio.value.Text != "Stopped" {
		t.Fatal("stale running status after stop")
	}
}
