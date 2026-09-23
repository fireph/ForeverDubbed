// Package appicon draws the companion's waveform icon for Fyne and app bundles.
package appicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// PNG renders the same rounded navy tile and gold waveform at any icon size.
func PNG(size int) []byte {
	im := image.NewNRGBA(image.Rect(0, 0, size, size))
	points := [][2]float64{{12, 33}, {18, 33}, {23, 19}, {31, 47}, {39, 15}, {45, 33}, {52, 33}}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := (float64(x)+0.5)*64/float64(size), (float64(y)+0.5)*64/float64(size)
			dx, dy := math.Max(math.Abs(px-32)-14, 0), math.Max(math.Abs(py-32)-14, 0)
			edge := math.Hypot(dx, dy) - 16
			alpha := math.Min(1, math.Max(0, 0.5-edge*float64(size)/64))
			if alpha == 0 {
				continue
			}
			col := color.NRGBA{R: 24, G: 34, B: 49, A: uint8(alpha * 255)}
			distance := 100.0
			for i := 1; i < len(points); i++ {
				ax, ay := points[i-1][0], points[i-1][1]
				bx, by := points[i][0], points[i][1]
				t := math.Max(0, math.Min(1, ((px-ax)*(bx-ax)+(py-ay)*(by-ay))/((bx-ax)*(bx-ax)+(by-ay)*(by-ay))))
				distance = math.Min(distance, math.Hypot(px-ax-t*(bx-ax), py-ay-t*(by-ay)))
			}
			ink := math.Min(1, math.Max(0, 0.5+(2.5-distance)*float64(size)/64))
			col.R = uint8(24 + ink*(231-24))
			col.G = uint8(34 + ink*(188-34))
			col.B = uint8(49 + ink*(112-49))
			im.SetNRGBA(x, y, col)
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, im) // bytes.Buffer writes cannot fail.
	return out.Bytes()
}
