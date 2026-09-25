//go:build gui

package desktop

import (
	"foreverdubbed/internal/update"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestUpdatePromptRequiresConfirmation(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("update test")
	defer w.Close()
	w.Resize(fyne.NewSize(760, 740))
	accepted := 0
	prompt := updatePrompt(w, "v1.2.3", "1.2.2", func() { accepted++ })
	prompt.Show()
	content := prompt.Content.(*fyne.Container)
	buttons := content.Objects[2].(*fyne.Container)
	test.Tap(buttons.Objects[0].(*widget.Button))
	if accepted != 0 || prompt.Visible() {
		t.Fatal("Later must dismiss without downloading")
	}
	prompt.Show()
	test.Tap(buttons.Objects[1].(*widget.Button))
	if accepted != 1 || prompt.Visible() {
		t.Fatal("OK must start exactly one update")
	}
}
func TestUpdateProgressStages(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	v := newUpdateProgress()
	v.render(update.Progress{Stage: "Downloading update", Done: 1 << 20, Total: 2 << 20})
	if v.bar.Value != 0.5 || v.label.Text != "Downloading update: 1.0 / 2.0 MB" {
		t.Fatal(v.bar.Value, v.label.Text)
	}
	v.render(update.Progress{Stage: "Installing update", Done: 3, Total: 3})
	if v.bar.Value != 1 || v.label.Text != "Installing update" {
		t.Fatal(v.bar.Value, v.label.Text)
	}
}
