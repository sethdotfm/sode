package swp08

import (
	"errors"
	"fmt"
	"strings"
)

// Command bytes. Extended variants are the general ones + 0x80 and carry
// 16-bit source/destination numbers and a full byte each for matrix and level.
const (
	CmdInterrogate   = 0x01
	CmdConnect       = 0x02
	CmdTally         = 0x03
	CmdConnected     = 0x04
	CmdTallyDumpReq  = 0x15
	CmdTallyDumpByte = 0x16
	CmdTallyDumpWord = 0x17
	CmdSrcNamesReq   = 0x64
	CmdDstNamesReq   = 0x66
	CmdSrcNames      = 0x6A
	CmdDstNames      = 0x6B

	CmdExtInterrogate  = 0x81
	CmdExtConnect      = 0x82
	CmdExtTally        = 0x83
	CmdExtConnected    = 0x84
	CmdExtTallyDumpReq = 0x95
	CmdExtTallyDump    = 0x97
	CmdExtSrcNamesReq  = 0xE4
	CmdExtDstNamesReq  = 0xE6
	CmdExtSrcNames     = 0xEA
	CmdExtDstNames     = 0xEB
)

// Name length codes used by name requests and responses.
var nameLens = map[byte]int{0: 4, 1: 8, 2: 12, 3: 16}

// NameLenCode returns the protocol code for a name length of 4, 8, 12 or 16.
func NameLenCode(chars int) (byte, error) {
	for c, n := range nameLens {
		if n == chars {
			return c, nil
		}
	}
	return 0, fmt.Errorf("swp08: name length %d not one of 4, 8, 12, 16", chars)
}

// NamesPerMsg is how many names of a given length fit one response.
func NamesPerMsg(code byte) int {
	switch code {
	case 0:
		return 32
	case 1:
		return 16
	case 2:
		return 10
	default:
		return 8
	}
}

type Crosspoint struct {
	Matrix, Level int
	Dst, Src      int
}

type NameBlock struct {
	Dest          bool
	Matrix, Level int
	First         int
	Names         []string
}

// Event is a parsed inbound message. Unknown commands leave both empty.
type Event struct {
	Cmd         byte
	Crosspoints []Crosspoint
	Names       *NameBlock
}

func ExtInterrogate(matrix, level, dst int) []byte {
	return []byte{CmdExtInterrogate, byte(matrix), byte(level), byte(dst >> 8), byte(dst)}
}

func ExtConnect(matrix, level, dst, src int) []byte {
	return xp(CmdExtConnect, matrix, level, dst, src)
}

func ExtTally(matrix, level, dst, src int) []byte {
	return xp(CmdExtTally, matrix, level, dst, src)
}

func ExtConnected(matrix, level, dst, src int) []byte {
	return xp(CmdExtConnected, matrix, level, dst, src)
}

func xp(cmd byte, matrix, level, dst, src int) []byte {
	return []byte{cmd, byte(matrix), byte(level), byte(dst >> 8), byte(dst), byte(src >> 8), byte(src)}
}

func ExtTallyDumpReq(matrix, level int) []byte {
	return []byte{CmdExtTallyDumpReq, byte(matrix), byte(level)}
}

// ExtTallyDump builds one tally dump message: srcs[i] feeds firstDst+i.
func ExtTallyDump(matrix, level, firstDst int, srcs []int) []byte {
	m := []byte{CmdExtTallyDump, byte(matrix), byte(level), byte(len(srcs)), byte(firstDst >> 8), byte(firstDst)}
	for _, s := range srcs {
		m = append(m, byte(s>>8), byte(s))
	}
	return m
}

// NamesReq requests all source (or destination association) names.
func NamesReq(dest, ext bool, matrix, level int, lenCode byte) []byte {
	if ext {
		cmd := byte(CmdExtSrcNamesReq)
		if dest {
			cmd = CmdExtDstNamesReq
		}
		return []byte{cmd, byte(matrix), byte(level), lenCode}
	}
	cmd := byte(CmdSrcNamesReq)
	if dest {
		cmd = CmdDstNamesReq
	}
	return []byte{cmd, byte(matrix<<4 | level&0x0f), lenCode}
}

// NamesResp builds one names response message.
func NamesResp(dest, ext bool, matrix, level int, lenCode byte, first int, names []string) []byte {
	n := nameLens[lenCode]
	var m []byte
	if ext {
		cmd := byte(CmdExtSrcNames)
		if dest {
			cmd = CmdExtDstNames
		}
		m = []byte{cmd, byte(matrix), byte(level)}
	} else {
		cmd := byte(CmdSrcNames)
		if dest {
			cmd = CmdDstNames
		}
		m = []byte{cmd, byte(matrix<<4 | level&0x0f)}
	}
	m = append(m, lenCode, byte(first>>8), byte(first), byte(len(names)))
	for _, s := range names {
		b := []byte(s)
		if len(b) > n {
			b = b[:n]
		}
		m = append(m, b...)
		m = append(m, []byte(strings.Repeat(" ", n-len(b)))...)
	}
	return m
}

// Parse decodes the inbound messages Sode cares about.
func Parse(msg []byte) (Event, error) {
	if len(msg) == 0 {
		return Event{}, ErrShort
	}
	ev := Event{Cmd: msg[0]}
	d := msg[1:]
	short := func(n int) error {
		if len(d) < n {
			return fmt.Errorf("swp08: cmd 0x%02X: %d data bytes, need %d", msg[0], len(d), n)
		}
		return nil
	}

	switch msg[0] {
	case CmdTally, CmdConnected:
		if err := short(4); err != nil {
			return ev, err
		}
		ev.Crosspoints = []Crosspoint{{
			Matrix: int(d[0] >> 4),
			Level:  int(d[0] & 0x0f),
			Dst:    int(d[1]>>4&0x07)*128 + int(d[2]&0x7f),
			Src:    int(d[1]&0x07)*128 + int(d[3]&0x7f),
		}}

	case CmdExtTally, CmdExtConnected:
		if err := short(6); err != nil {
			return ev, err
		}
		ev.Crosspoints = []Crosspoint{{
			Matrix: int(d[0]),
			Level:  int(d[1]),
			Dst:    int(d[2])<<8 | int(d[3]),
			Src:    int(d[4])<<8 | int(d[5]),
		}}

	case CmdTallyDumpByte:
		if err := short(3); err != nil {
			return ev, err
		}
		count, first := int(d[1]), int(d[2])
		if err := short(3 + count); err != nil {
			return ev, err
		}
		for i := 0; i < count; i++ {
			ev.Crosspoints = append(ev.Crosspoints, Crosspoint{
				Matrix: int(d[0] >> 4), Level: int(d[0] & 0x0f),
				Dst: first + i, Src: int(d[3+i]),
			})
		}

	case CmdTallyDumpWord:
		if err := short(4); err != nil {
			return ev, err
		}
		ev.Crosspoints = dumpWords(int(d[0]>>4), int(d[0]&0x0f), d[1:])
		if ev.Crosspoints == nil {
			return ev, errTruncDump
		}

	case CmdExtTallyDump:
		if err := short(5); err != nil {
			return ev, err
		}
		ev.Crosspoints = dumpWords(int(d[0]), int(d[1]), d[2:])
		if ev.Crosspoints == nil {
			return ev, errTruncDump
		}

	case CmdSrcNames, CmdDstNames:
		if err := short(5); err != nil {
			return ev, err
		}
		nb, err := names(msg[0] == CmdDstNames, int(d[0]>>4), int(d[0]&0x0f), d[1:])
		ev.Names = nb
		return ev, err

	case CmdExtSrcNames, CmdExtDstNames:
		if err := short(6); err != nil {
			return ev, err
		}
		nb, err := names(msg[0] == CmdExtDstNames, int(d[0]), int(d[1]), d[2:])
		ev.Names = nb
		return ev, err
	}
	return ev, nil
}

var errTruncDump = errors.New("swp08: tally dump truncated")

// dumpWords parses: count, firstDst(2), count × src(2). Nil if truncated.
func dumpWords(matrix, level int, d []byte) []Crosspoint {
	count := int(d[0])
	first := int(d[1])<<8 | int(d[2])
	if len(d) < 3+count*2 {
		return nil
	}
	xps := make([]Crosspoint, 0, count)
	for i := 0; i < count; i++ {
		o := 3 + i*2
		xps = append(xps, Crosspoint{
			Matrix: matrix, Level: level,
			Dst: first + i, Src: int(d[o])<<8 | int(d[o+1]),
		})
	}
	return xps
}

// names parses: lenCode, first(2), count, names.
func names(dest bool, matrix, level int, d []byte) (*NameBlock, error) {
	count := int(d[3])
	nb := &NameBlock{Dest: dest, Matrix: matrix, Level: level, First: int(d[1])<<8 | int(d[2])}
	if count == 0 {
		return nb, nil
	}
	rest := d[4:]
	size, ok := nameLens[d[0]]
	if !ok || size*count > len(rest) {
		size = len(rest) / count
	}
	if size == 0 {
		return nb, fmt.Errorf("swp08: names message too short for %d names", count)
	}
	for i := 0; i < count; i++ {
		raw := string(rest[i*size : (i+1)*size])
		nb.Names = append(nb.Names, strings.TrimRight(raw, " \x00"))
	}
	return nb, nil
}
