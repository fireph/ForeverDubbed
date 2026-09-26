//go:build gui

package desktop

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Caudex is distributed under the SIL Open Font License; see docs/licenses.
//
//go:embed assets/Caudex-Regular.ttf
var questRegular []byte

//go:embed assets/Caudex-Bold.ttf
var questBold []byte

//go:embed assets/Caudex-Italic.ttf
var questItalic []byte

var regularFont = fyne.NewStaticResource("Caudex-Regular.ttf", questRegular)
var boldFont = fyne.NewStaticResource("Caudex-Bold.ttf", questBold)
var italicFont = fyne.NewStaticResource("Caudex-Italic.ttf", questItalic)

var gold = color.NRGBA{R: 239, G: 196, B: 87, A: 255}
var green = color.NRGBA{R: 42, G: 91, B: 39, A: 255}
var soft = color.NRGBA{R: 110, G: 77, B: 40, A: 255}
var ink = color.NRGBA{R: 48, G: 32, B: 19, A: 255}
var iron = color.NRGBA{R: 34, G: 32, B: 28, A: 255}

type companionTheme struct{ fyne.Theme }

func (t companionTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace || s.Symbol {
		return t.Theme.Font(s)
	}
	if s.Italic {
		return italicFont
	}
	if s.Bold {
		return boldFont
	}
	return regularFont
}
func (t companionTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 17
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 20
	case theme.SizeNameButtonRadius:
		return 5
	case theme.SizeNameInputRadius:
		return 1
	case theme.SizeNameScrollBar:
		return 9
	}
	return t.Theme.Size(n)
}
func (t companionTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return iron
	case theme.ColorNameForeground:
		return ink
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return color.NRGBA{R: 116, G: 49, B: 24, A: 255}
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return soft
	case theme.ColorNameInputBackground, theme.ColorNameButton:
		return color.NRGBA{R: 194, G: 157, B: 94, A: 255}
	case theme.ColorNameMenuBackground:
		// Dropdown popups show ink text; keep them on a parchment tone.
		return color.NRGBA{R: 220, G: 181, B: 119, A: 255}
	case theme.ColorNameHover, theme.ColorNameFocus:
		return color.NRGBA{R: 97, G: 57, B: 20, A: 40}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 69, G: 31, B: 9, A: 75}
	case theme.ColorNameSeparator, theme.ColorNameInputBorder:
		return color.NRGBA{R: 115, G: 78, B: 38, A: 150}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 100, G: 72, B: 40, A: 220}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 70, G: 45, B: 20, A: 30}
	case theme.ColorNameError:
		return color.NRGBA{R: 144, G: 32, B: 24, A: 255}
	}
	return t.Theme.Color(n, theme.VariantLight)
}

type chromeTheme struct{ companionTheme }

func (t chromeTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameForeground, theme.ColorNameForegroundOnPrimary:
		return gold
	case theme.ColorNameButton:
		return color.Transparent
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 52, G: 43, B: 37, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 153, G: 136, B: 103, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 112, G: 15, B: 14, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 248, G: 157, B: 70, A: 42}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 95}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 255, G: 207, B: 100, A: 110}
	}
	return t.companionTheme.Color(n, v)
}

func inset(n float32, object fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedLayout(n, n, n, n), object)
}

// Layered metal edges are drawn at the current size, so they stay sharp on Retina
// displays and when the user resizes the window. No game textures are required.
func questFrame(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(
		canvas.NewRectangle(color.NRGBA{R: 12, G: 11, B: 9, A: 255}),
		inset(1, canvas.NewVerticalGradient(color.NRGBA{R: 158, G: 145, B: 115, A: 255}, color.NRGBA{R: 57, G: 53, B: 46, A: 255})),
		inset(3, canvas.NewRectangle(color.NRGBA{R: 20, G: 18, B: 15, A: 255})),
		inset(5, canvas.NewVerticalGradient(color.NRGBA{R: 99, G: 89, B: 65, A: 255}, color.NRGBA{R: 40, G: 36, B: 27, A: 255})),
		inset(7, chromeTexture()),
		inset(11, content),
	)
}

// A deterministic paper texture: broad discoloration, fine grain, and uneven
// darkened edges. Generated once, then stretched as a normal cached Fyne image.
var parchmentImage = sync.OnceValue(func() image.Image {
	const w, h = 640, 800
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			xf, yf := float64(x), float64(y)
			hash := uint32(x)*374761393 + uint32(y)*668265263
			hash = (hash ^ (hash >> 13)) * 1274126177
			grain := float64(hash&255)/255 - 0.5
			cloud := 4*math.Sin(xf/83+yf/139) + 3*math.Sin(xf/37-yf/71) + 2*math.Sin(xf/17+yf/29)
			edge := math.Min(math.Min(xf, float64(w-1-x)), math.Min(yf, float64(h-1-y)))
			rough := math.Sin(xf/9+yf/7) + 1.5*math.Sin(yf/13-xf/17) + grain*3
			burn := 32 * math.Exp(-math.Max(0, edge+rough)/16)
			if edge+rough < 3 {
				burn += 24
			}
			tone := cloud + grain*9 - burn
			im.SetNRGBA(x, y, color.NRGBA{R: uint8(220 + tone), G: uint8(181 + tone), B: uint8(119 + tone), A: 255})
		}
	}
	return im
})

func parchment(content fyne.CanvasObject) fyne.CanvasObject {
	paper := canvas.NewImageFromImage(parchmentImage())
	paper.FillMode = canvas.ImageFillStretch
	rim := canvas.NewRectangle(color.NRGBA{R: 67, G: 47, B: 26, A: 255})
	return container.NewStack(rim, inset(2, paper), inset(20, content))
}
func questRule() fyne.CanvasObject {
	line := canvas.NewRectangle(color.NRGBA{R: 108, G: 73, B: 31, A: 120})
	line.SetMinSize(fyne.NewSize(1, 1))
	return inset(3, line)
}
func questMedallion() fyne.CanvasObject {
	outer := canvas.NewCircle(color.NRGBA{R: 94, G: 75, B: 38, A: 255})
	outer.StrokeColor = gold
	outer.StrokeWidth = 2
	middle := canvas.NewCircle(color.NRGBA{R: 15, G: 18, B: 18, A: 255})
	middle.StrokeColor = color.NRGBA{R: 180, G: 141, B: 61, A: 255}
	middle.StrokeWidth = 1
	mark := canvas.NewText("!", gold)
	mark.TextSize = 46
	mark.TextStyle.Bold = true
	return container.NewGridWrap(fyne.NewSize(66, 66), container.NewStack(outer, inset(5, middle), container.NewCenter(mark)))
}
func questButton(label string, action func()) fyne.CanvasObject {
	return questButtonWidget(widget.NewButton(label, action))
}

func questButtonWidget(button *widget.Button) fyne.CanvasObject {
	// Each layer has its own radius so neither the frame nor the face leaves
	// square corners outside the button's rounded hover/pressed background.
	edge := roundedButtonLayer(color.NRGBA{R: 24, G: 17, B: 11, A: 255}, color.NRGBA{R: 24, G: 17, B: 11, A: 255}, 8)
	bevel := roundedButtonLayer(color.NRGBA{R: 186, G: 159, B: 102, A: 255}, color.NRGBA{R: 77, G: 56, B: 29, A: 255}, 7)
	red := roundedButtonLayer(color.NRGBA{R: 140, G: 24, B: 20, A: 255}, color.NRGBA{R: 74, G: 6, B: 7, A: 255}, 5)
	surface := container.NewStack(red, button)
	return container.NewThemeOverride(container.NewStack(edge, inset(1, bevel), inset(3, surface)), chromeTheme{companionTheme{theme.DefaultTheme()}})
}

// Mask a vertical gradient with antialiased rounded corners at canvas scale.
func roundedButtonLayer(top, bottom color.NRGBA, radius float64) *canvas.Raster {
	var raster *canvas.Raster
	raster = canvas.NewRaster(func(w, h int) image.Image {
		im := image.NewNRGBA(image.Rect(0, 0, w, h))
		scale := 1.0
		if raster.Size().Height > 0 {
			scale = float64(h) / float64(raster.Size().Height)
		}
		r := math.Min(radius*scale, float64(min(w, h))/2)
		for y := 0; y < h; y++ {
			t := float64(y) / float64(max(1, h-1))
			mix := func(a, b uint8) uint8 { return uint8(float64(a)*(1-t) + float64(b)*t) }
			for x := 0; x < w; x++ {
				dx := math.Max(r-(float64(x)+0.5), float64(x)+0.5-(float64(w)-r))
				dy := math.Max(r-(float64(y)+0.5), float64(y)+0.5-(float64(h)-r))
				alpha := math.Min(1, math.Max(0, r+0.5-math.Hypot(math.Max(0, dx), math.Max(0, dy))))
				im.SetNRGBA(x, y, color.NRGBA{R: mix(top.R, bottom.R), G: mix(top.G, bottom.G), B: mix(top.B, bottom.B), A: uint8(255 * alpha)})
			}
		}
		return im
	})
	return raster
}

var chromeImage = sync.OnceValue(func() image.Image {
	const w, h = 400, 400
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			hash := uint32(x)*374761393 + uint32(y)*668265263
			hash = (hash ^ (hash >> 13)) * 1274126177
			grain := float64(hash&255)/255 - 0.5
			tone := grain*17 + 3*math.Sin(float64(x)/11+float64(y)/19)
			im.SetNRGBA(x, y, color.NRGBA{R: uint8(35 + tone), G: uint8(34 + tone), B: uint8(29 + tone), A: 255})
		}
	}
	return im
})

func chromeTexture() fyne.CanvasObject {
	texture := canvas.NewImageFromImage(chromeImage())
	texture.FillMode = canvas.ImageFillStretch
	return texture
}
