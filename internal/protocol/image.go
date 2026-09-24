package protocol

import (
	"errors"
	"fmt"
	"image"
	"image/color"
)

// Chromaglyph OKLab pack, h 264 / L 0.25 / C 0.05. Indices are protocol data.
const Outline = 2 // physical pixels at every cell size
var finderColor = color.RGBA{128, 192, 240, 255}
var colors = [16]color.RGBA{
	{21, 33, 58, 255},
	{0, 36, 61, 255},
	{36, 29, 53, 255},
	{13, 31, 71, 255},
	{23, 35, 46, 255},
	{10, 37, 50, 255},
	{24, 29, 69, 255},
	{11, 34, 61, 255},
	{33, 28, 62, 255},
	{5, 33, 69, 255},
	{28, 32, 54, 255},
	{3, 37, 54, 255},
	{30, 33, 46, 255},
	{18, 32, 64, 255},
	{22, 34, 52, 255},
	{27, 31, 60, 255},
}

const waveSymbol byte = 16

func palette(v byte) color.RGBA {
	if v == waveSymbol {
		return finderColor
	}
	return colors[v]
}

const WavePhases = DataGrid // one integer-column shift per frame

// One hard-coded loop, identical to Lua. Inclusive zero-based data rows per
// column approximate a two-cell perpendicular stroke. Only translate it;
// never re-rasterize the curve between animation frames.
var waveColumns = [DataGrid][2]int{
	{22, 25}, {24, 26}, {25, 28}, {27, 29}, {28, 30}, {29, 31}, {30, 32}, {31, 33},
	{32, 34}, {33, 34}, {34, 35}, {34, 35}, {34, 35}, {34, 35}, {34, 35}, {33, 34},
	{32, 34}, {31, 33}, {30, 32}, {29, 31}, {28, 30}, {27, 29}, {25, 28}, {24, 26},
	{22, 25}, {21, 23}, {19, 22}, {18, 20}, {17, 19}, {16, 18}, {15, 17}, {14, 16},
	{13, 15}, {13, 14}, {12, 13}, {12, 13}, {12, 13}, {12, 13}, {12, 13}, {13, 14},
	{13, 15}, {14, 16}, {15, 17}, {16, 18}, {17, 19}, {18, 20}, {19, 22}, {21, 23},
}

func waveRange(x, phase int) (int, int) {
	rows := waveColumns[((x+phase)%DataGrid+DataGrid)%DataGrid]
	return rows[0], rows[1]
}
func isWave(x, y, phase int) bool {
	if x < 1 || x > DataGrid || y < 1 || y > DataGrid {
		return false
	}
	lo, hi := waveRange(x-1, phase)
	return y-1 >= lo && y-1 <= hi
}

// The dark calibration ring contains a known pattern and sixteen bottom-row
// reference swatches. The separate light-blue outline locates the tile.
func Border(x, y int) byte {
	if y == 0 {
		return byte((x*5 + 1) % 8)
	}
	if y == Grid-1 {
		if x >= 1 && x <= 16 {
			return byte(x - 1)
		}
		return byte((x*3 + 6) % 8)
	}
	if x == 0 {
		return byte((y*3 + 2) % 8)
	}
	return byte((y*5 + 4) % 8)
}

func Render(frame []byte, cell int) (*image.RGBA, error) { return RenderWave(frame, cell, 0) }

// RenderWave produces a complete frame with data packed around the wave mask.
func RenderWave(frame []byte, cell, phase int) (*image.RGBA, error) {
	if len(frame) != FrameBytes || string(frame[:4]) != "FDB5" || cell < 2 || cell > 8 {
		return nil, errors.New("invalid frame or cell size (2–8)")
	}
	im := image.NewRGBA((Location{Cell: cell}).Rect())
	for y := 0; y < im.Bounds().Dy(); y++ {
		for x := 0; x < im.Bounds().Dx(); x++ {
			im.SetRGBA(x, y, finderColor)
		}
	}
	nibble := 0
	for y := 0; y < Grid; y++ {
		for x := 0; x < Grid; x++ {
			v := Border(x, y)
			if x > 0 && x < Grid-1 && y > 0 && y < Grid-1 {
				if isWave(x, y, phase) {
					v = waveSymbol
				} else {
					v = (frame[nibble/2] >> uint(4*(1-nibble%2))) & 15
					nibble++
				}
			}
			for py := y * cell; py < (y+1)*cell; py++ {
				for px := x * cell; px < (x+1)*cell; px++ {
					im.SetRGBA(Outline+px, Outline+py, palette(v))
				}
			}
		}
	}
	return im, nil
}

func pixel(im image.Image, x, y int) [3]int {
	if img, ok := im.(*image.RGBA); ok {
		i := img.PixOffset(x, y)
		return [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
	}
	r, g, b, _ := im.At(x, y).RGBA()
	return [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
}

// Broad light-blue gate for discovery only. Captured references, ring pattern,
// frame magic, and checksums must still validate before accepting a tile.
func isFinder(rgb [3]int) bool {
	return rgb[0] >= 25 && rgb[1] >= 70 && rgb[2] >= 100 && rgb[1]-rgb[0] >= 8 && rgb[2]-rgb[1] >= 8
}

type Location struct{ X, Y, Cell int }

func (l Location) Rect() image.Rectangle {
	return image.Rect(l.X, l.Y, l.X+Grid*l.Cell+2*Outline, l.Y+Grid*l.Cell+2*Outline)
}

type calibration struct {
	refs       [16][3]int
	separation [16]int // squared distance to each reference's nearest neighbor
}

func distance(a, b [3]int) int {
	d := 0
	for i := 0; i < 3; i++ {
		delta := a[i] - b[i]
		d += delta * delta
	}
	return d
}

func calibrate(im image.Image, l Location) (calibration, error) {
	var c calibration
	y := l.Y + Outline + (Grid-1)*l.Cell + l.Cell/2
	for i := range c.refs {
		x := l.X + Outline + (i+1)*l.Cell + l.Cell/2
		c.refs[i] = pixel(im, x, y)
	}
	for i := range c.refs {
		c.separation[i] = 3 * 255 * 255
		for j := range c.refs {
			if i != j {
				c.separation[i] = min(c.separation[i], distance(c.refs[i], c.refs[j]))
			}
		}
		if c.separation[i] < 3*3 {
			return c, fmt.Errorf("palette reference %X (%v) is too close to another color", i, c.refs[i])
		}
	}
	return c, nil
}

func (c *calibration) decode(rgb [3]int) (byte, bool) {
	best, d := 0, 3*255*255+1
	for i, ref := range c.refs {
		if candidate := distance(rgb, ref); candidate < d {
			best, d = i, candidate
		}
	}
	// Stay within 40% of the distance to the nearest competing reference.
	// Ambiguous colors are rejected instead of guessing at a bit value.
	return byte(best), d*100 <= c.separation[best]*16
}

func Decode(im image.Image, l Location) (Packet, error) {
	if l.Cell < 2 || l.Cell > 8 || !l.Rect().In(im.Bounds()) {
		return Packet{}, errors.New("tile outside image")
	}
	if !validOutline(im, l) {
		return Packet{}, errors.New("light-blue outline mismatch")
	}
	c, err := calibrate(im, l)
	if err != nil {
		return Packet{}, err
	}
	b := make([]byte, FrameBytes)
	var wave [DataGrid][DataGrid]bool
	waveColor := pixel(im, l.X, l.Y)
	nibble := 0
	for y := 0; y < Grid; y++ {
		for x := 0; x < Grid; x++ {
			px, py := l.X+Outline+x*l.Cell+l.Cell/2, l.Y+Outline+y*l.Cell+l.Cell/2
			if x == 0 || y == 0 || x == Grid-1 || y == Grid-1 {
				if y == Grid-1 && x >= 1 && x <= 16 {
					continue
				}
				v, ok := c.decode(pixel(im, px, py))
				if !ok || v != Border(x, y) {
					return Packet{}, fmt.Errorf("border mismatch at cell (%d,%d): RGB %v", x, y, pixel(im, px, py))
				}
				continue
			}
			if distance(pixel(im, px, py), waveColor) <= 12*12 {
				wave[x-1][y-1] = true
				continue
			}
			v, ok := c.decode(pixel(im, px, py))
			if !ok {
				return Packet{}, fmt.Errorf("ambiguous color at cell (%d,%d): RGB %v", x, y, pixel(im, px, py))
			}
			if nibble >= FrameBytes*2 {
				return Packet{}, errors.New("missing wave cells")
			}
			b[nibble/2] |= v << uint(4*(1-nibble%2))
			nibble++
		}
	}
	if nibble != FrameBytes*2 || !validWave(wave) {
		return Packet{}, errors.New("invalid wave mask")
	}
	p, err := Parse(b)
	if err != nil {
		return p, fmt.Errorf("decoded header %q: %w", b[:4], err)
	}
	return p, nil
}

// Infer the mask from the captured wave, not an animation clock. A complete
// known phase is required, rejecting extra/missing blue cells and torn frames.
func validWave(mask [DataGrid][DataGrid]bool) bool {
	var captured [DataGrid]int
	for x := 0; x < DataGrid; x++ {
		lo, hi, count := -1, -1, 0
		for y := 0; y < DataGrid; y++ {
			if mask[x][y] {
				if lo < 0 {
					lo = y
				}
				hi = y
				count++
			}
		}
		if lo < 12 || hi > 35 || count < 2 || count > 4 || hi-lo+1 != count {
			return false
		}
		captured[x] = lo*64 + hi
	}
	for phase := 0; phase < WavePhases; phase++ {
		matches := true
		for x := 0; x < DataGrid; x++ {
			lo, hi := waveRange(x, phase)
			if captured[x] != lo*64+hi {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

// Validate both pixels of all four sides against the captured outline color.
func validOutline(im image.Image, l Location) bool {
	r := l.Rect()
	if !r.In(im.Bounds()) {
		return false
	}
	ref := pixel(im, l.X, l.Y)
	if !isFinder(ref) {
		return false
	}
	match := func(x, y int) bool { return distance(pixel(im, x, y), ref) <= 12*12 }
	for offset := 0; offset < Outline; offset++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if !match(x, r.Min.Y+offset) || !match(x, r.Max.Y-1-offset) {
				return false
			}
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			if !match(r.Min.X+offset, y) || !match(r.Max.X-1-offset, y) {
				return false
			}
		}
	}
	return true
}

// Find locates the blue outline's top-left inner corner, then validates the
// complete outline, calibrated ring, and checksummed data at each cell size.
func Find(im image.Image) (Location, Packet, error) {
	b := im.Bounds()
	var candidateErr error
	var candidate Location
	minimum := Grid*2 + 2*Outline
	for y := b.Min.Y; y <= b.Max.Y-minimum; y++ {
		for x := b.Min.X; x <= b.Max.X-minimum; x++ {
			if !isFinder(pixel(im, x, y)) || isFinder(pixel(im, x+Outline, y+Outline)) ||
				!isFinder(pixel(im, x+Outline-1, y+Outline)) || !isFinder(pixel(im, x+Outline, y+Outline-1)) {
				continue
			}
			for cell := 2; cell <= 8; cell++ {
				l := Location{x, y, cell}
				if !validOutline(im, l) {
					continue
				}
				if p, err := Decode(im, l); err == nil {
					return l, p, nil
				} else if candidateErr == nil {
					candidate, candidateErr = l, err
				}
			}
		}
	}
	if candidateErr != nil {
		return Location{}, Packet{}, fmt.Errorf("finder detected near (%d,%d), %dpx cells, but %w", candidate.X, candidate.Y, candidate.Cell, candidateErr)
	}
	return Location{}, Packet{}, errors.New("no tile finder detected (check blue outline visibility, addon version, and pixel scaling)")
}
