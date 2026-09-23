//go:build darwin && !cgo

package platform

import (
	"context"
	"errors"
	"image"
)

var errMacCGO = errors.New("macOS capture and PCM playback require CGO_ENABLED=1 and Xcode command-line tools; rebuild on macOS")

func Init(_ string) error                               { return errMacCGO }
func Desktop() image.Rectangle                          { return image.Rectangle{} }
func Capture(image.Rectangle) (*image.RGBA, error)      { return nil, errMacCGO }
func PlayPCM(context.Context, int, <-chan []byte) error { return errMacCGO }
