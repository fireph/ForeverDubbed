package protocol

import (
	"bytes"
	"encoding/binary"
	"hash/adler32"
	"image"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"
)

func colorFixture(t *testing.T) []byte {
	t.Helper()
	b := framesFor(t, Message{Text: "palette coverage"})[0]
	// Exercise every high/low nibble, including colors absent from ASCII prose.
	for i := 0; i < PayloadBytes; i++ {
		b[HeaderBytes+i] = byte(i)
	}
	binary.BigEndian.PutUint16(b[20:], PayloadBytes)
	binary.BigEndian.PutUint32(b[12:], adler32.Checksum(b[HeaderBytes:FrameBytes-4]))
	binary.BigEndian.PutUint32(b[FrameBytes-4:], adler32.Checksum(b[:FrameBytes-4]))
	return b
}

func TestDiscoveryExplainsRejectedCandidate(t *testing.T) {
	im, _ := Render(colorFixture(t), 2)
	im.SetRGBA(19, 99, colors[0])
	_, _, err := Find(im)
	if err == nil || !strings.Contains(err.Error(), "finder detected") || !strings.Contains(err.Error(), "palette") {
		t.Fatalf("missing candidate diagnostics: %v", err)
	}
	_, _, err = Find(image.NewRGBA(image.Rect(0, 0, 100, 100)))
	if err == nil || !strings.Contains(err.Error(), "no tile finder") {
		t.Fatalf("missing discovery diagnostics: %v", err)
	}
}

func TestPaletteCapacityAndSeparation(t *testing.T) {
	if Grid*2 != 100 || DataGrid*DataGrid*4/8 != FrameBytes || PayloadBytes != 1124 {
		t.Fatal("incorrect layout")
	}
	for i, a := range colors {
		for j, b := range colors {
			if i != j && distance([3]int{int(a.R), int(a.G), int(a.B)}, [3]int{int(b.R), int(b.G), int(b.B)}) < 127*127 {
				t.Fatal("palette colors too close")
			}
		}
	}
	for _, n := range []int{1, 1122, 1123, 2246} {
		frames := framesFor(t, Message{Text: string(bytes.Repeat([]byte{'x'}, n))})
		if len(frames) != (n+2+PayloadBytes-1)/PayloadBytes {
			t.Fatal("incorrect page boundary")
		}
	}
	for _, magic := range []string{"FDB1", "FDB2"} {
		b := colorFixture(t)
		copy(b, magic)
		binary.BigEndian.PutUint32(b[FrameBytes-4:], adler32.Checksum(b[:FrameBytes-4]))
		if _, err := Parse(b); err == nil {
			t.Fatal("obsolete format accepted")
		}
	}
}

func TestCalibratedGammaAndTint(t *testing.T) {
	b := colorFixture(t)
	for _, gamma := range [][3]float64{{1, 1, 1}, {0.5, 0.7, 0.9}, {1.8, 1.4, 1.2}, {2.2, 1.8, 2.0}} {
		im, _ := Render(b, 2)
		for i := 0; i < len(im.Pix); i += 4 {
			for ch := 0; ch < 3; ch++ {
				value := float64(im.Pix[i+ch]) / 255
				offset, gain := [3]float64{12, 18, 6}[ch], [3]float64{230, 220, 238}[ch]
				// Deterministic per-pixel noise as well as a uniform channel transform.
				noise := float64((i/4+ch)%5 - 2)
				im.Pix[i+ch] = byte(math.Round(offset + gain*math.Pow(value, gamma[ch]) + noise))
			}
		}
		l, p, err := Find(im)
		if err != nil || l.Cell != 2 || !reflect.DeepEqual(p, packetFor(t, b)) {
			t.Fatalf("gamma %v: %v", gamma, err)
		}
		crop := im.SubImage(l.Rect())
		if _, err := Decode(crop, l); err != nil {
			t.Fatal("tracking failed", err)
		}
	}
}

func TestDamagedCalibrationAndAmbiguousData(t *testing.T) {
	b := colorFixture(t)
	for _, damage := range []func(*image.RGBA){
		func(im *image.RGBA) { im.SetRGBA(19, 99, colors[0]) },                                // duplicate reference
		func(im *image.RGBA) { im.SetRGBA(3, 99, colors[7]) },                                 // invalid black anchor
		func(im *image.RGBA) { im.SetRGBA(19, 99, colors[9]); im.SetRGBA(21, 99, colors[8]) }, // swapped references
		func(im *image.RGBA) { im.SetRGBA(3, 3, color.RGBA{64, 0, 0, 255}) },                  // ambiguous between black and dark red
	} {
		im, _ := Render(b, 2)
		damage(im)
		if _, err := Decode(im, Location{Cell: 2}); err == nil {
			t.Fatal("damaged frame accepted")
		}
		if _, _, err := Find(im); err == nil {
			t.Fatal("damaged frame discovered")
		}
	}
}
