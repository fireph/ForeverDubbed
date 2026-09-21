package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"hash/adler32"
	"image"
	"image/color"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixture() Message {
	return Message{Session: 123456789, Sequence: 42, Kind: 2, Speaker: "Thrall", Title: "A new beginning", Text: "Hello, champion! " + strings.Repeat("Café — 世界. ", 90)}
}

func framesFor(t *testing.T, m Message) [][]byte {
	t.Helper()
	f, e := Encode(m)
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func packetFor(t *testing.T, b []byte) Packet {
	t.Helper()
	p, e := Parse(b)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestMultipageUnicodeOutOfOrder(t *testing.T) {
	want := fixture()
	frames := framesFor(t, want)
	var a Assembler
	for i := len(frames) - 1; i >= 0; i-- {
		p := packetFor(t, frames[i])
		m, e := a.Add(p, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			if m == nil || *m != want {
				t.Fatalf("got %#v", m)
			}
		} else if m != nil {
			t.Fatal("premature message")
		}
		if m, e := a.Add(p, time.Now()); m != nil || e != nil {
			t.Fatal("duplicate emitted", m, e)
		}
	}
}

func TestCorruptionAndLengths(t *testing.T) {
	b := framesFor(t, fixture())[0]
	for i := range b {
		bad := bytes.Clone(b)
		bad[i] ^= 1
		if _, e := Parse(bad); e == nil {
			t.Fatalf("accepted mutation at %d", i)
		}
	}
	for _, n := range []int{0, 1, FrameBytes - 1, FrameBytes + 1} {
		if _, e := Parse(make([]byte, n)); e == nil {
			t.Fatal("invalid size accepted")
		}
	}
	for _, change := range []func([]byte){
		func(b []byte) { binary.BigEndian.PutUint16(b[18:], 0) },
		func(b []byte) { binary.BigEndian.PutUint16(b[18:], 257) },
		func(b []byte) { binary.BigEndian.PutUint16(b[16:], 256) },
		func(b []byte) { binary.BigEndian.PutUint16(b[20:], PayloadBytes+1) },
		func(b []byte) { binary.BigEndian.PutUint16(b[20:], 1) },
	} {
		bad := bytes.Clone(b)
		change(bad)
		binary.BigEndian.PutUint32(bad[FrameBytes-4:], adler32.Checksum(bad[:FrameBytes-4]))
		if _, e := Parse(bad); e == nil {
			t.Fatal("invalid dimensions accepted")
		}
	}
}

func TestLimitsAndMessageIntegrity(t *testing.T) {
	m := Message{Text: strings.Repeat("x", MaxPages*PayloadBytes-2)}
	if len(framesFor(t, m)) != MaxPages {
		t.Fatal("wrong maximum")
	}
	m.Text += "x"
	if _, e := Encode(m); e == nil {
		t.Fatal("accepted oversized message")
	}
	for _, s := range []string{"bad\x00field", "\xff"} {
		if _, e := Encode(Message{Text: s}); e == nil {
			t.Fatal("accepted invalid text")
		}
	}
	b := framesFor(t, Message{Text: "hello"})[0]
	b[26] ^= 1
	binary.BigEndian.PutUint32(b[FrameBytes-4:], adler32.Checksum(b[:FrameBytes-4]))
	var a Assembler
	if _, e := a.Add(packetFor(t, b), time.Now()); e == nil {
		t.Fatal("accepted incorrect whole-message checksum")
	}
}

func TestNewerMessageAndSession(t *testing.T) {
	var a Assembler
	old := fixture()
	old.Sequence = 0xffffffff
	pages := framesFor(t, old)
	a.Add(packetFor(t, pages[0]), time.Now())
	next := old
	next.Sequence = 0
	next.Text = "new"
	p := packetFor(t, framesFor(t, next)[0])
	if m, e := a.Add(p, time.Now()); e != nil || m == nil || *m != next {
		t.Fatal(m, e)
	}
	if m, e := a.Add(packetFor(t, pages[1]), time.Now()); m != nil || e != nil {
		t.Fatal("old sequence returned")
	}
	p.Session++
	if m, e := a.Add(p, time.Now()); m == nil || e != nil {
		t.Fatal("session restart failed", e)
	}
	p.Session--
	if m, e := a.Add(p, time.Now()); m != nil || e != nil {
		t.Fatal("retired session returned")
	}
}

func TestFindMovedAndScaledTiles(t *testing.T) {
	b := framesFor(t, fixture())[0]
	for cell := 2; cell <= 8; cell++ {
		tile, e := Render(b, cell)
		if e != nil {
			t.Fatal(e)
		}
		for _, origin := range []image.Point{{0, 0}, {37, 23}, {-144, -69}} {
			bounds := image.Rect(-200, -100, 640, 540)
			desktop := image.NewRGBA(bounds)
			draw.Draw(desktop, bounds, &image.Uniform{color.RGBA{44, 50, 60, 255}}, image.Point{}, draw.Src)
			r := tile.Bounds().Add(origin)
			draw.Draw(desktop, r, tile, image.Point{}, draw.Src)
			l, p, e := Find(desktop)
			if e != nil || l.Cell != cell || !reflect.DeepEqual(p, packetFor(t, b)) {
				t.Fatalf("cell %d, origin %v: %v %v", cell, origin, l, e)
			}
			// Candidate origins may differ by a pixel while sampling identical cells.
			if !l.Rect().In(bounds) {
				t.Fatal("out of bounds")
			}
		}
	}
}

func TestImageNoiseAndOcclusion(t *testing.T) {
	b := framesFor(t, fixture())[0]
	im, _ := Render(b, 3)
	for i := 0; i < len(im.Pix); i += 4 {
		for j := 0; j < 3; j++ {
			im.Pix[i+j] = byte(40 + int(im.Pix[i+j])*175/255)
		}
	}
	if _, e := Decode(im, Location{Cell: 3}); e != nil {
		t.Fatal(e)
	}
	im.SetRGBA(4, 4, color.RGBA{128, 128, 128, 255})
	if _, e := Decode(im, Location{Cell: 3}); e == nil {
		t.Fatal("ambiguous cell accepted")
	}
	if _, _, e := Find(im); e == nil {
		t.Fatal("corrupted tile found")
	}
	blank := image.NewRGBA(image.Rect(0, 0, 200, 200))
	if _, _, e := Find(blank); e == nil {
		t.Fatal("false positive")
	}
}

func TestLuaCompatibility(t *testing.T) {
	lua := os.Getenv("LUA")
	if lua == "" {
		var e error
		lua, e = exec.LookPath("lua")
		if e != nil {
			t.Skip("set LUA to a Lua 5.1+ interpreter to run cross-language checks")
		}
	}
	root := filepath.Join("..", "..")
	cmd := exec.Command(lua, "tests/codec.lua")
	cmd.Dir = root
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("Lua: %v\n%s", e, out)
	}
	lines := strings.Fields(string(out))
	m := fixture()
	m.Race, m.Gender, m.NPCID = "Orc", "male", "4949"
	want := framesFor(t, m)
	if len(lines) != len(want)*2+1 {
		t.Fatalf("unexpected Lua output: %s", out)
	}
	rgb, err := hex.DecodeString(lines[0])
	if err != nil || len(rgb) != 48 {
		t.Fatal("invalid Lua palette")
	}
	for i, c := range colors {
		if rgb[i*3] != c.R || rgb[i*3+1] != c.G || rgb[i*3+2] != c.B {
			t.Fatal("Lua/Go palette mismatch", i)
		}
	}
	lines = lines[1:]
	for i, b := range want {
		got, e := hex.DecodeString(lines[i*2])
		if e != nil || !bytes.Equal(got, b) {
			t.Fatalf("Lua frame %d differs", i)
		}
		cells, e := hex.DecodeString(lines[i*2+1])
		if e != nil || len(cells) != Grid*Grid {
			t.Fatal("invalid cells")
		}
		im := image.NewRGBA(image.Rect(0, 0, Grid*2, Grid*2))
		for j, v := range cells {
			draw.Draw(im, image.Rect(j%Grid*2, j/Grid*2, j%Grid*2+2, j/Grid*2+2), &image.Uniform{palette(v)}, image.Point{}, draw.Src)
		}
		l, p, e := Find(im)
		if e != nil || l.Cell != 2 || !reflect.DeepEqual(p, packetFor(t, b)) {
			t.Fatal("Lua cells do not decode", e)
		}
	}
	cmd = exec.Command(lua, "tests/speakers.lua")
	cmd.Dir = root
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("speaker lookup test: %v\n%s", e, out)
	}
	cmd = exec.Command(lua, "tests/addon.lua")
	cmd.Dir = root
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("addon smoke test: %v\n%s", e, out)
	}
}

func FuzzParse(f *testing.F) {
	b, _ := Encode(Message{Text: "hello"})
	f.Add(b[0])
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		p, e := Parse(b)
		if e == nil {
			var a Assembler
			a.Add(p, time.Now())
		}
	})
}

func BenchmarkFind1080p(b *testing.B) {
	frames, _ := Encode(Message{Text: "hello"})
	tile, _ := Render(frames[0], 3)
	im := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	draw.Draw(im, im.Bounds(), &image.Uniform{color.RGBA{30, 50, 70, 255}}, image.Point{}, draw.Src)
	draw.Draw(im, tile.Bounds().Add(image.Pt(1700, 900)), tile, image.Point{}, draw.Src)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, e := Find(im); e != nil {
			b.Fatal(e)
		}
	}
}

func TestSpeakerMetadataRoundTrip(t *testing.T) {
	for _, metadata := range []bool{false, true} {
		want := fixture()
		if metadata {
			want.Race = "Orc"
			want.Gender = "male"
			want.NPCID = "4949"
		}
		var a Assembler
		frames := framesFor(t, want)
		for i := len(frames) - 1; i >= 0; i-- {
			p := packetFor(t, frames[i])
			if (p.Flags == 1) != metadata {
				t.Fatal("incorrect metadata flag")
			}
			got, err := a.Add(p, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 && (got == nil || *got != want) {
				t.Fatalf("metadata lost: %#v", got)
			}
		}
	}
}

func TestMixedMetadataPagesRejected(t *testing.T) {
	frames := framesFor(t, fixture())
	var a Assembler
	a.Add(packetFor(t, frames[0]), time.Now())
	p := packetFor(t, frames[1])
	p.Flags = 1
	if _, err := a.Add(p, time.Now()); err == nil {
		t.Fatal("accepted mixed field layouts")
	}
}
