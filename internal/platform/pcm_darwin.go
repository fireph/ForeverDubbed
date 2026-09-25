//go:build darwin && cgo

package platform

/*
#include "native_darwin.h"
*/
import "C"
import (
	"context"
	"fmt"
	"time"
	"unsafe"
)

type macAudio struct{ handle *C.fdb_audio }

func audioResult(op string, status C.int) error {
	if status != 0 {
		return fmt.Errorf("%s failed (OSStatus %d)", op, int(status))
	}
	return nil
}
func openAudio(rate int) (pcmDevice, error) {
	var status C.int
	handle := C.fdb_audio_open(C.int(rate), &status)
	if err := audioResult("AudioQueueNewOutput", status); err != nil {
		return nil, err
	}
	return &macAudio{handle}, nil
}
func (d *macAudio) Queue(pcm []byte) error {
	// The bridge copies bytes into AudioQueue-owned memory before returning.
	return audioResult("AudioQueueEnqueueBuffer", C.fdb_audio_queue(d.handle, unsafe.Pointer(&pcm[0]), C.int(len(pcm))))
}
func (d *macAudio) Pending() (int, error) { return int(C.fdb_audio_pending(d.handle)), nil }
func (d *macAudio) Close() error {
	return audioResult("AudioQueueDispose", C.fdb_audio_close(d.handle))
}

func (d *macAudio) Drain(ctx context.Context) error {
	if err := audioResult("AudioQueueStop", C.fdb_audio_drain(d.handle)); err != nil {
		return err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var running C.uint
		if err := audioResult("AudioQueueGetProperty", C.fdb_audio_running(d.handle, &running)); err != nil {
			return err
		}
		if running == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
