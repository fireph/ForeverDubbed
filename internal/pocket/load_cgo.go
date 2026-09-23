//go:build pocket_native && cgo && (windows || linux || darwin)

package pocket

/*
#cgo CXXFLAGS: -std=c++17 -O2 -DPTT_SHARED_LIB
// ONNX Runtime uses _stdcall, which MinGW omits in strict C++17 mode.
#cgo windows CXXFLAGS: -D_stdcall=__stdcall
#cgo windows,amd64 CXXFLAGS: -I${SRCDIR}/../../.runtime/sdk/windows_amd64/include
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/../../.runtime/sdk/windows_amd64/lib
#cgo linux,amd64 CXXFLAGS: -I${SRCDIR}/../../.runtime/sdk/linux_amd64/include
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../.runtime/sdk/linux_amd64/lib
#cgo linux,arm64 CXXFLAGS: -I${SRCDIR}/../../.runtime/sdk/linux_arm64/include
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../.runtime/sdk/linux_arm64/lib
#cgo darwin,amd64 CXXFLAGS: -I${SRCDIR}/../../.runtime/sdk/darwin_amd64/include
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../.runtime/sdk/darwin_amd64/lib
#cgo darwin,arm64 CXXFLAGS: -I${SRCDIR}/../../.runtime/sdk/darwin_arm64/include
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../.runtime/sdk/darwin_arm64/lib
#cgo LDFLAGS: -lsentencepiece -lonnxruntime
#cgo windows LDFLAGS: -static -lstdc++ -lws2_32
#cgo linux LDFLAGS: -lstdc++ -lm -lpthread -Wl,-rpath,$ORIGIN/native -Wl,-rpath,$ORIGIN/../.runtime/native
#cgo darwin LDFLAGS: -lc++
#include <stdlib.h>
#include "bridge.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type linkedLibrary struct{}

func openLibrary() (library, error) { return linkedLibrary{}, nil }
func (linkedLibrary) create(models string, threads int) (unsafe.Pointer, error) {
	p := C.CString(models)
	defer C.free(unsafe.Pointer(p))
	var message [2048]C.char
	h := C.fdb_create(p, C.int(threads), &message[0], 2048)
	if h == nil {
		return nil, fmt.Errorf("PocketTTS.cpp: %s", C.GoString(&message[0]))
	}
	return h, nil
}
func (l linkedLibrary) start(h unsafe.Pointer, text, voice string, steps int) error {
	t, v := C.CString(text), C.CString(voice)
	defer C.free(unsafe.Pointer(t))
	defer C.free(unsafe.Pointer(v))
	if C.fdb_start(h, t, v, C.int(steps)) != 0 {
		return l.message(h)
	}
	return nil
}
func (l linkedLibrary) read(h unsafe.Pointer, out []int16) (int, error) {
	n := int(C.fdb_read(h, (*C.int16_t)(unsafe.Pointer(&out[0])), C.int(len(out))))
	if n == -2 {
		return 0, l.message(h)
	}
	return n, nil
}
func (linkedLibrary) stop(h unsafe.Pointer)    { C.fdb_stop(h) }
func (linkedLibrary) destroy(h unsafe.Pointer) { C.fdb_destroy(h) }
func (linkedLibrary) close()                   {}
func (linkedLibrary) message(h unsafe.Pointer) error {
	var message [2048]C.char
	C.fdb_error(h, &message[0], 2048)
	return fmt.Errorf("PocketTTS.cpp: %s", C.GoString(&message[0]))
}
