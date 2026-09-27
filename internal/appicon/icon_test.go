package appicon

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// The checked-in resource lets ordinary builds work without windres. Catch
// artwork changes that would otherwise silently ship the previous icon.
func TestWindowsIconResourceMatchesArtwork(t *testing.T) {
	ico, err := os.ReadFile("assets/forever-dubbed-logo.ico")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := os.ReadFile("windowsresource/icon_windows_amd64.syso")
	if err != nil {
		t.Fatal(err)
	}
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:]) != 1 {
		t.Fatal("invalid ICO header")
	}
	count := int(binary.LittleEndian.Uint16(ico[4:]))
	if count == 0 || len(ico) < 6+16*count {
		t.Fatal("invalid ICO directory")
	}
	for i := 0; i < count; i++ {
		entry := ico[6+16*i:]
		size := uint64(binary.LittleEndian.Uint32(entry[8:]))
		offset := uint64(binary.LittleEndian.Uint32(entry[12:]))
		if size == 0 || offset+size > uint64(len(ico)) {
			t.Fatalf("invalid ICO image %d", i)
		}
		if !bytes.Contains(resource, ico[offset:offset+size]) {
			t.Fatal("Windows icon resource is stale; run go generate ./internal/appicon after changing the ICO")
		}
	}
}
