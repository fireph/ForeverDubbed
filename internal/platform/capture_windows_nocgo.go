//go:build windows && !cgo

package platform

import (
	"errors"
	"image"
)

var errWinCGO = errors.New("Windows window capture requires a CGO_ENABLED=1 build with a Windows C/C++ compiler; use a prepared release")

func Init(string) error                            { return errWinCGO }
func CloseCapture()                                {}
func Desktop() image.Rectangle                     { return image.Rectangle{} }
func Capture(image.Rectangle) (*image.RGBA, error) { return nil, errWinCGO }
