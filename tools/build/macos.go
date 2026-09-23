package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"foreverdubbed/internal/appicon"
)

// macApp stages a standalone Finder-launchable app before replacing the previous
// generated app. Documentation and the addon stay beside it in the release ZIP.
func macApp(dist string, files map[string]string) (map[string]string, error) {
	stage, err := os.MkdirTemp(dist, ".foreverdubbed-app-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	result := make(map[string]string)
	appFiles := make(map[string]string)
	for name, source := range files {
		var target string
		switch {
		case name == "foreverdubbed":
			target = "Contents/MacOS/foreverdubbed"
		case strings.HasPrefix(name, "native/") || strings.HasPrefix(name, "tts/"):
			target = "Contents/Resources/" + name
		default:
			result[name] = source
			continue
		}
		destination := filepath.Join(stage, filepath.FromSlash(target))
		if err := copyBundleFile(source, destination); err != nil {
			return nil, err
		}
		appFiles[target] = destination
	}
	if _, ok := appFiles["Contents/MacOS/foreverdubbed"]; !ok {
		return nil, fmt.Errorf("macOS app is missing its executable")
	}
	const plist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>foreverdubbed</string>
<key>CFBundleIdentifier</key><string>io.foreverdubbed.companion</string>
<key>CFBundleName</key><string>ForeverDubbed</string>
<key>CFBundleDisplayName</key><string>ForeverDubbed</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.5.0</string>
<key>CFBundleVersion</key><string>0.5.0</string>
<key>CFBundleIconFile</key><string>app.icns</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
`
	for name, data := range map[string][]byte{"Contents/Info.plist": []byte(plist), "Contents/Resources/app.icns": macIcon()} {
		destination := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(destination, data, 0644); err != nil {
			return nil, err
		}
		appFiles[name] = destination
	}
	destination := filepath.Join(dist, "ForeverDubbed.app")
	if err := os.RemoveAll(destination); err != nil {
		return nil, err
	}
	if err := os.Rename(stage, destination); err != nil {
		return nil, err
	}
	for name := range appFiles {
		result["ForeverDubbed.app/"+name] = filepath.Join(destination, filepath.FromSlash(name))
	}
	return result, nil
}
func copyBundleFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func macIcon() []byte {
	var payload bytes.Buffer
	for _, size := range []struct {
		code   string
		pixels int
	}{{"icp5", 32}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024}} {
		data := appicon.PNG(size.pixels)
		payload.WriteString(size.code)
		_ = binary.Write(&payload, binary.BigEndian, uint32(len(data)+8))
		payload.Write(data)
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(payload.Len()+8))
	out.Write(payload.Bytes())
	return out.Bytes()
}
