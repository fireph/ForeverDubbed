package platform

import (
	"encoding/binary"
	"fmt"
	"time"
)

func wavDuration(b []byte) (time.Duration, error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return 0, fmt.Errorf("invalid WAV header")
	}
	var rate, bytesPerSecond uint32
	var size int
	for p := 12; p+8 <= len(b); {
		n := int(binary.LittleEndian.Uint32(b[p+4:]))
		start := p + 8
		if n > len(b)-start {
			return 0, fmt.Errorf("truncated WAV chunk")
		}
		switch string(b[p : p+4]) {
		case "fmt ":
			if n < 16 || binary.LittleEndian.Uint16(b[start:]) != 1 {
				return 0, fmt.Errorf("WAV must contain PCM audio")
			}
			channels := binary.LittleEndian.Uint16(b[start+2:])
			rate = binary.LittleEndian.Uint32(b[start+4:])
			bits := binary.LittleEndian.Uint16(b[start+14:])
			bytesPerSecond = binary.LittleEndian.Uint32(b[start+8:])
			if channels < 1 || channels > 2 || bits != 16 || rate < 8000 || rate > 96000 || bytesPerSecond != rate*uint32(channels)*2 {
				return 0, fmt.Errorf("unsupported PCM format")
			}
		case "data":
			size += n
		}
		p = start + n + n%2
	}
	if rate == 0 || bytesPerSecond == 0 || size == 0 {
		return 0, fmt.Errorf("empty WAV")
	}
	return time.Duration(int64(size) * int64(time.Second) / int64(bytesPerSecond)), nil
}
