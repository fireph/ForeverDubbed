//go:build windows

package platform

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	waveDLL       = syscall.NewLazyDLL("winmm.dll")
	waveOpen      = waveDLL.NewProc("waveOutOpen")
	wavePrepare   = waveDLL.NewProc("waveOutPrepareHeader")
	waveWrite     = waveDLL.NewProc("waveOutWrite")
	waveUnprepare = waveDLL.NewProc("waveOutUnprepareHeader")
	waveReset     = waveDLL.NewProc("waveOutReset")
	waveClose     = waveDLL.NewProc("waveOutClose")
)

type waveFormat struct {
	Tag, Channels        uint16
	Rate, BytesPerSecond uint32
	Align, Bits, Extra   uint16
}

type waveHeader struct {
	Data             *byte
	Length, Recorded uint32
	User             uintptr
	Flags, Loops     uint32
	Next             *waveHeader
	Reserved         uintptr
}

type waveBuffer struct {
	header waveHeader
	pin    runtime.Pinner
}

type waveDevice struct {
	handle  uintptr
	buffers []*waveBuffer
}

// If a broken driver refuses to return buffers, retain their pins for process
// lifetime rather than free memory that Windows may still access.
var failedWaveDevices struct {
	sync.Mutex
	devices []*waveDevice
}

func waveResult(op string, code uintptr) error {
	if code != 0 {
		return fmt.Errorf("%s failed (MMRESULT %d)", op, code)
	}
	return nil
}

func openAudio(rate int) (pcmDevice, error) {
	format := waveFormat{Tag: 1, Channels: 1, Rate: uint32(rate), BytesPerSecond: uint32(rate) * 2, Align: 2, Bits: 16}
	device := &waveDevice{}
	code, _, _ := waveOpen.Call(uintptr(unsafe.Pointer(&device.handle)), 0xffffffff, uintptr(unsafe.Pointer(&format)), 0, 0, 0)
	if err := waveResult("waveOutOpen", code); err != nil {
		return nil, err
	}
	return device, nil
}

func (d *waveDevice) Queue(pcm []byte) error {
	buffer := &waveBuffer{}
	buffer.header.Data = &pcm[0]
	buffer.header.Length = uint32(len(pcm))
	// WinMM holds the data and WAVEHDR after the syscall returns. Pin both
	// until the driver has returned and unprepared the buffer.
	buffer.pin.Pin(&pcm[0])
	buffer.pin.Pin(&buffer.header)
	ptr := uintptr(unsafe.Pointer(&buffer.header))
	size := unsafe.Sizeof(buffer.header)
	code, _, _ := wavePrepare.Call(d.handle, ptr, size)
	if err := waveResult("waveOutPrepareHeader", code); err != nil {
		buffer.pin.Unpin()
		return err
	}
	d.buffers = append(d.buffers, buffer)
	code, _, _ = waveWrite.Call(d.handle, ptr, size)
	return waveResult("waveOutWrite", code)
}

func (d *waveDevice) release(buffer *waveBuffer) error {
	code, _, _ := waveUnprepare.Call(d.handle, uintptr(unsafe.Pointer(&buffer.header)), unsafe.Sizeof(buffer.header))
	if err := waveResult("waveOutUnprepareHeader", code); err != nil {
		return err
	}
	buffer.pin.Unpin()
	return nil
}

func (d *waveDevice) Pending() (int, error) {
	for len(d.buffers) > 0 && atomic.LoadUint32(&d.buffers[0].header.Flags)&1 != 0 { // WHDR_DONE
		if err := d.release(d.buffers[0]); err != nil {
			return 0, err
		}
		d.buffers = d.buffers[1:]
	}
	return len(d.buffers), nil
}

func (d *waveDevice) Close() (err error) {
	defer func() {
		if err != nil {
			failedWaveDevices.Lock()
			failedWaveDevices.devices = append(failedWaveDevices.devices, d)
			failedWaveDevices.Unlock()
		}
	}()
	// Reset returns queued buffers and stops playback immediately on cancellation.
	code, _, _ := waveReset.Call(d.handle)
	if err := waveResult("waveOutReset", code); err != nil {
		return err
	}
	for len(d.buffers) > 0 {
		if err := d.release(d.buffers[0]); err != nil {
			return err
		}
		d.buffers = d.buffers[1:]
	}
	code, _, _ = waveClose.Call(d.handle)
	return waveResult("waveOutClose", code)
}
