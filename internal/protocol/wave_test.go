package protocol

import (
	"image"
	"image/draw"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestWaveEveryPhaseAndCellSize(t *testing.T) {
	frame := colorFixture(t)
	for phase := 0; phase < WavePhases; phase++ {
		waveCount, lo, hi := 0, DataGrid, 0
		for x := 1; x <= DataGrid; x++ {
			perColumn := 0
			for y := 1; y <= DataGrid; y++ {
				if isWave(x, y, phase) {
					perColumn++
					waveCount++
					lo = min(lo, y-1)
					hi = max(hi, y-1)
				}
			}
			lo1, hi1 := waveRange(x-1, phase+1)
			lo2, hi2 := waveRange(x, phase)
			if perColumn < 2 || perColumn > 4 || lo1 != lo2 || hi1 != hi2 {
				t.Fatal("wave thickness or direction", phase, x)
			}
		}
		if waveCount != WaveCells || lo != 12 || hi != 35 {
			t.Fatalf("wave envelope %d: %d %d..%d", phase, waveCount, lo, hi)
		}
		for cell := 2; cell <= 8; cell++ {
			im, err := RenderWave(frame, cell, phase)
			if err != nil {
				t.Fatal(err)
			}
			l, p, err := Find(im)
			if err != nil || l.Cell != cell || !reflect.DeepEqual(p, packetFor(t, frame)) {
				t.Fatalf("phase %d cell %d: %v", phase, cell, err)
			}
			if _, err := Decode(im.SubImage(l.Rect()), l); err != nil {
				t.Fatal("tracking", err)
			}
			// Every pixel of a wave cell is solid blue, and no ring cell is blue.
			for y := 0; y < Grid; y++ {
				for x := 0; x < Grid; x++ {
					if isWave(x, y, phase) {
						for dy := 0; dy < cell; dy++ {
							for dx := 0; dx < cell; dx++ {
								if im.RGBAAt(Outline+x*cell+dx, Outline+y*cell+dy) != finderColor {
									t.Fatal("aliased wave")
								}
							}
						}
					} else if x == 0 || y == 0 || x == Grid-1 || y == Grid-1 {
						if im.RGBAAt(Outline+x*cell, Outline+y*cell) == finderColor {
							t.Fatal("wave overlaps calibration ring")
						}
					}
				}
			}
		}
	}
}

func TestWaveDamageAndTornFrames(t *testing.T) {
	frame := colorFixture(t)
	for _, damage := range []func(*image.RGBA){
		func(im *image.RGBA) { im.SetRGBA(Outline+3, Outline+3, finderColor) },                                  // extra blue data cell
		func(im *image.RGBA) { lo, _ := waveRange(0, 0); im.SetRGBA(Outline+3, Outline+(lo+1)*2+1, colors[0]) }, // missing wave cell
		func(im *image.RGBA) { // Partial redraw from a different phase.
			next, _ := RenderWave(frame, 2, 12)
			cut := im.Bounds()
			cut.Min.X = cut.Dx() / 2
			draw.Draw(im, cut, next, cut.Min, draw.Src)
		},
	} {
		im, _ := RenderWave(frame, 2, 0)
		damage(im)
		if _, err := Decode(im, Location{Cell: 2}); err == nil {
			t.Fatal("damaged wave accepted")
		}
	}
}

func TestWaveMultipageWithIndependentPhases(t *testing.T) {
	want := fixture()
	frames := framesFor(t, want)
	var assembler Assembler
	for i := len(frames) - 1; i >= 0; i-- {
		im, _ := RenderWave(frames[i], 3, (i*17+31)%WavePhases)
		_, p, err := Find(im)
		if err != nil {
			t.Fatal(err)
		}
		m, err := assembler.Add(p, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && (m == nil || *m != want) {
			t.Fatal("message lost across wave phases", m)
		}
	}
}

// Independently compare the integer masks with distance to the continuous
// curve. This catches the old fixed-vertical-width stroke on steep slopes.
func TestWavePerpendicularWidth(t *testing.T) {
	for i, rows := range waveColumns {
		theta := 2 * math.Pi * float64(i) / DataGrid
		for row := 0; row < DataGrid; row++ {
			py := float64(row) + 0.5
			d2 := func(x float64) float64 { dy := py - (24 + 11*math.Sin(theta+2*math.Pi*x/48)); return x*x + dy*dy }
			expected := false
			if math.Abs(py-(24+11*math.Sin(theta))) < 3 {
				a, b := -1.0, 1.0
				for n := 0; n < 60; n++ {
					left, right := (2*a+b)/3, (a+2*b)/3
					if d2(left) < d2(right) {
						b = right
					} else {
						a = left
					}
				}
				expected = d2((a+b)/2) <= 1
			}
			actual := row >= rows[0] && row <= rows[1]
			if actual != expected {
				t.Fatalf("stroke width differs at angular sample %d row %d", i, row)
			}
		}
	}
}

func TestWaveRigidTranslation(t *testing.T) {
	frame := colorFixture(t)
	base, _ := RenderWave(frame, 2, 0)
	for phase := 0; phase < WavePhases; phase++ {
		im, _ := RenderWave(frame, 2, phase)
		count := 0
		for y := 1; y <= DataGrid; y++ {
			for x := 1; x <= DataGrid; x++ {
				blue := im.RGBAAt(Outline+x*2, Outline+y*2) == finderColor
				originalX := (x-1+phase)%DataGrid + 1
				originalBlue := base.RGBAAt(Outline+originalX*2, Outline+y*2) == finderColor
				if blue != originalBlue {
					t.Fatal("wave shape changed while translating", phase, x, y)
				}
				if blue {
					count++
				}
			}
		}
		if count != 136 {
			t.Fatal("wave cell count changed", phase, count)
		}
	}
	if FrameBytes != 1084 || PayloadBytes != 1056 {
		t.Fatal("incorrect fixed capacity")
	}
}
