//go:build !windows && !darwin

package platform

import (
	"context"
	"errors"
	"image"
)

var errWindows = errors.New("live capture and speech require Windows or macOS; use -image for offline decoding")

func Init(_ string) error                              { return errWindows }
func Desktop() image.Rectangle                         { return image.Rectangle{} }
func Capture(image.Rectangle) (*image.RGBA, error)     { return nil, errWindows }
func Speak(context.Context, string, string, int) error { return errWindows }
func Voices(context.Context) (string, error)           { return "", errWindows }

func PlayPCM(context.Context, int, <-chan []byte) error { return errWindows }
