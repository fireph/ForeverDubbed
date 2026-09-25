package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foreverdubbed/addon"
	"foreverdubbed/internal/buildinfo"
)

func TestStageAddonUsesReleaseVersionInBothPackages(t *testing.T) {
	original := buildinfo.Version
	defer func() { buildinfo.Version = original }()
	t.Setenv("GITHUB_REF_TYPE", "tag")
	t.Setenv("GITHUB_REF_NAME", "v9.12.3")
	release, err := releaseVersion()
	if err != nil {
		t.Fatal(err)
	}
	buildinfo.Version = release
	root := t.TempDir()
	dist := filepath.Join(root, "dist")
	os.MkdirAll(dist, 0755)
	const toc = "## Interface: 16001\r\n## Version: 1.2.3\r\n\r\nForeverDubbed.lua\r\n"
	putFile(t, root, "addon/ForeverDubbed/ForeverDubbed.toc", toc)
	putFile(t, root, "addon/ForeverDubbed/ForeverDubbed.lua", "return 'addon'")
	addonFiles := map[string]string{}
	for _, name := range []string{"ForeverDubbed.toc", "ForeverDubbed.lua"} {
		addonFiles["ForeverDubbed/"+name] = filepath.Join(root, "addon", "ForeverDubbed", name)
	}
	bundle := map[string]string{}
	if err = stageAddon(dist, bundle, addonFiles); err != nil {
		t.Fatal(err)
	}
	source, _ := os.ReadFile(filepath.Join(root, "addon", "ForeverDubbed", "ForeverDubbed.toc"))
	if string(source) != toc {
		t.Fatal("tag build modified source metadata")
	}
	if addonFiles["ForeverDubbed/ForeverDubbed.toc"] != bundle["addon/ForeverDubbed/ForeverDubbed.toc"] {
		t.Fatal("standalone and desktop addons use different versions")
	}
	for _, item := range []struct {
		name, key string
		files     map[string]string
	}{{"addon.zip", "ForeverDubbed/ForeverDubbed.toc", addonFiles}, {"desktop.zip", "addon/ForeverDubbed/ForeverDubbed.toc", bundle}} {
		archive := filepath.Join(dist, item.name)
		if err = writeZIP(archive, item.files); err != nil {
			t.Fatal(err)
		}
		z, err := zip.OpenReader(archive)
		if err != nil {
			t.Fatal(err)
		}
		f, err := z.Open(item.key)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		z.Close()
		if err != nil || string(data) != strings.Replace(toc, "1.2.3", release, 1) {
			t.Fatalf("%s: incorrect release TOC: %s %v", item.name, data, err)
		}
	}
}
func TestDefaultVersionComesFromAddonMetadata(t *testing.T) {
	if buildinfo.Version != addon.Version() {
		t.Fatalf("desktop %s differs from addon %s", buildinfo.Version, addon.Version())
	}
}
