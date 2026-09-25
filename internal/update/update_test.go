package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body []byte, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Header: http.Header{}}
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func fixtureRelease(archive []byte, osname, arch string) Release {
	name := "ForeverDubbed-windows-" + arch + "-portable.zip"
	if osname == "darwin" {
		name = "ForeverDubbed-mac-" + arch + ".zip"
	}
	base := Repository + "/releases/download/v0.10.0/"
	a := Asset{name, base + name, int64(len(archive))}
	sums := []byte(digest(archive) + "  " + name + "\n")
	checksums := Asset{"SHA256SUMS.txt", base + "SHA256SUMS.txt", int64(len(sums))}
	return Release{Tag: "v0.10.0", Assets: []Asset{a, checksums}, Archive: a, Checksums: checksums}
}
func clientFor(r Release, archive []byte) *Client {
	raw, _ := json.Marshal(r)
	sums := []byte(digest(archive) + "  " + r.Archive.Name + "\n")
	return &Client{LatestURL: latestURL, HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		switch req.URL.String() {
		case latestURL:
			return response(raw, 200), nil
		case r.Checksums.URL:
			return response(sums, 200), nil
		case r.Archive.URL:
			return response(archive, 200), nil
		}
		return response(nil, 404), nil
	})}}
}
func TestReleaseSelection(t *testing.T) {
	for _, platform := range []struct{ os, arch string }{{"windows", "amd64"}, {"darwin", "arm64"}, {"darwin", "amd64"}} {
		r := fixtureRelease([]byte("archive"), platform.os, platform.arch)
		client := clientFor(r, []byte("archive"))
		got, err := client.Check(context.Background(), "0.9.9", platform.os, platform.arch)
		if err != nil || got == nil || got.Archive.Name != r.Archive.Name {
			t.Fatalf("selection: %+v %v", got, err)
		}
		for _, version := range []string{"0.10.0", "1.0.0"} {
			got, err = client.Check(context.Background(), version, platform.os, platform.arch)
			if err != nil || got != nil {
				t.Fatalf("offered downgrade: %+v %v", got, err)
			}
		}
	}
	for _, change := range []func(*Release){func(r *Release) { r.Prerelease = true }, func(r *Release) { r.Draft = true }, func(r *Release) { r.Tag = "v1.0.0-rc.1" }} {
		r := fixtureRelease([]byte("archive"), "windows", "amd64")
		change(&r)
		got, err := clientFor(r, []byte("archive")).Check(context.Background(), "0.1.0", "windows", "amd64")
		if err != nil || got != nil {
			t.Fatalf("offered unstable release: %+v %v", got, err)
		}
	}
	for _, change := range []func(*Release){func(r *Release) { r.Assets[0].URL = "https://evil.example/update.zip" }, func(r *Release) { r.Assets = r.Assets[:1] }, func(r *Release) { r.Assets[0].Name = "ForeverDubbed-windows-amd64.zip" }, func(r *Release) { r.Assets[0].Size = 0 }} {
		r := fixtureRelease([]byte("archive"), "windows", "amd64")
		change(&r)
		if _, err := clientFor(r, []byte("archive")).Check(context.Background(), "0.1.0", "windows", "amd64"); err == nil {
			t.Fatal("accepted incomplete/unsafe release")
		}
	}
}
func TestVersions(t *testing.T) {
	for _, bad := range []string{"", "1.2", "1.2.3.4", "1.2.-3", "1.2.3-beta", "1.02.3", "+1.2.3", "1.2.18446744073709551616"} {
		if _, err := Version(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if !newer("v0.10.0", "0.9.9") || newer("1.0.0", "1.0.0") || newer("0.9.9", "0.10.0") {
		t.Fatal("incorrect version comparison")
	}
}
func TestVerifiedDownloadAndCancellation(t *testing.T) {
	archive := bytes.Repeat([]byte("payload"), 100000)
	r := fixtureRelease(archive, "windows", "amd64")
	client := clientFor(r, archive)
	path := filepath.Join(t.TempDir(), "download.zip")
	var progress []Progress
	if err := client.Download(context.Background(), &r, path, func(p Progress) { progress = append(progress, p) }); err != nil {
		t.Fatal(err)
	}
	actual, _ := os.ReadFile(path)
	if !bytes.Equal(actual, archive) || len(progress) < 3 {
		t.Fatal("missing download/progress")
	}
	for _, kind := range []string{"checksum", "truncated", "canceled", "http"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "download.zip")
			client := clientFor(r, archive)
			original := client.HTTP.Transport
			client.HTTP.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == r.Archive.URL {
					if kind == "checksum" {
						return response(bytes.Repeat([]byte("x"), len(archive)), 200), nil
					}
					if kind == "truncated" {
						res := response(archive[:5], 200)
						res.ContentLength = -1
						return res, nil
					}
					if kind == "http" {
						return response(nil, 429), nil
					}
				}
				return original.RoundTrip(req)
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "canceled" {
				cancel()
			}
			if err := client.Download(ctx, &r, path, nil); err == nil {
				t.Fatal("accepted bad download")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("left unverified archive")
			}
		})
	}
}
func makeZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(f, data)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestArchiveValidation(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "a/../../b", `a\b`, "a:stream", "CON.txt", "a/COM1.log", "a./b"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "a.zip")
			os.WriteFile(archive, makeZIP(t, map[string]string{name: "bad"}), 0600)
			if err := extract(context.Background(), archive, filepath.Join(root, "payload"), nil); err == nil {
				t.Fatal("accepted unsafe path")
			}
		})
	}
	root := t.TempDir()
	archive := filepath.Join(root, "a.zip")
	os.WriteFile(archive, makeZIP(t, map[string]string{"Foo.txt": "a", "foo.txt": "b"}), 0600)
	if err := extract(context.Background(), archive, filepath.Join(root, "payload"), nil); err == nil {
		t.Fatal("accepted case collision")
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	header := &zip.FileHeader{Name: "link"}
	header.SetMode(os.ModeSymlink | 0777)
	f, _ := w.CreateHeader(header)
	io.WriteString(f, "/tmp")
	w.Close()
	os.WriteFile(archive, b.Bytes(), 0600)
	if err := extract(context.Background(), archive, filepath.Join(root, "payload"), nil); err == nil {
		t.Fatal("accepted symlink")
	}
}
func writeFixture(t *testing.T, root, name, data string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}
func installFixture(t *testing.T) (Installation, string) {
	t.Helper()
	root := t.TempDir()
	exe := filepath.Join(root, "foreverdubbed.exe")
	helper := filepath.Join(root, "foreverdubbed-updater.exe")
	install := Installation{Root: root, Executable: exe, Helper: helper, OS: "windows", Arch: "amd64"}
	writeFixture(t, root, "foreverdubbed.exe", "old exe")
	writeFixture(t, root, "foreverdubbed-updater.exe", "old helper")
	writeFixture(t, root, "tts/voices.json", "my edited voice config")
	writeFixture(t, root, "unrelated.txt", "keep me")
	job, err := os.MkdirTemp(root, ".foreverdubbed-update-")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"foreverdubbed.exe": "new exe", "foreverdubbed-updater.exe": "new helper", "tts/voices.json": "release voice config"}
	m := Manifest{Version: "0.10.0", Files: map[string]string{}}
	for name, data := range files {
		writeFixture(t, filepath.Join(job, "payload"), name, data)
		m.Files[name] = digest([]byte(data))
	}
	raw, _ := json.Marshal(m)
	writeFixture(t, filepath.Join(job, "payload"), ManifestName, string(raw))
	path := filepath.Join(job, "plan.json")
	if err := writePlan(path, Plan{Install: install, Version: "0.10.0", ParentPID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	return install, path
}
func TestApplyReplacesBundledConfiguration(t *testing.T) {
	install, plan := installFixture(t)
	var progress []Progress
	if err := Apply(plan, func(p Progress) { progress = append(progress, p) }); err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string]string{"foreverdubbed.exe": "new exe", "tts/voices.json": "release voice config", "unrelated.txt": "keep me"} {
		got, _ := os.ReadFile(filepath.Join(install.Root, filepath.FromSlash(name)))
		if string(got) != expected {
			t.Fatalf("%s: %s", name, got)
		}
	}
	if len(progress) == 0 || progress[len(progress)-1].Done != progress[len(progress)-1].Total {
		t.Fatal("missing completion progress")
	}
	backup, _ := os.ReadFile(filepath.Join(filepath.Dir(plan), "previous", "foreverdubbed.exe"))
	if string(backup) != "old exe" {
		t.Fatal("missing recovery copy")
	}
}
func TestApplyRollsBackOnPartialFailure(t *testing.T) {
	install, plan := installFixture(t)
	// Force a late failure after both executables have already been replaced.
	os.Remove(filepath.Join(filepath.Dir(plan), "payload", "tts", "voices.json"))
	if err := Apply(plan, nil); err == nil {
		t.Fatal("missing staged file did not fail")
	}
	for name, expected := range map[string]string{"foreverdubbed.exe": "old exe", "foreverdubbed-updater.exe": "old helper", "tts/voices.json": "my edited voice config"} {
		got, _ := os.ReadFile(filepath.Join(install.Root, filepath.FromSlash(name)))
		if string(got) != expected {
			t.Fatalf("rollback failed for %s: %s", name, got)
		}
	}
}
func TestApplyRejectsSymlinkParents(t *testing.T) {
	install, plan := installFixture(t)
	outside := t.TempDir()
	if err := os.RemoveAll(filepath.Join(install.Root, "tts")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(install.Root, "tts")); err != nil {
		t.Skip(err)
	}
	if err := Apply(plan, nil); err == nil {
		t.Fatal("followed symlink outside install")
	}
	if _, err := os.Stat(filepath.Join(outside, "voices.json")); !os.IsNotExist(err) {
		t.Fatal("wrote outside install")
	}
	got, _ := os.ReadFile(install.Executable)
	if string(got) != "old exe" {
		t.Fatal("did not roll back")
	}
}
func TestPrepareEndToEnd(t *testing.T) {
	install, _ := installFixture(t)
	files := map[string]string{"foreverdubbed.exe": "new exe", "foreverdubbed-updater.exe": "new helper", "tts/voices.json": "new config"}
	manifest := Manifest{Version: "0.10.0", Files: map[string]string{}}
	for name, data := range files {
		manifest.Files[name] = digest([]byte(data))
	}
	raw, _ := json.Marshal(manifest)
	files[ManifestName] = string(raw)
	archive := makeZIP(t, files)
	r := fixtureRelease(archive, "windows", "amd64")
	c := clientFor(r, archive)
	plan, err := Prepare(context.Background(), c, &r, install, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := LoadPlan(plan)
	if err != nil || p.Version != "0.10.0" {
		t.Fatal(p, err)
	}
	helper, _ := os.ReadFile(filepath.Join(filepath.Dir(plan), "foreverdubbed-updater.exe"))
	if string(helper) != "old helper" {
		t.Fatal("helper not staged separately")
	}
	got, _ := os.ReadFile(install.Executable)
	if string(got) != "old exe" {
		t.Fatal("preparation changed running app")
	}
	if err := Apply(plan, nil); err != nil {
		t.Fatal(err)
	}
	// Bad packaged versions are rejected before touching the installed app.
	files[ManifestName] = strings.Replace(string(raw), "0.10.0", "0.9.0", 1)
	archive = makeZIP(t, files)
	r = fixtureRelease(archive, "windows", "amd64")
	if _, err = Prepare(context.Background(), clientFor(r, archive), &r, install, nil); err == nil {
		t.Fatal("accepted version mismatch")
	}
}
func TestChecksumParser(t *testing.T) {
	hash := digest([]byte("x"))
	name := "file.zip"
	for _, line := range []string{hash + "  " + name + "\n", hash + " *" + name + "\n"} {
		got, err := checksum([]byte(line), name)
		if err != nil || got != hash {
			t.Fatal(got, err)
		}
	}
	if _, err := checksum([]byte(fmt.Sprintf("%s  %s\n%s  %s\n", hash, name, hash, name)), name); err == nil {
		t.Fatal("accepted duplicate checksum")
	}
}
