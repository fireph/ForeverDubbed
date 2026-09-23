//go:build gui

package desktop

import (
	"fmt"
	"image/color"
	"strings"

	"foreverdubbed/internal/appstate"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var gold = color.NRGBA{R: 231, G: 188, B: 112, A: 255}
var green = color.NRGBA{R: 118, G: 209, B: 168, A: 255}
var soft = color.NRGBA{R: 163, G: 175, B: 197, A: 255}

type companionTheme struct{ fyne.Theme }

func (t companionTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 16, G: 22, B: 34, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 24, G: 33, B: 48, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 234, G: 239, B: 247, A: 255}
	case theme.ColorNamePrimary:
		return gold
	case theme.ColorNameDisabled:
		return soft
	}
	return t.Theme.Color(n, theme.VariantDark)
}

type statusCard struct {
	value  *canvas.Text
	detail *widget.Label
	dot    *canvas.Circle
	root   fyne.CanvasObject
}

func newStatusCard(title string) *statusCard {
	c := &statusCard{value: canvas.NewText("Starting", soft), detail: widget.NewLabel(""), dot: canvas.NewCircle(soft)}
	c.value.TextSize = 20
	c.value.TextStyle.Bold = true
	c.detail.Wrapping = fyne.TextWrapWord
	label := canvas.NewText(strings.ToUpper(title), soft)
	label.TextSize = 11
	label.TextStyle.Bold = true
	dot := container.NewCenter(container.NewGridWrap(fyne.NewSize(9, 9), c.dot))
	content := container.NewVBox(label, container.NewHBox(dot, c.value), c.detail)
	bg := canvas.NewRectangle(color.NRGBA{R: 25, G: 34, B: 49, A: 255})
	bg.CornerRadius = 12
	c.root = container.NewStack(bg, container.NewPadded(content))
	return c
}
func (c *statusCard) set(value, detail string, active bool) {
	col := color.Color(soft)
	if active {
		col = green
	}
	c.value.Text, c.value.Color, c.dot.FillColor = value, col, col
	c.value.Refresh()
	c.dot.Refresh()
	c.detail.SetText(detail)
}

type dashboard struct {
	root                                           fyne.CanvasObject
	headline                                       *canvas.Text
	hint, diagnostics, speaker, received, dialogue *widget.Label
	window, tile, audio                            *statusCard
}

func newDashboard(version string, hide, quit func()) *dashboard {
	d := &dashboard{window: newStatusCard("Game window"), tile: newStatusCard("Dialogue tile"), audio: newStatusCard("Audio")}
	brand := canvas.NewText("FOREVERDUBBED", gold)
	brand.TextSize = 14
	brand.TextStyle.Bold = true
	d.headline = canvas.NewText("Bringing Azeroth to life.", color.White)
	d.headline.TextSize = 28
	d.headline.TextStyle.Bold = true
	d.hint = widget.NewLabel("Starting your companion…")
	d.hint.Wrapping = fyne.TextWrapWord
	d.speaker = widget.NewLabelWithStyle("Waiting for dialogue", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	d.speaker.Wrapping = fyne.TextWrapWord
	d.received = widget.NewLabel("")
	d.dialogue = widget.NewLabel("Open a conversation or quest in WoW to hear it read aloud.")
	d.dialogue.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(d.dialogue)
	scroll.SetMinSize(fyne.NewSize(0, 155))
	dialogue := widget.NewCard("Latest dialogue", "", container.NewBorder(container.NewVBox(d.speaker, d.received), nil, nil, nil, scroll))
	d.diagnostics = widget.NewLabel("")
	d.diagnostics.Wrapping = fyne.TextWrapWord
	diagnostics := widget.NewAccordion(widget.NewAccordionItem("Details & troubleshooting", d.diagnostics))
	privacy := widget.NewLabelWithStyle("Only your game window is captured. Speech stays on this computer.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	privacy.Wrapping = fyne.TextWrapWord
	hideButton := widget.NewButtonWithIcon("Minimize to tray", theme.ViewRestoreIcon(), hide)
	footer := container.NewHBox(widget.NewLabel("v"+version), layout.NewSpacer(), hideButton, widget.NewButton("Quit", quit))
	header := container.NewVBox(brand, d.headline, d.hint, widget.NewSeparator(), container.NewGridWithColumns(3, d.window.root, d.tile.root, d.audio.root))
	d.root = container.New(layout.NewCustomPaddedLayout(18, 18, 18, 18), container.NewBorder(header, container.NewVBox(diagnostics, privacy, footer), nil, nil, dialogue))
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
	d.audio.set(audio, audioDetail, s.Audio == "Playing audio" || s.Audio == "Speaking (system voice)")
	if !s.Received.IsZero() {
		speaker := s.Message.Speaker
		if speaker == "" {
			speaker = "Dialogue"
		}
		d.speaker.SetText(speaker)
		d.received.SetText(s.Received.Format("15:04:05"))
		text := s.Message.Text
		if s.Message.Title != "" {
			text = s.Message.Title + "\n\n" + text
		}
		d.dialogue.SetText(text)
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
}
