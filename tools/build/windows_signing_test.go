package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"foreverdubbed/internal/update"
)

func TestWindowsSignedPayloadManifestAndArchive(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{}
	for _, name := range []string{"foreverdubbed.exe", "foreverdubbed-updater.exe", "onnxruntime.dll"} {
		putFile(t, root, name, "unsigned")
		files[name] = filepath.Join(root, name)
	}
	if err := signWindowsPayload(files, func(file string) error {
		return os.WriteFile(file, []byte("signed "+filepath.Base(file)), 0644)
	}); err != nil {
		t.Fatal(err)
	}
	if err := addReleaseManifest(root, files); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "signed.zip")
	if err := writeZIP(archive, files); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	contents := map[string][]byte{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		contents[f.Name], err = io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	var manifest update.Manifest
	if err := json.Unmarshal(contents[update.ManifestName], &manifest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"foreverdubbed.exe", "foreverdubbed-updater.exe"} {
		if string(contents[name]) != "signed "+name {
			t.Fatalf("unsigned archive entry: %s", name)
		}
		hash := sha256.Sum256(contents[name])
		if manifest.Files[name] != hex.EncodeToString(hash[:]) {
			t.Fatalf("stale manifest hash: %s", name)
		}
	}
	if string(contents["onnxruntime.dll"]) != "unsigned" {
		t.Fatal("third-party DLL was modified")
	}
}

func TestWindowsSigningFailureStopsPayload(t *testing.T) {
	want := errors.New("Azure signing failed")
	calls := 0
	err := signWindowsPayload(map[string]string{"foreverdubbed.exe": "app", "foreverdubbed-updater.exe": "helper"}, func(string) error {
		calls++
		return want
	})
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("got %v after %d calls", err, calls)
	}
	if err := signWindowsPayload(nil, func(string) error { t.Fatal("signed missing input"); return nil }); err == nil {
		t.Fatal("accepted missing executable")
	}
	if err := signWindowsPayload(nil, nil); err != nil {
		t.Fatal(err)
	}
}
