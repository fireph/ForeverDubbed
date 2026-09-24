//go:build gui

package desktop

import (
	"fmt"

	"foreverdubbed/internal/appstate"
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
	queue               *widget.Check
	root                fyne.CanvasObject
	headline            *canvas.Text
	hint                *widget.Label
	window, tile, audio *statusCard
	stop                *widget.Button
	skip                *widget.Button
	skipControl         fyne.CanvasObject
}

func newDashboard(version string, hide, quit, stop func(), queue func(bool)) *dashboard {
	d := &dashboard{window: newStatusCard("Game window"), tile: newStatusCard("Dialogue tile"), audio: newStatusCard("Audio")}
	title := canvas.NewText("ForeverDubbed", gold)
	title.TextSize = 27
	title.TextStyle.Bold = true
	subtitle := canvas.NewText("World of Warcraft companion", gold)
	subtitle.TextSize = 14
	banner := container.NewBorder(nil, nil, questMedallion(), nil, container.NewCenter(container.NewVBox(title, subtitle)))
	d.headline = canvas.NewText("Bringing Azeroth to life.", ink)
	d.headline.TextSize = 23
	d.headline.TextStyle.Bold = true
	d.hint = widget.NewLabel("Starting your companion…")
	d.hint.Wrapping = fyne.TextWrapWord
	d.stop = widget.NewButton("Stop", stop)
	d.stop.Disable()
	// The speech worker already advances the queue after cancelling the current
	// utterance, and becomes idle when there is no next message.
	d.skip = widget.NewButton("Skip", stop)
	d.skip.Disable()
	d.skipControl = questButtonWidget(d.skip)
	d.skipControl.Hide()
	d.queue = widget.NewCheck("Queue new dialogue", queue)
	audioCard := container.NewVBox(d.audio.root, inset(3, container.NewHBox(questButtonWidget(d.stop), d.skipControl)))
	header := container.NewVBox(d.headline, d.hint, questRule(), container.NewGridWithColumns(3, d.window.root, d.tile.root, audioCard), d.queue)
	paper := parchment(container.NewVScroll(header))
	privacy := canvas.NewText("Only your game window is captured. Speech stays on this computer.", gold)
	privacy.TextSize = 12
	privacy.TextStyle.Italic = true
	versionLabel := canvas.NewText("v"+version, gold)
	versionLabel.TextSize = 13
	privacyNote := container.New(layout.NewCustomPaddedLayout(0, 10, 0, 0), container.NewCenter(privacy))
	footer := container.NewVBox(privacyNote, container.NewHBox(container.NewCenter(versionLabel), layout.NewSpacer(), questButton("Minimize to tray", hide), questButton("Quit", quit)))
	d.root = questFrame(container.NewBorder(inset(5, banner), inset(5, footer), nil, nil, paper))
	return d
}
func (d *dashboard) render(s appstate.Snapshot) {
	// Reflect the saved preference without firing the user's change callback.
	if d.queue.Checked != s.QueueSpeech {
		d.queue.Checked = s.QueueSpeech
		d.queue.Refresh()
	}
	headline, hint := "Waiting for your adventure", "Open WoW and keep its game window available."
	win, winDetail := "Waiting", "No game frames available"
	tile, tileDetail := "Searching", "In WoW, use /fdb unlock"
	if s.Window {
		win, winDetail = "Detected", "Receiving game-window frames"
		headline, hint = "Your game is connected", "Use /fdb test in WoW to check the addon connection."
	}
	if s.Tile {
		tile, tileDetail = "Connected", "Reading dialogue from the addon"
		headline, hint = "Ready for the next story", "Talk to an NPC or open a quest. New dialogue interrupts the previous speech."
	}
	if s.Tile && s.QueueSpeech {
		hint = "Talk to an NPC or open a quest. New dialogue waits for the current speech to finish."
	}
	if !s.Ready {
		headline, hint = "Starting your companion", "Loading speech and preparing game-window capture…"
	}
	if s.Audio == "Preparing speech" {
		headline = "Preparing the next voice"
	}
	if s.Audio == "Playing audio" || s.Audio == "Speaking (system voice)" {
		headline = "A voice for every story"
	}
	if s.Stopped {
		headline, hint = "Companion stopped", "Quit and reopen the app to start again."
		win, tile = "Stopped", "Stopped"
	}
	if s.FatalError != "" {
		headline, hint = "Unable to start", s.FatalError
	}
	if s.SpeechError != "" {
		hint = "Speech failed: " + s.SpeechError
	}
	d.headline.Text = headline
	d.headline.Refresh()
	d.hint.SetText(hint)
	d.window.set(win, winDetail, s.Window)
	d.tile.set(tile, tileDetail, s.Tile)
	audio := s.Audio
	if audio == "Speaking (system voice)" {
		audio = "Speaking"
	}
	playing := !s.Stopped && (s.Audio == "Playing audio" || s.Audio == "Speaking (system voice)")
	audioDetail := ""
	if playing && s.Voice != "" {
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
