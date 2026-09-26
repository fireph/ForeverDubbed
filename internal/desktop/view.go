//go:build gui

package desktop

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"unicode"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/speech"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type statusCard struct {
	value  *canvas.Text
	detail *widget.Label
	dot    *canvas.Circle
	root   fyne.CanvasObject
}

func newStatusCard(title string) *statusCard {
	c := &statusCard{value: canvas.NewText("Starting", soft), detail: widget.NewLabel(""), dot: canvas.NewCircle(soft)}
	c.value.TextSize = 19
	c.value.TextStyle.Bold = true
	c.detail.Wrapping = fyne.TextWrapWord
	label := canvas.NewText(title, soft)
	label.TextSize = 15
	label.TextStyle.Bold = true
	dot := container.NewCenter(container.NewGridWrap(fyne.NewSize(9, 9), c.dot))
	content := container.NewVBox(label, container.NewHBox(dot, c.value), c.detail)
	c.root = inset(3, content)
	return c
}
func (c *statusCard) set(value, detail string, active bool) {
	col := soft
	if active {
		col = green
	}
	c.value.Text, c.value.Color, c.dot.FillColor = value, col, col
	c.value.Refresh()
	c.dot.Refresh()
	c.detail.SetText(detail)
}

type dashboard struct {
	queue                            *widget.Check
	quests, conversations, npcSpeech *widget.Check
	questObjectives                  *widget.Check
	questTitle                       *widget.Check
	questOptions                     *fyne.Container
	root                             fyne.CanvasObject
	window, tile, audio              *statusCard
	stop                             *widget.Button
	skip                             *widget.Button
	skipControl                      fyne.CanvasObject
	tabSettings, tabVoices           *widget.Button
	voiceSelects                     map[string]*widget.Select
	voiceChoices                     map[string]string
	onVoiceChoices                   func(map[string]string)
}

func newDashboard(version string, hide, quit, stop func(), queue func(bool), filters func(appstate.SpeechFilters), races []string, voiceChoices map[string]string, onVoiceChoices func(map[string]string)) *dashboard {
	d := &dashboard{window: newStatusCard("Game window"), tile: newStatusCard("Dialogue tile"), audio: newStatusCard("Audio"), voiceSelects: map[string]*widget.Select{}, voiceChoices: maps.Clone(voiceChoices)}
	if d.voiceChoices == nil {
		d.voiceChoices = map[string]string{}
	}
	title := canvas.NewText("ForeverDubbed", gold)
	title.TextSize = 27
	title.TextStyle.Bold = true
	subtitle := canvas.NewText("World of Warcraft companion", gold)
	subtitle.TextSize = 14
	banner := container.NewBorder(nil, nil, questMedallion(), nil, container.NewCenter(container.NewVBox(title, subtitle)))
	d.stop = widget.NewButton("Stop", stop)
	d.stop.Disable()
	// The speech worker already advances the queue after cancelling the current
	// utterance, and becomes idle when there is no next message.
	d.skip = widget.NewButton("Skip", stop)
	d.skip.Disable()
	d.skipControl = questButtonWidget(d.skip)
	d.skipControl.Hide()
	d.queue = widget.NewCheck("Queue new dialogue", queue)
	updateFilters := func(bool) {
		d.showQuestOptions()
		filters(appstate.SpeechFilters{Quests: d.quests.Checked, Conversations: d.conversations.Checked, NPCSpeech: d.npcSpeech.Checked, QuestObjectives: d.questObjectives.Checked, QuestTitle: d.questTitle.Checked})
	}
	d.quests = widget.NewCheck("Quest dialogue", updateFilters)
	d.conversations = widget.NewCheck("NPC conversations", updateFilters)
	d.npcSpeech = widget.NewCheck("NPC speech", updateFilters)
	d.questObjectives = widget.NewCheck("Quest objectives", updateFilters)
	d.questTitle = widget.NewCheck("Quest title", updateFilters)
	d.questOptions = container.New(layout.NewCustomPaddedLayout(0, 0, 18, 0),
		container.NewVBox(d.questTitle, d.questObjectives))
	d.showQuestOptions()
	categories := container.NewVBox(
		widget.NewLabelWithStyle("Read aloud", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(3,
			container.NewVBox(d.quests, d.questOptions),
			container.NewVBox(d.conversations),
			container.NewVBox(d.npcSpeech)))

	audioCard := container.NewVBox(d.audio.root, inset(3, container.NewHBox(questButtonWidget(d.stop), d.skipControl)))
	header := container.NewVBox(container.NewGridWithColumns(3, d.window.root, d.tile.root, audioCard), questRule(), categories, d.queue)
	paper := parchment(container.NewVScroll(header))
	voices := parchment(container.NewVScroll(d.voicePanel(races)))
	voices.Hide()
	d.tabSettings = widget.NewButton("Settings", func() {
		paper.Show()
		voices.Hide()
		d.tabSettings.Disable()
		d.tabVoices.Enable()
	})
	d.tabVoices = widget.NewButton("Voices", func() {
		voices.Show()
		paper.Hide()
		d.tabVoices.Disable()
		d.tabSettings.Enable()
	})
	d.tabSettings.Disable()
	tabBar := container.NewGridWithColumns(2, questButtonWidget(d.tabSettings), questButtonWidget(d.tabVoices))
	panels := container.NewStack(paper, voices)
	privacy := canvas.NewText("Only your game window is captured. Speech stays on this computer.", gold)
	privacy.TextSize = 12
	privacy.TextStyle.Italic = true
	versionLabel := canvas.NewText("v"+version, gold)
	versionLabel.TextSize = 13
	privacyNote := container.New(layout.NewCustomPaddedLayout(0, 10, 0, 0), container.NewCenter(privacy))
	footer := container.NewVBox(privacyNote, container.NewHBox(container.NewCenter(versionLabel), layout.NewSpacer(), questButton("Minimize to tray", hide), questButton("Quit", quit)))
	d.root = questFrame(container.NewBorder(container.NewVBox(inset(5, banner), inset(2, tabBar)), inset(5, footer), nil, nil, panels))
	// Set only after the selects above restored their saved selections, so
	// building the tab never persists or replays user choices.
	d.onVoiceChoices = onVoiceChoices
	return d
}

// voicePanel lists one dropdown pair (male/female) per race. "Default" keeps
// the voices.json mapping, "Narrator" uses the narrator voice, "None" stays
// silent.
func (d *dashboard) voicePanel(races []string) fyne.CanvasObject {
	// Keep saved selections editable even if a race was removed from the
	// configuration or the system backend is running without that file.
	available := make(map[string]bool, len(races))
	for _, race := range races {
		available[race] = true
	}
	for key := range d.voiceChoices {
		race, gender, ok := strings.Cut(key, ":")
		if ok && race != "" && (gender == "male" || gender == "female") {
			available[race] = true
		}
	}
	races = make([]string, 0, len(available))
	for race := range available {
		races = append(races, race)
	}
	sort.Strings(races)
	if len(races) == 0 {
		note := widget.NewLabel("No races are available. Add race mappings to the voice configuration and restart the companion.")
		note.Wrapping = fyne.TextWrapWord
		return note
	}
	options := []string{"Default", "Narrator", "None"}
	display := map[string]string{"": "Default", speech.VoiceNarrator: "Narrator", speech.VoiceNone: "None"}
	bold := fyne.TextStyle{Bold: true}
	rows := make([]fyne.CanvasObject, 0, len(races)+1)
	rows = append(rows, container.NewGridWithColumns(3,
		widget.NewLabelWithStyle("Race", fyne.TextAlignLeading, bold),
		widget.NewLabelWithStyle("Male", fyne.TextAlignLeading, bold),
		widget.NewLabelWithStyle("Female", fyne.TextAlignLeading, bold)))
	for _, race := range races {
		cells := make([]fyne.CanvasObject, 0, 3)
		cells = append(cells, widget.NewLabel(raceLabel(race)))
		for _, gender := range []string{"male", "female"} {
			key := race + ":" + gender
			sel := widget.NewSelect(options, func(value string) { d.setVoiceChoice(key, value) })
			label, ok := display[d.voiceChoices[key]]
			if !ok {
				label = "Default"
			}
			sel.SetSelected(label)
			d.voiceSelects[key] = sel
			cells = append(cells, sel)
		}
		rows = append(rows, container.NewGridWithColumns(3, cells...))
	}
	return container.NewVBox(rows...)
}

func (d *dashboard) setVoiceChoice(key, label string) {
	value := map[string]string{"Narrator": speech.VoiceNarrator, "None": speech.VoiceNone}[label]
	if value == "" {
		delete(d.voiceChoices, key)
	} else {
		d.voiceChoices[key] = value
	}
	if d.onVoiceChoices != nil {
		d.onVoiceChoices(maps.Clone(d.voiceChoices))
	}
}

var raceLabels = map[string]string{
	"human": "Human", "orc": "Orc", "dwarf": "Dwarf", "nightelf": "Night Elf",
	"undead": "Undead", "tauren": "Tauren", "gnome": "Gnome", "troll": "Troll",
	"goblin": "Goblin", "skyborne": "Skyborne",
}

func raceLabel(id string) string {
	if label, ok := raceLabels[id]; ok {
		return label
	}
	runes := []rune(id)
	if len(runes) == 0 {
		return id
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func (d *dashboard) showQuestOptions() {
	if d.quests.Checked {
		d.questOptions.Show()
	} else {
		d.questOptions.Hide()
	}
}

func (d *dashboard) render(s appstate.Snapshot) {
	// Reflect saved preferences without firing the user's change callbacks.
	for check, enabled := range map[*widget.Check]bool{
		d.quests: s.Filters.Quests, d.conversations: s.Filters.Conversations, d.npcSpeech: s.Filters.NPCSpeech,
		d.questObjectives: s.Filters.QuestObjectives,
		d.questTitle:      s.Filters.QuestTitle,
	} {
		if check.Checked != enabled {
			check.Checked = enabled
			check.Refresh()
		}
	}
	d.showQuestOptions()
	if d.queue.Checked != s.QueueSpeech {
		d.queue.Checked = s.QueueSpeech
		d.queue.Refresh()
	}
	win, winDetail := "Waiting", "No game frames available"
	tile, tileDetail := "Searching", "In WoW, use /fdb unlock"
	if s.Window {
		win, winDetail = "Detected", "Receiving game-window frames"
	}
	if s.Tile {
		tile, tileDetail = "Connected", "Reading dialogue from the addon"
	}
	if s.Stopped {
		win, tile = "Stopped", "Stopped"
	}
	d.window.set(win, winDetail, s.Window)
	d.tile.set(tile, tileDetail, s.Tile)
	audio := s.Audio
	if audio == "Speaking (system voice)" {
		audio = "Speaking"
	}
	playing := !s.Stopped && (s.Audio == "Playing audio" || s.Audio == "Speaking (system voice)")
	audioDetail := ""
	if s.FatalError != "" {
		audioDetail = "Unable to start: " + s.FatalError
	} else if s.SpeechError != "" {
		audioDetail = "Speech failed: " + s.SpeechError
	} else if playing && s.Voice != "" {
		audioDetail = s.Voice
		if s.Queued > 0 {
			audioDetail += fmt.Sprintf(" · %d queued", s.Queued)
		}
	}
	d.audio.set(audio, audioDetail, playing)
	if playing && s.PlaybackID != 0 {
		d.stop.Enable()
	} else {
		d.stop.Disable()
	}
	if s.QueueSpeech {
		d.skipControl.Show()
	} else {
		d.skipControl.Hide()
	}
	if s.QueueSpeech && playing && s.PlaybackID != 0 {
		d.skip.Enable()
	} else {
		d.skip.Disable()
	}
	// Wrapped status text can change child minimum sizes.
	d.root.Refresh()
}
