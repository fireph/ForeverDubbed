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
	im.SetRGBA(Outline+19, Outline+99, colors[0])
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
	if Grid*2+2*Outline != 104 || DataGrid*DataGrid*4/8 != FrameBytes || PayloadBytes != 1124 {
		t.Fatal("incorrect layout")
	}
	for i, a := range colors {
		for j, b := range colors {
			if i != j && distance([3]int{int(a.R), int(a.G), int(a.B)}, [3]int{int(b.R), int(b.G), int(b.B)}) < 37 {
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
	for _, magic := range []string{"FDB1", "FDB2", "FDB3"} {
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
	for _, gamma := range [][3]float64{{1, 1, 1}, {0.8, 0.9, 1.0}, {1.1, 1.0, 0.9}, {1.2, 1.1, 1.0}} {
		im, _ := Render(b, 2)
		for i := 0; i < len(im.Pix); i += 4 {
			for ch := 0; ch < 3; ch++ {
				value := float64(im.Pix[i+ch]) / 255
				offset, gain := [3]float64{12, 18, 6}[ch], [3]float64{230, 220, 238}[ch]
				im.Pix[i+ch] = byte(math.Round(offset + gain*math.Pow(value, gamma[ch])))
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
		func(im *image.RGBA) { im.SetRGBA(Outline+19, Outline+99, colors[0]) }, // duplicate reference
		func(im *image.RGBA) { im.SetRGBA(Outline+3, Outline+99, colors[7]) },  // duplicated first reference
		func(im *image.RGBA) {
			im.SetRGBA(Outline+19, Outline+99, colors[9])
			im.SetRGBA(Outline+21, Outline+99, colors[8])
		}, // swapped references
		func(im *image.RGBA) { im.SetRGBA(Outline+3, Outline+3, color.RGBA{16, 26, 50, 255}) }, // ambiguous between references 0 and E
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

func TestOutlineGeometryAndRejection(t *testing.T) {
	for cell := 2; cell <= 8; cell++ {
		im, _ := Render(colorFixture(t), cell)
		size := Grid*cell + 4
		if im.Bounds() != image.Rect(0, 0, size, size) {
			t.Fatal("wrong outer dimensions")
		}
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				outer := x < 2 || y < 2 || x >= size-2 || y >= size-2
				if (im.RGBAAt(x, y) == finderColor) != outer {
					t.Fatalf("outline thickness changed at %d,%d for cell %d", x, y, cell)
				}
			}
		}
		l, _, err := Find(im)
		if err != nil || l != (Location{Cell: cell}) {
			t.Fatalf("edge discovery: %v %v", l, err)
		}
		if _, _, err := Find(im.SubImage(image.Rect(1, 0, size, size))); err == nil {
			t.Fatal("clipped outline accepted")
		}
		// Each side, including its inner pixel, must be intact.
		for _, pt := range []image.Point{{size / 2, 1}, {1, size / 2}, {size - 2, size / 2}, {size / 2, size - 2}} {
			im.SetRGBA(pt.X, pt.Y, colors[0])
			if _, _, err := Find(im); err == nil {
				t.Fatal("damaged outline accepted", pt)
			}
			im.SetRGBA(pt.X, pt.Y, finderColor)
		}
	}
	// An ordinary blue box is not a valid tile.
	im, _ := Render(colorFixture(t), 2)
	for y := Outline; y < im.Bounds().Max.Y-Outline; y++ {
		for x := Outline; x < im.Bounds().Max.X-Outline; x++ {
			im.SetRGBA(x, y, colors[0])
		}
	}
	if _, _, err := Find(im); err == nil {
		t.Fatal("plain blue box accepted")
	}
}

func TestSmallNoiseAndCollapsedPalette(t *testing.T) {
	frame := colorFixture(t)
	im, _ := Render(frame, 2)
	// A one-level perturbation per data channel remains within the source
	// palette's decision radius. References remain intact in this fixture.
	for y := 1; y < Grid-1; y++ {
		for x := 1; x < Grid-1; x++ {
			px, py := Outline+x*2+1, Outline+y*2+1
			c := im.RGBAAt(px, py)
			im.SetRGBA(px, py, color.RGBA{c.R + 1, c.G + 1, c.B + 1, 255})
		}
	}
	if _, p, err := Find(im); err != nil || !reflect.DeepEqual(p, packetFor(t, frame)) {
		t.Fatal("small noise", err)
	}
	// Aggressive dark-level quantization can merge nearby colors; reject it.
	im, _ = Render(frame, 2)
	for i := 0; i < len(im.Pix); i += 4 {
		for ch := 0; ch < 3; ch++ {
			im.Pix[i+ch] = (im.Pix[i+ch] / 16) * 16
		}
	}
	if _, _, err := Find(im); err == nil {
		t.Fatal("collapsed palette accepted")
	}
}
