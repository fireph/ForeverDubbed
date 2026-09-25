package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/update"
)

func TestReleaseVersionFromTag(t *testing.T) {
	t.Setenv("GITHUB_REF_TYPE", "tag")
	t.Setenv("GITHUB_REF_NAME", "v1.12.3")
	got, err := releaseVersion()
	if err != nil || got != "1.12.3" {
		t.Fatal(got, err)
	}
	t.Setenv("GITHUB_REF_NAME", "v1.12.3-beta")
	if _, err = releaseVersion(); err == nil {
		t.Fatal("accepted prerelease tag")
	}
	t.Setenv("GITHUB_REF_TYPE", "branch")
	got, err = releaseVersion()
	if err != nil || got != buildinfo.Version {
		t.Fatal(got, err)
	}
}
func TestReleaseManifest(t *testing.T) {
	root := t.TempDir()
	putFile(t, root, "app.exe", "app")
	files := map[string]string{"foreverdubbed.exe": filepath.Join(root, "app.exe")}
	if err := addReleaseManifest(root, files); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(files[update.ManifestName])
	if err != nil {
		t.Fatal(err)
	}
	var m update.Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Version != buildinfo.Version || len(m.Files) != 1 || len(m.Files["foreverdubbed.exe"]) != 64 {
		t.Fatal(m)
	}
}
