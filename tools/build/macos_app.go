package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"foreverdubbed/internal/appicon"
	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/buildtool"
	"foreverdubbed/internal/update"
)

// macApp stages a Finder-launchable bundle before replacing the previous build.
// Documentation and the addon stay beside it in the release ZIP.
func macApp(dist string, files map[string]string, sign func(string) error) (map[string]string, error) {
	staging, err := os.MkdirTemp(dist, ".foreverdubbed-app-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	stage := filepath.Join(staging, "ForeverDubbed.app")
	result := make(map[string]string)
	appFiles := make(map[string]string)
	for name, source := range files {
		var target string
		switch {
		case name == "foreverdubbed" || name == "foreverdubbed-updater":
			target = "Contents/MacOS/" + name
		case name == update.ManifestName:
			target = "Contents/Resources/" + name
		case strings.HasPrefix(name, "docs/licenses/"):
			// Keep redistributable font notices with the standalone app too.
			result[name] = source
			target = "Contents/Resources/licenses/" + strings.TrimPrefix(name, "docs/licenses/")
		case strings.HasPrefix(name, "native/") || strings.HasPrefix(name, "tts/") || strings.HasPrefix(name, "data/"):
			target = "Contents/Resources/" + name
		default:
			result[name] = source
			continue
		}
		destination := filepath.Join(stage, filepath.FromSlash(target))
		if err := buildtool.CopyFile(source, destination); err != nil {
			return nil, err
		}
		appFiles[target] = destination
	}
	if _, ok := appFiles["Contents/MacOS/foreverdubbed"]; !ok {
		return nil, fmt.Errorf("macOS app is missing its executable")
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>foreverdubbed</string>
<key>CFBundleIdentifier</key><string>io.foreverdubbed.companion</string>
<key>CFBundleName</key><string>ForeverDubbed</string>
<key>CFBundleDisplayName</key><string>ForeverDubbed</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>` + buildinfo.Version + `</string>
<key>CFBundleVersion</key><string>` + buildinfo.Version + `</string>
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
	if sign != nil {
		if err := sign(stage); err != nil {
			return nil, err
		}
	}
	// Signing adds CodeResources and may change nested code. Build the archive
	// manifest from the signed app, never from the pre-signing inputs.
	appFiles = make(map[string]string)
	if err := filepath.WalkDir(stage, func(source string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if err := buildtool.RegularFile(source); err != nil {
			return err
		}
		name, err := filepath.Rel(stage, source)
		if err == nil {
			appFiles[filepath.ToSlash(name)] = source
		}
		return err
	}); err != nil {
		return nil, err
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

// macSigner requires a persistent identity unless unsigned output was explicitly
// requested. Missing credentials must not silently change the app's identity.
func macSigner(root string, unsigned bool) (func(string) error, error) {
	if unsigned {
		return nil, nil
	}
	identity := os.Getenv("FDB_MACOS_SIGNING_PEM")
	if identity == "" {
		identity = filepath.Join(root, ".runtime", "macos-signing", "identity.pem")
	}
	identity, err := filepath.Abs(identity)
	if err != nil {
		return nil, err
	}
	if err := buildtool.RegularFile(identity); err != nil {
		return nil, fmt.Errorf("macOS signing identity: %w; run bash scripts/setup-macos-signing.sh once, restore FDB_MACOS_SIGNING_PEM, or explicitly use -mac-unsigned for a test build", err)
	}
	signer := os.Getenv("FDB_RCODESIGN")
	if signer == "" {
		signer = filepath.Join(root, ".runtime", "rcodesign", "0.29.0", "rcodesign")
	}
	signer, err = exec.LookPath(signer)
	if err != nil {
		return nil, fmt.Errorf("rcodesign: %w; run bash scripts/setup-macos-signing.sh --tools-only", err)
	}
	return func(app string) error {
		cmd := exec.Command(signer, "--config-file", "/dev/null", "sign", "--pem-file", identity, "--timestamp-url", "none", app)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sign macOS app: %w", err)
		}
		cmd = exec.Command(signer, "--config-file", "/dev/null", "verify", filepath.Join(app, "Contents", "MacOS", "foreverdubbed"))
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("verify signed macOS executable: %w", err)
		}
		return nil
	}, nil
}
