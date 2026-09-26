//go:build !windows && !darwin

package platform

import (
	"context"
	"errors"
	"image"
)

var errUnsupportedPlatform = errors.New("live capture and speech require Windows or macOS; use -image for offline decoding")

func Init(_ string) error                              { return errUnsupportedPlatform }
func Desktop() image.Rectangle                         { return image.Rectangle{} }
func Capture(image.Rectangle) (*image.RGBA, error)     { return nil, errUnsupportedPlatform }
func Speak(context.Context, string, string, int) error { return errUnsupportedPlatform }
func Voices(context.Context) (string, error)           { return "", errUnsupportedPlatform }

func PlayPCM(context.Context, int, <-chan []byte) error { return errUnsupportedPlatform }

// CloseCapture releases platform capture resources at shutdown.
func CloseCapture() {}
