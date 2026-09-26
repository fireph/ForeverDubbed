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
	var choices map[string]string
	d := newDashboard("0.5.0", func() {}, func() {}, func() { stops++ }, func(enabled bool) { queued = enabled }, func(f appstate.SpeechFilters) { filters = f },
		[]string{"human", "nightelf", "orc"}, map[string]string{"nightelf:female": "narrator"}, func(c map[string]string) { choices = c })
	w := a.NewWindow("ForeverDubbed")
	defer w.Close()
	w.SetContent(d.root)
	w.Resize(fyne.NewSize(760, 740))
	state := appstate.New("World of Warcraft Beta.app", "pocket", false)
	d.render(state.Snapshot())
	if d.skipControl.Visible() || !d.skip.Disabled() {
		t.Fatal("skip available without queue mode")
	}
	if !d.quests.Checked || !d.conversations.Checked || !d.npcSpeech.Checked {
		t.Fatal("filters should default on")
	}
	if d.questObjectives.Checked || d.questTitle.Checked {
		t.Fatal("quest extras should default off")
	}
	if !d.questOptions.Visible() {
		t.Fatal("quest options hidden while quest dialogue is enabled")
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
	test.Tap(d.quests)
	if d.questOptions.Visible() || !d.questTitle.Checked || !filters.QuestTitle {
		t.Fatal("disabling quests must hide options and retain their selections")
	}
	test.Tap(d.quests)
	if !d.questOptions.Visible() || !d.questTitle.Checked {
		t.Fatal("enabling quests must restore selected options")
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
	state.SetSpeechFilters(filters)
	d.render(state.Snapshot())
	if d.questOptions.Visible() {
		t.Fatal("render ignored saved quest dialogue setting")
	}
	state.SetSpeechFilters(appstate.SpeechFilters{Quests: true, Conversations: true, NPCSpeech: true})
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
	// Voices tab: saved selections render, and edits report compact maps.
	if len(d.voiceSelects) != 6 {
		t.Fatal("missing race/gender dropdowns", len(d.voiceSelects))
	}
	if d.tabSettings.Disabled() == d.tabVoices.Disabled() {
		t.Fatal("exactly one tab button must start disabled")
	}
	test.Tap(d.tabVoices)
	if !d.tabVoices.Disabled() || d.tabSettings.Disabled() {
		t.Fatal("Voices tab did not activate")
	}
	test.Tap(d.tabSettings)
	if !d.tabSettings.Disabled() || d.tabVoices.Disabled() {
		t.Fatal("Settings tab did not activate")
	}
	for key, want := range map[string]string{
		"human:male": "Default", "human:female": "Default",
		"orc:male": "Default", "orc:female": "Default",
		"nightelf:male": "Default", "nightelf:female": "Narrator",
	} {
		if d.voiceSelects[key].Selected != want {
			t.Fatalf("%s shows %q, want %q", key, d.voiceSelects[key].Selected, want)
		}
	}
	if choices != nil {
		t.Fatal("building the tab must not replay saved choices", choices)
	}
	d.voiceSelects["orc:male"].SetSelected("None")
	if choices["orc:male"] != "none" || choices["nightelf:female"] != "narrator" {
		t.Fatal("dropdown did not report selection", choices)
	}
	d.voiceSelects["nightelf:female"].SetSelected("Default")
	if _, ok := choices["nightelf:female"]; ok || len(choices) != 1 {
		t.Fatal("default selection must drop the stored entry", choices)
	}
	d.voiceSelects["orc:female"].SetSelected("Narrator")
	if choices["orc:female"] != "narrator" || choices["orc:male"] != "none" {
		t.Fatal("selections are not per race/gender", choices)
	}
}

func TestDashboardSavedVoicesWithoutConfig(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	var choices map[string]string
	d := newDashboard("test", func() {}, func() {}, func() {}, func(bool) {}, func(appstate.SpeechFilters) {},
		nil, map[string]string{"orc:male": "none", "human:female": "obsolete"}, func(c map[string]string) { choices = c })
	d.render(appstate.New("game", "system", false).Snapshot())
	if d.voiceSelects["orc:male"].Selected != "None" || d.voiceSelects["human:female"].Selected != "Default" {
		t.Fatal("saved choices are not editable without race mappings")
	}
	d.voiceSelects["orc:male"].SetSelected("Default")
	if len(choices) != 0 {
		t.Fatal("could not clear saved mute choice", choices)
	}
}
