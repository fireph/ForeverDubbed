//go:build windows

package platform

import (
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32       = syscall.NewLazyDLL("user32.dll")
	gdi32        = syscall.NewLazyDLL("gdi32.dll")
	getDC        = user32.NewProc("GetDC")
	releaseDC    = user32.NewProc("ReleaseDC")
	getMetrics   = user32.NewProc("GetSystemMetrics")
	createDC     = gdi32.NewProc("CreateCompatibleDC")
	deleteDC     = gdi32.NewProc("DeleteDC")
	createDIB    = gdi32.NewProc("CreateDIBSection")
	selectObject = gdi32.NewProc("SelectObject")
	deleteObject = gdi32.NewProc("DeleteObject")
	bitBlt       = gdi32.NewProc("BitBlt")
	gdiFlush     = gdi32.NewProc("GdiFlush")
)

func Init() error {
	// Opt out of DPI virtualization so detection and capture use physical pixels.
	p := user32.NewProc("SetProcessDpiAwarenessContext")
	if p.Find() == nil {
		if ok, _, _ := p.Call(^uintptr(3)); ok != 0 {
			return nil
		}
	}
	user32.NewProc("SetProcessDPIAware").Call()
	return nil
}

func metric(n uintptr) int { v, _, _ := getMetrics.Call(n); return int(int32(v)) }

func Desktop() image.Rectangle {
	x, y := metric(76), metric(77)
	return image.Rect(x, y, x+metric(78), y+metric(79))
}

type bitmapInfo struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}

func Capture(rect image.Rectangle) (*image.RGBA, error) {
	if rect.Empty() || !rect.In(Desktop()) || int64(rect.Dx())*int64(rect.Dy()) > 100_000_000 {
		return nil, fmt.Errorf("invalid capture rectangle %v", rect)
	}
	// GetDC and ReleaseDC must execute on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dc, _, err := getDC.Call(0)
	if dc == 0 {
		return nil, fmt.Errorf("GetDC: %v", err)
	}
	defer releaseDC.Call(0, dc)
	mem, _, err := createDC.Call(dc)
	if mem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC: %v", err)
	}
	defer deleteDC.Call(mem)
	info := bitmapInfo{Size: 40, Width: int32(rect.Dx()), Height: -int32(rect.Dy()), Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bmp, _, err := createDIB.Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection: %v", err)
	}
	defer deleteObject.Call(bmp)
	old, _, err := selectObject.Call(mem, bmp)
	if old == 0 || old == ^uintptr(0) {
		return nil, fmt.Errorf("SelectObject: %v", err)
	}
	defer selectObject.Call(mem, old)
	ok, _, err := bitBlt.Call(mem, 0, 0, uintptr(rect.Dx()), uintptr(rect.Dy()), dc, uintptr(rect.Min.X), uintptr(rect.Min.Y), 0x40CC0020)
	if ok == 0 {
		return nil, fmt.Errorf("BitBlt: %v", err)
	}
	gdiFlush.Call()
	src := unsafe.Slice((*byte)(bits), rect.Dx()*rect.Dy()*4)
	im := image.NewRGBA(rect)
	for i := 0; i < len(src); i += 4 {
		im.Pix[i] = src[i+2]
		im.Pix[i+1] = src[i+1]
		im.Pix[i+2] = src[i]
		im.Pix[i+3] = 255
	}
	return im, nil
}
