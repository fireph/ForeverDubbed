// Package appicon shares the companion's logo with the UI and app bundles.
package appicon

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"

	"golang.org/x/image/draw"
)

// Logo is the original transparent artwork used for application and tray icons.
//
//go:embed assets/forever-dubbed-logo.png
var Logo []byte

// Banner is the wide artwork used in the desktop header.
//
//go:embed assets/forever-dubbed-banner.png
var Banner []byte

// Keep the Windows resource in source control so ordinary go builds include
// the executable icon without requiring a resource compiler.
//go:generate x86_64-w64-mingw32-windres --input assets/icon.rc --output icon_windows_amd64.syso --output-format coff --target pe-x86-64

var logoImage = sync.OnceValue(func() image.Image {
	im, err := png.Decode(bytes.NewReader(Logo))
	if err != nil {
		panic("decode embedded app logo: " + err.Error())
	}
	return im
})

// PNG scales the logo for Fyne and macOS icon sizes, preserving transparency.
func PNG(size int) []byte {
	im := image.NewNRGBA(image.Rect(0, 0, size, size))
	source := logoImage()
	draw.CatmullRom.Scale(im, im.Bounds(), source, source.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	_ = png.Encode(&out, im) // bytes.Buffer writes cannot fail.
	return out.Bytes()
}
