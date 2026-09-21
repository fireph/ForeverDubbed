package platform

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestWAVDurationAndValidation(t *testing.T) {
	// One second of 24 kHz, mono, signed 16-bit PCM as emitted by Pocket TTS.
	b := make([]byte, 44+48000)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 24000)
	binary.LittleEndian.PutUint32(b[28:], 48000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], 48000)
	if d, err := wavDuration(b); err != nil || d != time.Second {
		t.Fatal(d, err)
	}
	for name, change := range map[string]func([]byte) []byte{
		"truncated": func(v []byte) []byte { return v[:len(v)-1] },
		"float":     func(v []byte) []byte { v[20] = 3; return v },
		"bad rate":  func(v []byte) []byte { v[28] = 0; return v },
		"empty":     func(v []byte) []byte { binary.LittleEndian.PutUint32(v[40:], 0); return v[:44] },
		"header":    func(v []byte) []byte { v[0] = '?'; return v },
	} {
		t.Run(name, func(t *testing.T) {
			v := append([]byte(nil), b...)
			if _, err := wavDuration(change(v)); err == nil {
				t.Fatal("accepted invalid WAV")
			}
		})
	}
}
