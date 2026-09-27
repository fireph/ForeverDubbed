package update

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installedManifest(t *testing.T, root string, files map[string]string) {
	t.Helper()
	m := Manifest{Version: "0.9.0", Files: map[string]string{}}
	for name, data := range files {
		writeFixture(t, root, name, data)
		m.Files[name] = digest([]byte(data))
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, ManifestName, string(data))
}

func TestUpdateRemovesObsoleteFilesAndRollsBack(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "rollback"}[fail], func(t *testing.T) {
			install, plan := installFixture(t)
			installedManifest(t, install.Root, map[string]string{
				"foreverdubbed.exe":                 "old exe",
				"native/obsolete/voice.safetensors": "old voice",
			})
			writeFixture(t, install.Root, "native/personal.txt", "keep")
			if fail {
				if err := os.Remove(filepath.Join(filepath.Dir(plan), "payload", "tts", "voices.json")); err != nil {
					t.Fatal(err)
				}
			}
			err := Apply(plan, nil)
			if (err != nil) != fail {
				t.Fatalf("Apply: %v", err)
			}
			obsolete := filepath.Join(install.Root, "native", "obsolete", "voice.safetensors")
			if fail {
				data, err := os.ReadFile(obsolete)
				if err != nil || string(data) != "old voice" {
					t.Fatalf("obsolete file was not restored: %q, %v", data, err)
				}
				m, err := readManifest(filepath.Join(install.Root, ManifestName))
				if err != nil || m.Version != "0.9.0" {
					t.Fatal("old manifest was not retained", err)
				}
			} else {
				if _, err := os.Stat(filepath.Dir(obsolete)); !os.IsNotExist(err) {
					t.Fatal("obsolete directory survived", err)
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(plan), "previous", "native", "obsolete", "voice.safetensors")); err != nil {
					t.Fatal("obsolete file not backed up", err)
				}
			}
			data, err := os.ReadFile(filepath.Join(install.Root, "native", "personal.txt"))
			if err != nil || string(data) != "keep" {
				t.Fatal("unrelated file changed", err)
			}
		})
	}
}

func TestUpdateRefreshesUninstallInventory(t *testing.T) {
	install, plan := installFixture(t)
	// Add a file that the original installer could not have known about.
	payload := filepath.Join(filepath.Dir(plan), "payload")
	m, err := readManifest(filepath.Join(payload, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, payload, "native/new/voice.safetensors", "new voice")
	m.Files["native/new/voice.safetensors"] = digest([]byte("new voice"))
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, payload, ManifestName, string(data))
	if err := Apply(plan, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(install.Root, InventoryName))
	if err != nil {
		t.Fatal(err)
	}
	want, err := UninstallInventory(m.Files)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, inventoryBytes(want)) {
		t.Fatal("updated inventory does not match release")
	}
	if !strings.Contains(want, `Fnative\new\voice.safetensors`) {
		t.Fatal("new voice missing from inventory")
	}
}

func TestUpdateRollsBackUninstallInventory(t *testing.T) {
	install, plan := installFixture(t)
	old := "previous inventory"
	writeFixture(t, install.Root, InventoryName, old)
	var injected error
	err := Apply(plan, func(p Progress) {
		// The manifest is committed last, after the new inventory is installed.
		if p.Stage == "Installing update" && p.Done == p.Total-1 {
			injected = os.Remove(filepath.Join(filepath.Dir(plan), "payload", ManifestName))
		}
	})
	if injected != nil {
		t.Fatal(injected)
	}
	if err == nil {
		t.Fatal("expected late installation failure")
	}
	got, err := os.ReadFile(filepath.Join(install.Root, InventoryName))
	if err != nil || string(got) != old {
		t.Fatalf("inventory was not restored: %q, %v", got, err)
	}
}

func TestObsoleteCleanupRejectsSymlinks(t *testing.T) {
	install, plan := installFixture(t)
	installedManifest(t, install.Root, map[string]string{"foreverdubbed.exe": "old exe", "zobsolete/file": "old"})
	outside := t.TempDir()
	writeFixture(t, outside, "file", "outside")
	if err := os.RemoveAll(filepath.Join(install.Root, "zobsolete")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(install.Root, "zobsolete")); err != nil {
		t.Skip(err)
	}
	if err := Apply(plan, nil); err == nil {
		t.Fatal("followed a symlink")
	}
	for name, want := range map[string]string{install.Executable: "old exe", filepath.Join(outside, "file"): "outside"} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != want {
			t.Fatalf("file changed: %s: %q, %v", name, data, err)
		}
	}
}

func TestUninstallInventoryValidationAndEncoding(t *testing.T) {
	text, err := UninstallInventory(map[string]string{"docs/é $notes.txt": "hash", "native/models/model.onnx": "hash"})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []string{`Fdocs\é $notes.txt`, `Fnative\models\model.onnx`, `F` + ManifestName, `Dnative\models`, `Dnative`} {
		if !strings.Contains(text, record) {
			t.Errorf("missing %s", record)
		}
	}
	if strings.Index(text, `Dnative\models`) > strings.LastIndex(text, `Dnative`) {
		t.Fatal("parents precede children")
	}
	if data := inventoryBytes("é"); !bytes.Equal(data, []byte{255, 254, 233, 0}) {
		t.Fatalf("wrong Unicode encoding: %x", data)
	}
	for _, name := range []string{"../outside", "a*", "a?", "a\n", "a\x00", `C:/outside`, "Uninstall.exe", "UNINSTALL-FILES.INI", strings.Repeat("a", 801)} {
		if _, err := UninstallInventory(map[string]string{name: "hash"}); err == nil {
			t.Errorf("accepted invalid inventory file %q", name)
		}
	}
}
