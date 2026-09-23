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
	root                fyne.CanvasObject
	headline            *canvas.Text
	hint, diagnostics   *widget.Label
	window, tile, audio *statusCard
	stop                *widget.Button
}

func newDashboard(version string, hide, quit, stop func()) *dashboard {
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
	audioCard := container.NewVBox(d.audio.root, inset(3, container.NewHBox(questButtonWidget(d.stop))))
	d.diagnostics = widget.NewLabel("")
	d.diagnostics.Wrapping = fyne.TextWrapWord
	detailScroll := container.NewVScroll(d.diagnostics)
	detailScroll.SetMinSize(fyne.NewSize(0, 110))
	diagnostics := widget.NewAccordion(widget.NewAccordionItem("Details & troubleshooting", detailScroll))
	header := container.NewVBox(d.headline, d.hint, questRule(), container.NewGridWithColumns(3, d.window.root, d.tile.root, audioCard), questRule())
	paper := parchment(container.NewBorder(header, nil, nil, nil, container.NewVScroll(container.NewVBox(diagnostics))))
	privacy := canvas.NewText("Only your game window is captured. Speech stays on this computer.", gold)
	privacy.TextSize = 12
	privacy.TextStyle.Italic = true
	versionLabel := canvas.NewText("v"+version, gold)
	versionLabel.TextSize = 13
	footer := container.NewVBox(container.NewCenter(privacy), container.NewHBox(container.NewCenter(versionLabel), layout.NewSpacer(), questButton("Minimize to tray", hide), questButton("Quit", quit)))
	d.root = questFrame(container.NewBorder(inset(5, banner), inset(5, footer), nil, nil, paper))
	return d
}
func (d *dashboard) render(s appstate.Snapshot) {
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
		hint = "Speech failed. See Details & troubleshooting below."
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
	audioDetail := "Pocket TTS · on your CPU"
	if s.Backend == "system" {
		audioDetail = "System voice"
	}
	if s.Voice != "" {
		audioDetail = s.Voice
	}
	playing := !s.Stopped && (s.Audio == "Playing audio" || s.Audio == "Speaking (system voice)")
	d.audio.set(audio, audioDetail, playing)
	if playing && s.PlaybackID != 0 {
		d.stop.Enable()
	} else {
		d.stop.Disable()
	}
	details := fmt.Sprintf("Capture target: %s\nSpeech engine: %s", s.Target, s.Backend)
	if s.CaptureError != "" {
		details += "\n\nCapture: " + s.CaptureError
	}
	if s.SpeechError != "" {
		details += "\n\nSpeech: " + s.SpeechError
	}
	if s.FatalError != "" {
		details += "\n\nStartup: " + s.FatalError
	}
	d.diagnostics.SetText(details)
	// Wrapped status text can change child minimum sizes.
	d.root.Refresh()
}
