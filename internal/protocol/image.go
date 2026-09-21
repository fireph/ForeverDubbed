package protocol

import (
	"errors"
	"fmt"
	"image"
	"image/color"
)

// Eight RGB cube corners plus eight edge midpoints. The closest pair is 127
// RGB units apart before display transforms. Indices are protocol data.
var colors = [16]color.RGBA{
	{0, 0, 0, 255}, {0, 0, 255, 255}, {0, 255, 0, 255}, {0, 255, 255, 255},
	{255, 0, 0, 255}, {255, 0, 255, 255}, {255, 255, 0, 255}, {255, 255, 255, 255},
	{128, 0, 0, 255}, {0, 128, 255, 255}, {128, 255, 0, 255}, {128, 0, 255, 255},
	{255, 128, 0, 255}, {255, 0, 128, 255}, {0, 255, 128, 255}, {128, 255, 255, 255},
}

func palette(v byte) color.RGBA { return colors[v] }

// Border retains a binary RGB finder pattern. Sixteen bottom-row swatches
// expose the actual captured palette, including the display's color transform.
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

func Render(frame []byte, cell int) (*image.RGBA, error) {
	if len(frame) != FrameBytes || string(frame[:4]) != "FDB3" || cell < 2 || cell > 8 {
		return nil, errors.New("invalid frame or cell size (2–8)")
	}
	im := image.NewRGBA(image.Rect(0, 0, Grid*cell, Grid*cell))
	nibble := 0
	for y := 0; y < Grid; y++ {
		for x := 0; x < Grid; x++ {
			v := Border(x, y)
			if x > 0 && x < Grid-1 && y > 0 && y < Grid-1 {
				v = (frame[nibble/2] >> uint(4*(1-nibble%2))) & 15
				nibble++
			}
			for py := y * cell; py < (y+1)*cell; py++ {
				for px := x * cell; px < (x+1)*cell; px++ {
					im.SetRGBA(px, py, palette(v))
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

// Only discovery and non-calibration border cells use fixed binary thresholds.
func sample(im image.Image, x, y int) (byte, bool) {
	v := byte(0)
	for _, c := range pixel(im, x, y) {
		if c > 72 && c < 183 {
			return 0, false
		}
		v <<= 1
		if c >= 183 {
			v |= 1
		}
	}
	return v, true
}

type Location struct{ X, Y, Cell int }

func (l Location) Rect() image.Rectangle {
	return image.Rect(l.X, l.Y, l.X+Grid*l.Cell, l.Y+Grid*l.Cell)
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
	y := l.Y + (Grid-1)*l.Cell + l.Cell/2
	for i := range c.refs {
		x := l.X + (i+1)*l.Cell + l.Cell/2
		c.refs[i] = pixel(im, x, y)
		if i < 8 {
			v, ok := sample(im, x, y)
			if !ok || int(v) != i {
				return c, fmt.Errorf("palette anchor %d invalid: captured RGB %v", i, c.refs[i])
			}
		}
	}
	for i := range c.refs {
		c.separation[i] = 3 * 255 * 255
		for j := range c.refs {
			if i != j {
				c.separation[i] = min(c.separation[i], distance(c.refs[i], c.refs[j]))
			}
		}
		if c.separation[i] < 32*32 {
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
	c, err := calibrate(im, l)
	if err != nil {
		return Packet{}, err
	}
	b := make([]byte, FrameBytes)
	nibble := 0
	for y := 0; y < Grid; y++ {
		for x := 0; x < Grid; x++ {
			px, py := l.X+x*l.Cell+l.Cell/2, l.Y+y*l.Cell+l.Cell/2
			if x == 0 || y == 0 || x == Grid-1 || y == Grid-1 {
				if y == Grid-1 && x >= 1 && x <= 16 {
					continue
				}
				v, ok := sample(im, px, py)
				if !ok || v != Border(x, y) {
					return Packet{}, fmt.Errorf("border mismatch at cell (%d,%d): RGB %v", x, y, pixel(im, px, py))
				}
				continue
			}
			v, ok := c.decode(pixel(im, px, py))
			if !ok {
				return Packet{}, fmt.Errorf("ambiguous color at cell (%d,%d): RGB %v", x, y, pixel(im, px, py))
			}
			b[nibble/2] |= v << uint(4*(1-nibble%2))
			nibble++
		}
	}
	p, err := Parse(b)
	if err != nil {
		return p, fmt.Errorf("decoded header %q: %w", b[:4], err)
	}
	return p, nil
}

// Find scans potential top-row centers; border, palette, and checksum must all
// validate before accepting a location. Integer physical cell sizes only.
func Find(im image.Image) (Location, Packet, error) {
	b := im.Bounds()
	var candidateErr error
	var candidate Location
	for y := b.Min.Y + 1; y < b.Max.Y-Grid*2+2; y++ {
		for x := b.Min.X + 1; x < b.Max.X-Grid*2+2; x++ {
			v, ok := sample(im, x, y)
			if !ok || v != Border(0, 0) {
				continue
			}
			for cell := 2; cell <= 8; cell++ {
				l := Location{x - cell/2, y - cell/2, cell}
				if !l.Rect().In(b) {
					continue
				}
				match := true
				for k := 1; k < Grid; k++ {
					v, ok = sample(im, x+k*cell, y)
					if !ok || v != Border(k, 0) {
						match = false
						break
					}
				}
				if match {
					if p, err := Decode(im, l); err == nil {
						return l, p, nil
					} else if candidateErr == nil {
						candidate, candidateErr = l, err
					}
				}
			}
		}
	}
	if candidateErr != nil {
		return Location{}, Packet{}, fmt.Errorf("finder detected near (%d,%d), %dpx cells, but %w", candidate.X, candidate.Y, candidate.Cell, candidateErr)
	}
	return Location{}, Packet{}, errors.New("no tile finder detected (check square visibility, addon version, and pixel scaling)")
}
