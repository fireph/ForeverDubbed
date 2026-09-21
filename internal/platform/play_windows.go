//go:build windows

package platform

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var playSound = syscall.NewLazyDLL("winmm.dll").NewProc("PlaySoundW")

func PlayWAV(ctx context.Context, data []byte) error {
	duration, err := wavDuration(data)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ok, _, err := playSound.Call(uintptr(unsafe.Pointer(&data[0])), 0, 0x7)
	if ok == 0 {
		return fmt.Errorf("PlaySound: %v", err)
	}
	timer := time.NewTimer(duration + 50*time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-timer.C:
		err = nil
	}
	playSound.Call(0, 0, 0)
	runtime.KeepAlive(data)
	return err
}
