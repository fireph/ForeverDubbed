package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"foreverdubbed/internal/pocket"
	"foreverdubbed/internal/speech"
)

func TestMacAppLayoutAndExecutablePermissions(t *testing.T) {
	root := t.TempDir()
	names := []string{"foreverdubbed", "native/libonnxruntime.dylib", "native/models/bundle.json", "native/presets/alba.safetensors", "tts/voices.json", "tts/custom/narrator.safetensors", "README.md", "addon/ForeverDubbed/ForeverDubbed.toc"}
	files := map[string]string{}
	for _, name := range names {
		putFile(t, root, name, name)
		files[name] = filepath.Join(root, filepath.FromSlash(name))
	}
	if err := os.Chmod(files["foreverdubbed"], 0755); err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(root, "dist")
	if err := os.MkdirAll(dist, 0755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dist, "ForeverDubbed.app", "stale")
	putFile(t, dist, "ForeverDubbed.app/stale", "old")
	manifest, err := macApp(dist, files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale app file survived rebuild")
	}
	for _, name := range names {
		target := name
		if name == "foreverdubbed" {
			target = "ForeverDubbed.app/Contents/MacOS/foreverdubbed"
		} else if strings.HasPrefix(name, "native/") || strings.HasPrefix(name, "tts/") {
			target = "ForeverDubbed.app/Contents/Resources/" + name
		}
		if manifest[target] == "" {
			t.Fatalf("missing %s", target)
		}
	}
	info, err := os.ReadFile(manifest["ForeverDubbed.app/Contents/Info.plist"])
	if err != nil || !bytes.Contains(info, []byte("CFBundleExecutable")) || !bytes.Contains(info, []byte("<string>APPL</string>")) {
		t.Fatalf("invalid plist: %v", err)
	}
	icon, err := os.ReadFile(manifest["ForeverDubbed.app/Contents/Resources/app.icns"])
	if err != nil || len(icon) < 8 || string(icon[:4]) != "icns" || int(binary.BigEndian.Uint32(icon[4:8])) != len(icon) {
		t.Fatalf("invalid icon: %v", err)
	}
	zipPath := filepath.Join(root, "release.zip")
	if err := writeZIP(zipPath, manifest); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, f := range archive.File {
		if f.Name == "ForeverDubbed.app/Contents/MacOS/foreverdubbed" && f.Mode().Perm()&0111 == 0 {
			t.Fatal("ZIP lost executable permission")
		}
	}
}

// Run a copied test executable from a Finder-style app layout with an unrelated
// working directory. This checks actual os.Executable-based resource discovery.
func TestMacAppResourceDiscovery(t *testing.T) {
	if os.Getenv("FDB_TEST_APP_PROBE") == "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		resources := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "Resources"))
		if got := pocket.DefaultDir(); got != filepath.Join(resources, "native") {
			t.Fatalf("native resources: %s", got)
		}
		if got := speech.DefaultConfigPath(); got != filepath.Join(resources, "tts", "voices.json") {
			t.Fatalf("voice config: %s", got)
		}
		return
	}
	root := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(root, "ForeverDubbed.app", "Contents", "MacOS", "probe")
	if err := copyBundleFile(exe, probe); err != nil {
		t.Fatal(err)
	}
	putFile(t, root, "ForeverDubbed.app/Contents/Resources/native/models/bundle.json", "{}")
	putFile(t, root, "ForeverDubbed.app/Contents/Resources/tts/voices.json", "{}")
	cmd := exec.Command(probe, "-test.run=^TestMacAppResourceDiscovery$")
	cmd.Env = append(os.Environ(), "FDB_TEST_APP_PROBE=1")
	cmd.Dir = t.TempDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("app resource discovery: %v\n%s", err, out)
	}
}

func TestSignedAppManifestAndFailedSigning(t *testing.T) {
	root := t.TempDir()
	putFile(t, root, "foreverdubbed", "executable")
	dist := filepath.Join(root, "dist")
	putFile(t, dist, "ForeverDubbed.app/previous", "keep until signing succeeds")
	files := map[string]string{"foreverdubbed": filepath.Join(root, "foreverdubbed")}
	_, err := macAppSigned(dist, files, func(app string) error { return fmt.Errorf("signing failed") })
	if err == nil {
		t.Fatal("ignored signing failure")
	}
	if _, err := os.Stat(filepath.Join(dist, "ForeverDubbed.app/previous")); err != nil {
		t.Fatal("failed signing replaced installed build", err)
	}
	manifest, err := macAppSigned(dist, files, func(app string) error {
		if !strings.HasSuffix(app, ".app") {
			t.Fatal("signer needs an app bundle path")
		}
		putFile(t, app, "Contents/_CodeSignature/CodeResources", "signed resources")
		return os.WriteFile(filepath.Join(app, "Contents/MacOS/foreverdubbed"), []byte("signed executable"), 0755)
	})
	if err != nil {
		t.Fatal(err)
	}
	name := "ForeverDubbed.app/Contents/_CodeSignature/CodeResources"
	if manifest[name] == "" {
		t.Fatal("signature resource missing from ZIP manifest")
	}
	content, err := os.ReadFile(manifest["ForeverDubbed.app/Contents/MacOS/foreverdubbed"])
	if err != nil || string(content) != "signed executable" {
		t.Fatal("packaged original unsigned executable", err)
	}
}

func TestMacSignerRequiresPersistentIdentity(t *testing.T) {
	t.Setenv("FDB_MACOS_SIGNING_PEM", "")
	root := t.TempDir()
	if _, err := macSigner(root, false); err == nil {
		t.Fatal("missing identity silently accepted")
	}
	if sign, err := macSigner(root, true); err != nil || sign != nil {
		t.Fatal("explicit unsigned test build failed", err)
	}
}
