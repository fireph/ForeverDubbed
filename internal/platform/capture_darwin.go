//go:build darwin && cgo

package platform

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c -fobjc-arc
#cgo LDFLAGS: -mmacosx-version-min=14.0 -framework Foundation -framework ScreenCaptureKit -framework CoreGraphics -framework AudioToolbox
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"image"
	"strings"
	"unsafe"
)

var macScreen windowCapture

func Init(app string) error {
	app = strings.TrimSpace(app)
	if app == "" {
		return fmt.Errorf("-capture-app must name the game app bundle, absolute path, or bundle identifier")
	}
	var message [1024]C.char
	if C.fdb_screen_init(&message[0], C.size_t(len(message))) != 0 {
		return fmt.Errorf("macOS capture: %s", C.GoString(&message[0]))
	}
	macScreen.init(app, macWindowDriver{})
	// A missing game window is retried by Bounds/Capture, not a startup failure.
	return nil
}

// Desktop is the platform capture surface: on macOS this is only the selected
// game's window, with coordinates relative to its top-left native pixel.
func Desktop() image.Rectangle                          { return macScreen.Bounds() }
func Capture(rect image.Rectangle) (*image.RGBA, error) { return macScreen.Capture(rect) }

type macWindowDriver struct{}

func (macWindowDriver) Windows() ([]captureWindow, error) {
	var message [1024]C.char
	var data *C.char
	if C.fdb_window_list(&data, &message[0], C.size_t(len(message))) != 0 {
		return nil, fmt.Errorf("macOS window discovery: %s", C.GoString(&message[0]))
	}
	defer C.free(unsafe.Pointer(data))
	var windows []captureWindow
	if err := json.Unmarshal([]byte(C.GoString(data)), &windows); err != nil {
		return nil, fmt.Errorf("macOS window metadata: %w", err)
	}
	return windows, nil
}
func (macWindowDriver) Select(window captureWindow) (image.Rectangle, error) {
	var message [1024]C.char
	var w, h C.int
	if C.fdb_window_select(C.uint32_t(window.ID), C.int32_t(window.PID), &w, &h, &message[0], C.size_t(len(message))) != 0 {
		return image.Rectangle{}, fmt.Errorf("macOS window selection: %s", C.GoString(&message[0]))
	}
	return image.Rect(0, 0, int(w), int(h)), nil
}
func (macWindowDriver) Capture(rect image.Rectangle) (*image.RGBA, error) {
	pixels := image.NewRGBA(rect)
	var message [1024]C.char
	if C.fdb_window_capture(C.int(rect.Min.X), C.int(rect.Min.Y), C.int(rect.Dx()), C.int(rect.Dy()), unsafe.Pointer(&pixels.Pix[0]), &message[0], C.size_t(len(message))) != 0 {
		return nil, fmt.Errorf("macOS window capture: %s", C.GoString(&message[0]))
	}
	return pixels, nil
}

// CloseCapture releases platform capture resources at shutdown.
func CloseCapture() { macScreen.Close() }
