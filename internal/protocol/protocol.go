// Package protocol implements the 16-color ForeverDubbed optical transport.
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/adler32"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// Control kinds carry empty text and must never enter the speech queue.
	KindStop     byte = 7
	KindSkip     byte = 8
	Grid              = 50
	DataGrid          = 48
	WaveCells         = 136
	FrameBytes        = (DataGrid*DataGrid - WaveCells) / 2
	HeaderBytes       = 24
	PayloadBytes      = FrameBytes - HeaderBytes - 4
	MaxPages          = 256
)

// Cosmetic noise for unused payload bytes, shared with the Lua encoder.
// Generate once; keeping it stable avoids randomizing textures on every tick.
var padding = func() [PayloadBytes]byte {
	var data [PayloadBytes]byte
	state := uint64(1)
	for i := range data {
		state = state * 48271 % 2147483647
		data[i] = byte(state % 256)
	}
	return data
}()

type Packet struct {
	Session, Sequence, Checksum uint32
	Index, Count                uint16
	Kind                        byte
	Flags                       byte
	Payload                     []byte
}

type Message struct {
	Session      uint32 `json:"session"`
	Sequence     uint32 `json:"sequence"`
	Kind         byte   `json:"kind"`
	Speaker      string `json:"speaker"`
	Title        string `json:"title"`
	Text         string `json:"text"`
	Race         string `json:"race,omitempty"`
	Gender       string `json:"gender,omitempty"`
	NPCID        string `json:"npc_id,omitempty"`
	DisplayID    string `json:"display_id,omitempty"`
	ModelID      string `json:"model_id,omitempty"`
	RaceOverride string `json:"race_override,omitempty"`
	RaceSource   string `json:"race_source,omitempty"` // Desktop diagnostic; not encoded.
}

func (m Message) IsControl() bool { return m.Kind == KindStop || m.Kind == KindSkip }

func (m Message) Speech() string {
	parts := []string{}
	for _, s := range []string{m.Speaker, m.Title, m.Text} {
		if strings.TrimSpace(s) != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ". ")
}

func Encode(m Message) ([][]byte, error) {
	for _, s := range []string{m.Speaker, m.Title, m.Text, m.Race, m.Gender, m.NPCID, m.DisplayID, m.ModelID, m.RaceOverride} {
		if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
			return nil, errors.New("fields must be UTF-8 without NUL")
		}
	}
	body := []byte(m.Speaker + "\x00" + m.Title + "\x00" + m.Text)
	flags := byte(0)
	if m.Race != "" || m.Gender != "" || m.NPCID != "" || m.DisplayID != "" || m.ModelID != "" || m.RaceOverride != "" {
		body = append(body, []byte("\x00"+m.Race+"\x00"+m.Gender+"\x00"+m.NPCID)...)
		flags = 1
	}
	if m.DisplayID != "" || m.ModelID != "" || m.RaceOverride != "" {
		body = append(body, []byte("\x00"+m.DisplayID+"\x00"+m.ModelID+"\x00"+m.RaceOverride)...)
		flags = 2
	}
	capacity := PayloadBytes
	count := (len(body) + capacity - 1) / capacity
	if count > MaxPages {
		return nil, fmt.Errorf("message exceeds %d bytes", MaxPages*capacity)
	}
	frames := make([][]byte, count)
	for i := range frames {
		end := min((i+1)*capacity, len(body))
		p := body[i*capacity : end]
		b := make([]byte, FrameBytes)
		copy(b, "FDB5")
		binary.BigEndian.PutUint32(b[4:], m.Session)
		binary.BigEndian.PutUint32(b[8:], m.Sequence)
		binary.BigEndian.PutUint32(b[12:], adler32.Checksum(body))
		binary.BigEndian.PutUint16(b[16:], uint16(i))
		binary.BigEndian.PutUint16(b[18:], uint16(count))
		binary.BigEndian.PutUint16(b[20:], uint16(len(p)))
		b[22] = m.Kind
		b[23] = flags
		copy(b[24:], p)
		checksumAt := len(b) - 4
		copy(b[HeaderBytes+len(p):checksumAt], padding[:])
		binary.BigEndian.PutUint32(b[checksumAt:], adler32.Checksum(b[:checksumAt]))
		frames[i] = b
	}
	return frames, nil
}

func Parse(b []byte) (Packet, error) {
	p := Packet{}
	if len(b) != FrameBytes || string(b[:4]) != "FDB5" || b[23] > 2 {
		return p, errors.New("invalid frame header")
	}
	checksumAt := len(b) - 4
	if adler32.Checksum(b[:checksumAt]) != binary.BigEndian.Uint32(b[checksumAt:]) {
		return p, errors.New("frame checksum mismatch")
	}
	p.Session = binary.BigEndian.Uint32(b[4:])
	p.Sequence = binary.BigEndian.Uint32(b[8:])
	p.Checksum = binary.BigEndian.Uint32(b[12:])
	p.Index = binary.BigEndian.Uint16(b[16:])
	p.Count = binary.BigEndian.Uint16(b[18:])
	n := int(binary.BigEndian.Uint16(b[20:]))
	p.Kind = b[22]
	p.Flags = b[23]
	if p.Count == 0 || p.Count > MaxPages || p.Index >= p.Count || n == 0 || n > PayloadBytes || (p.Index+1 < p.Count && n != PayloadBytes) {
		return Packet{}, errors.New("invalid page dimensions")
	}
	p.Payload = bytes.Clone(b[24 : 24+n])
	return p, nil
}

// Assembler follows the newest observed message and emits it once. Page order
// and duplicates do not matter. Only eight recently seen sessions are retained.
type Assembler struct {
	current  Packet
	parts    map[uint16][]byte
	done     bool
	last     time.Time
	sessions []uint32
}

func (a *Assembler) Add(p Packet, now time.Time) (*Message, error) {
	if p.Count == 0 || p.Count > MaxPages || p.Index >= p.Count || len(p.Payload) == 0 || len(p.Payload) > PayloadBytes || (p.Index+1 < p.Count && len(p.Payload) != PayloadBytes) {
		return nil, errors.New("invalid packet")
	}
	if a.parts == nil || p.Session != a.current.Session {
		for _, s := range a.sessions {
			if s == p.Session {
				return nil, nil
			}
		}
		if a.parts != nil {
			a.sessions = append(a.sessions, a.current.Session)
			if len(a.sessions) > 8 {
				a.sessions = a.sessions[1:]
			}
		}
		a.reset(p)
	} else if p.Sequence != a.current.Sequence {
		if int32(p.Sequence-a.current.Sequence) <= 0 {
			return nil, nil
		}
		a.reset(p)
	}
	if a.done {
		return nil, nil
	}
	if now.Sub(a.last) > 2*time.Minute {
		a.parts = make(map[uint16][]byte)
	}
	a.last = now
	if p.Flags != a.current.Flags || p.Checksum != a.current.Checksum || p.Count != a.current.Count || p.Kind != a.current.Kind {
		return nil, errors.New("inconsistent message metadata")
	}
	if old, ok := a.parts[p.Index]; ok && !bytes.Equal(old, p.Payload) {
		return nil, errors.New("conflicting page")
	}
	a.parts[p.Index] = bytes.Clone(p.Payload)
	if len(a.parts) != int(p.Count) {
		return nil, nil
	}
	var body []byte
	for i := uint16(0); i < p.Count; i++ {
		body = append(body, a.parts[i]...)
	}
	if adler32.Checksum(body) != p.Checksum {
		a.parts = make(map[uint16][]byte)
		return nil, errors.New("message checksum mismatch")
	}
	fields := bytes.Split(body, []byte{0})
	expected := 3
	if p.Flags <= 2 {
		expected += 3 * int(p.Flags)
	}
	if p.Flags > 2 || len(fields) != expected || !utf8.Valid(body) {
		return nil, fmt.Errorf("invalid UTF-8 message fields")
	}
	a.done = true
	m := &Message{Session: p.Session, Sequence: p.Sequence, Kind: p.Kind, Speaker: string(fields[0]), Title: string(fields[1]), Text: string(fields[2])}
	if p.Flags >= 1 {
		m.Race = string(fields[3])
		m.Gender = string(fields[4])
		m.NPCID = string(fields[5])
	}
	if p.Flags == 2 {
		m.DisplayID = string(fields[6])
		m.ModelID = string(fields[7])
		m.RaceOverride = string(fields[8])
	}
	return m, nil
}

func (a *Assembler) reset(p Packet) { a.current = p; a.parts = make(map[uint16][]byte); a.done = false }
