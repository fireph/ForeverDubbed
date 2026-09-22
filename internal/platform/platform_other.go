//go:build !windows

package platform

import (
	"context"
	"errors"
	"image"
)

var errWindows = errors.New("live capture and speech require Windows; use -image for offline decoding")

func Init() error                                      { return errWindows }
func Desktop() image.Rectangle                         { return image.Rectangle{} }
func Capture(image.Rectangle) (*image.RGBA, error)     { return nil, errWindows }
func Speak(context.Context, string, string, int) error { return errWindows }
func Voices(context.Context) (string, error)           { return "", errWindows }

func PlayPCM(context.Context, int, <-chan []byte) error { return errWindows }
