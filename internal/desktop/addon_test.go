//go:build gui

package desktop

import (
	"foreverdubbed/addon"
	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/protocol"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddonPermissionHelp(t *testing.T) {
	for _, platform := range []string{"windows", "darwin"} {
		root := filepath.Join("game", "_classic_beta_")
		got := addonPermissionHelp(root, platform)
		if !strings.Contains(got, "manually") || !strings.Contains(got, filepath.Join(root, "Interface", "AddOns")) {
			t.Fatalf("missing manual destination: %s", got)
		}
		if strings.Contains(got, "administrator") != (platform == "windows") {
			t.Fatalf("wrong platform guidance: %s", got)
		}
	}
}

func TestPendingAddonNoticeUsesLoadedAddon(t *testing.T) {
	for _, tc := range []struct {
		name, pending, loaded, want string
		tile                        bool
		session                     uint32
	}{
		{name: "fresh install before WoW starts", pending: addon.Restart, want: addon.Restart},
		{name: "saved restart with old addon loaded", pending: addon.Restart, loaded: "v0.8.2", tile: true, session: 1, want: addon.Reload},
		{name: "saved restart with legacy addon loaded", pending: addon.Restart, tile: true, session: 1, want: addon.Reload},
		{name: "current addon clears saved restart", pending: addon.Restart, loaded: "v0.8.3", tile: true, session: 2},
		{name: "update stays reload", pending: addon.Reload, loaded: "v0.8.2", tile: true, session: 1, want: addon.Reload},
		{name: "reload confirms update", pending: addon.Reload, loaded: "v0.8.3", tile: true, session: 2},
		{name: "current version clears without another reload", pending: addon.Reload, loaded: "v0.8.3", tile: true, session: 1},
		{name: "lost square cannot confirm", pending: addon.Restart, loaded: "v0.8.3", session: 2, want: addon.Restart},
		{name: "malformed newer version", pending: addon.Reload, loaded: "2.bad.bad", tile: true, session: 1, want: addon.Reload},
		{name: "duplicate version prefix", pending: addon.Reload, loaded: "vv0.8.3", tile: true, session: 1, want: addon.Reload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := appstate.Snapshot{Tile: tc.tile, AddonVersion: tc.loaded, AddonSession: tc.session}
			if got := reconcileAddonNotice(tc.pending, s, "0.8.3"); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCurrentInstalledAddonNeedsNoReloadWithoutWoW(t *testing.T) {
	root := t.TempDir()
	if prompt, err := addon.Install(root, "0.8.3"); err != nil || prompt != addon.Restart {
		t.Fatalf("fresh install: %q, %v", prompt, err)
	}
	// On the next companion launch the addon already exists and WoW is closed.
	prompt, err := addon.Install(root, "0.8.3")
	if err != nil {
		t.Fatal(err)
	}
	if notice := reconcileAddonNotice(prompt, appstate.Snapshot{}, "0.8.3"); notice != "" {
		t.Fatalf("current installed files requested an unnecessary reload: %s", notice)
	}
	// An actual update still requires /reload even without a captured square.
	prompt, err = addon.Install(root, "0.8.4")
	if err != nil || prompt != addon.Reload {
		t.Fatalf("update: %q, %v", prompt, err)
	}
}

func TestSquareVersionClearsReloadInSameSession(t *testing.T) {
	state := appstate.New("WoW", "pocket", false)
	var assembler protocol.Assembler
	notice := addon.Reload
	for i, version := range []string{"v0.8.2", "v0.8.3"} {
		frames, err := protocol.Encode(protocol.Message{Kind: protocol.KindVersion, Session: 42, Sequence: uint32(i + 1), AddonVersion: version})
		if err != nil {
			t.Fatal(err)
		}
		for _, frame := range frames {
			square, err := protocol.Render(frame, 2)
			if err != nil {
				t.Fatal(err)
			}
			_, packet, err := protocol.Find(square)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := assembler.Add(packet, time.Now())
			if err != nil || decoded == nil {
				t.Fatalf("decoded %+v, %v", decoded, err)
			}
			state.Capture(true, true, nil)
			state.ObserveAddon(*decoded)
		}
		notice = reconcileAddonNotice(notice, state.Snapshot(), "0.8.3")
		if i == 0 && notice != addon.Reload {
			t.Fatal("old version cleared the notice")
		}
	}
	if notice != "" {
		t.Fatalf("current optical version did not clear notice: %s", notice)
	}
	if state.Snapshot().AddonVersion != "v0.8.3" {
		t.Fatal("lost decoded version")
	}
}
