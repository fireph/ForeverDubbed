//go:build gui

package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"foreverdubbed/internal/appstate"
	"fyne.io/fyne/v2/test"
)

func gameFixture(t *testing.T, platform string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Data"), 0755); err != nil {
		t.Fatal(err)
	}
	if platform == "darwin" {
		client := filepath.Join(root, "Renamed WoW.app")
		if err := os.Mkdir(client, 0755); err != nil {
			t.Fatal(err)
		}
		return client
	}
	client := filepath.Join(root, "RenamedWoW.exe")
	if err := os.WriteFile(client, nil, 0600); err != nil {
		t.Fatal(err)
	}
	return client
}

func TestGameDiscovery(t *testing.T) {
	for _, platform := range []string{"windows", "darwin"} {
		for _, scenario := range []string{"saved", "first launch", "missing saved", "CLI override", "nothing found"} {
			t.Run(platform+"/"+scenario, func(t *testing.T) {
				a := test.NewApp()
				defer a.Quit()
				p := a.Preferences()
				saved, candidate := gameFixture(t, platform), gameFixture(t, platform)
				missing := filepath.Join(t.TempDir(), "missing.exe")
				state := appstate.New("default", "pocket", false)
				controller := gameSelection{p, state, platform}
				want := candidate
				candidates := []string{missing, candidate}
				switch scenario {
				case "saved":
					p.SetString("wowExecutable", saved)
					want = saved
				case "missing saved":
					p.SetString("wowExecutable", missing)
				case "CLI override":
					state.Update(func(s *appstate.Snapshot) { s.Target = "explicit"; s.CaptureTargetExplicit = true })
				case "nothing found":
					p.SetString("wowExecutable", missing)
					candidates, want = []string{missing}, ""
				}
				before := state.Snapshot()
				if got := controller.discover(candidates); got != want {
					t.Fatalf("discovered %q, want %q", got, want)
				}
				s := state.Snapshot()
				if want == "" {
					if s != before || p.String("wowExecutable") != missing {
						t.Fatal("failed discovery changed selection")
					}
					return
				}
				if s.WoWPath != want || p.String("wowExecutable") != want {
					t.Fatal("discovered installation was not displayed and saved")
				}
				target := want
				if scenario == "CLI override" {
					target = "explicit"
				}
				if s.Target != target {
					t.Fatalf("capture target %q, want %q", s.Target, target)
				}
				if scenario != "CLI override" {
					select {
					case <-state.CaptureChanges():
					default:
						t.Fatal("discovery did not wake capture")
					}
				}
			})
		}
	}
}

func TestManualGameSelection(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	p := a.Preferences()
	state := appstate.New("explicit.exe", "pocket", false)
	state.Update(func(s *appstate.Snapshot) { s.CaptureTargetExplicit = true })
	controller := gameSelection{p, state, "windows"}
	client := gameFixture(t, "windows")
	if dir, err := controller.selectPath(client, true); err != nil || dir != filepath.Dir(client) {
		t.Fatalf("selection: %q, %v", dir, err)
	}
	if s := state.Snapshot(); s.Target != client || s.WoWPath != client || p.String("wowExecutable") != client {
		t.Fatal("manual selection did not replace CLI target and persist")
	}
	// Re-selecting the same game should not discard capture's detected path.
	state.Update(func(s *appstate.Snapshot) { s.DetectedExecutable = client })
	before := state.Snapshot()
	if _, err := controller.selectPath(client, true); err != nil {
		t.Fatal(err)
	}
	if state.Snapshot() != before {
		t.Fatal("reselecting the same client reset capture")
	}
	// A rejected choice must not replace the working capture target or preferences.
	if _, err := controller.selectPath(filepath.Join(t.TempDir(), "missing.exe"), true); err == nil {
		t.Fatal("accepted missing client")
	}
	if state.Snapshot() != before || p.String("wowExecutable") != client {
		t.Fatal("invalid selection replaced the working installation")
	}
	// Saved manual selections use the same flow on the next launch.
	restarted := appstate.New("default", "pocket", false)
	next := gameSelection{p, restarted, "windows"}
	if next.discover(nil) != client || restarted.Snapshot().Target != client {
		t.Fatal("manual selection was not restored on restart")
	}
}
