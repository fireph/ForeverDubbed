//go:build windows && cgo

package platform

/*
#cgo CXXFLAGS: -std=c++17 -O2
#cgo LDFLAGS: -static -lstdc++ -ld3d11 -ldxgi -ldxguid -ldwmapi -lruntimeobject -lole32
#include "native_windows.h"
*/
import "C"

import (
	"fmt"
	"image"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var windowsScreen = windowCapture{driver: &winWindowDriver{}}

type winCaptureRequest struct {
	run    func(*C.fdb_wgc) error
	result chan error
}
type winWindowDriver struct {
	requests  chan winCaptureRequest
	done      chan struct{}
	stop      chan struct{}
	closeOnce sync.Once
}

func Init(app string) error {
	app = strings.TrimSpace(app)
	if app == "" {
		return fmt.Errorf("-capture-app must name the game executable")
	}
	// Keep HWND geometry and WGC's output in physical pixels at every DPI.
	user32 := syscall.NewLazyDLL("user32.dll")
	dpi := user32.NewProc("SetProcessDpiAwarenessContext")
	aware := uintptr(0)
	if dpi.Find() == nil {
		aware, _, _ = dpi.Call(^uintptr(3))
	}
	if aware == 0 {
		user32.NewProc("SetProcessDPIAware").Call()
	}
	driver := &winWindowDriver{requests: make(chan winCaptureRequest), done: make(chan struct{}), stop: make(chan struct{})}
	started := make(chan error, 1)
	go func() {
		// D3D's immediate context and all WinRT objects belong to this MTA thread.
		// CreateFreeThreaded supplies WGC's internal delivery thread, so no GUI
		// dispatcher or Go callbacks from COM are needed.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(driver.done)
		var message [1024]C.char
		native := C.fdb_wgc_open(&message[0], C.size_t(len(message)))
		if native == nil {
			started <- fmt.Errorf("Windows capture: %s", C.GoString(&message[0]))
			return
		}
		defer C.fdb_wgc_close(native)
		started <- nil
		for {
			select {
			case <-driver.stop:
				return
			case request := <-driver.requests:
				request.result <- request.run(native)
			}
		}
	}()
	if err := <-started; err != nil {
		return err
	}
	windowsScreen.driver = driver
	windowsScreen.app = app
	return nil
}

func (d *winWindowDriver) call(run func(*C.fdb_wgc) error) error {
	if d.requests == nil {
		return fmt.Errorf("Windows capture is not initialized")
	}
	result := make(chan error, 1)
	select {
	case <-d.done:
		return fmt.Errorf("Windows capture is closed")
	case <-d.stop:
		return fmt.Errorf("Windows capture is closed")
	case d.requests <- winCaptureRequest{run, result}:
	}
	return <-result
}
func CloseCapture() {
	windowsScreen.Lock()
	defer windowsScreen.Unlock()
	d := windowsScreen.driver.(*winWindowDriver)
	if d.requests != nil {
		d.closeOnce.Do(func() { close(d.stop); <-d.done })
	}
}
func Desktop() image.Rectangle                          { return windowsScreen.Bounds() }
func Capture(rect image.Rectangle) (*image.RGBA, error) { return windowsScreen.Capture(rect) }

func (d *winWindowDriver) Windows() ([]captureWindow, error) {
	var message [1024]C.char
	head := C.fdb_win_windows(&message[0], C.size_t(len(message)))
	defer C.fdb_win_free_windows(head)
	if message[0] != 0 {
		return nil, fmt.Errorf("Windows window discovery: %s", C.GoString(&message[0]))
	}
	var windows []captureWindow
	for node := head; node != nil; node = node.next {
		windows = append(windows, captureWindow{ID: uint64(node.id), PID: int32(node.pid), Executable: C.GoString(node.executable), Width: float64(node.width), Height: float64(node.height), OnScreen: true})
	}
	return windows, nil
}
func (d *winWindowDriver) Select(window captureWindow) (image.Rectangle, error) {
	var bounds image.Rectangle
	err := d.call(func(native *C.fdb_wgc) error {
		var message [1024]C.char
		var w, h C.int
		if C.fdb_wgc_select(native, C.uintptr_t(window.ID), C.uint32_t(window.PID), &w, &h, &message[0], C.size_t(len(message))) != 0 {
			return fmt.Errorf("Windows window selection: %s", C.GoString(&message[0]))
		}
		bounds = image.Rect(0, 0, int(w), int(h))
		return nil
	})
	return bounds, err
}
func (d *winWindowDriver) Capture(rect image.Rectangle) (*image.RGBA, error) {
	pixels := image.NewRGBA(rect)
	err := d.call(func(native *C.fdb_wgc) error {
		var message [1024]C.char
		if C.fdb_wgc_capture(native, C.int(rect.Min.X), C.int(rect.Min.Y), C.int(rect.Dx()), C.int(rect.Dy()), unsafe.Pointer(&pixels.Pix[0]), &message[0], C.size_t(len(message))) != 0 {
			return fmt.Errorf("Windows window capture: %s", C.GoString(&message[0]))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pixels, nil
}
func (d *winWindowDriver) Reset() {
	_ = d.call(func(native *C.fdb_wgc) error { C.fdb_wgc_reset(native); return nil })
}
