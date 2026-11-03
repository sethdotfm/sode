package swp08

// Group salvos: SW-P-88 Issue 4b, sections 8.1.4 (120), 8.1.5 (121),
// 8.2.5 (122), 8.2.6 (123). They use the standard (non-extended) layout: a
// 4-bit matrix and level, 10-bit destination and source numbers, a salvo
// group 0-127. Verified on a Ross Ultrix (2026-11-02): 120 is answered by
// 122 echoing its data, 121 by 123, both in about 5-7 ms. A Go's
// crosspoints are sometimes announced with ordinary Connecteds (about 4 ms
// apart, from ~100 ms after the Go) and sometimes not. The extended stage
// 248 (answered by 250) also works and is fired by the same Go; not used yet.
const (
	CmdConnectOnGo    = 120 // stage a crosspoint in a salvo group
	CmdGoSalvo        = 121 // set (fire) or clear a group's staged crosspoints
	CmdConnectOnGoAck = 122 // a crosspoint was staged: echoes 120's data
	CmdGoDoneAck      = 123 // a Go was carried out: status, group
)

// Limits of the standard layout group salvos use.
const (
	SalvoMaxMatrix = 15
	SalvoMaxLevel  = 15
	SalvoMaxNum    = 1023 // destination and source numbers
	SalvoMaxGroup  = 127
)

// Statuses of a 123 Go done acknowledge.
const (
	GoSet     = 0 // the staged crosspoints were set
	GoCleared = 1 // the staged crosspoints were cleared
	GoNothing = 2 // nothing was staged
)

// SalvoFits reports whether a crosspoint can be staged in a group salvo.
func SalvoFits(matrix, level, dst, src int) bool {
	return matrix >= 0 && matrix <= SalvoMaxMatrix && level >= 0 && level <= SalvoMaxLevel &&
		dst >= 0 && dst <= SalvoMaxNum && src >= 0 && src <= SalvoMaxNum
}

// ConnectOnGo stages one crosspoint in salvo group (0-127). It must fit
// (SalvoFits); out-of-range bits are masked off.
func ConnectOnGo(matrix, level, dst, src, group int) []byte {
	return []byte{
		CmdConnectOnGo,
		byte(matrix&0x0f<<4 | level&0x0f),
		byte((dst>>7&7)<<4 | src>>7&7), // dst DIV 128 in bits 4-6, src DIV 128 in bits 0-2
		byte(dst & 0x7f),
		byte(src & 0x7f),
		byte(group & 0x7f),
	}
}

// GoSalvo sets (clear=false) or discards (clear=true) the crosspoints staged
// in a salvo group.
func GoSalvo(clear bool, group int) []byte {
	b := byte(0)
	if clear {
		b = 1
	}
	return []byte{CmdGoSalvo, b, byte(group & 0x7f)}
}

// ConnectOnGoAck is the router's answer to ConnectOnGo (for the simulator
// and tests): the same data.
func ConnectOnGoAck(matrix, level, dst, src, group int) []byte {
	m := ConnectOnGo(matrix, level, dst, src, group)
	m[0] = CmdConnectOnGoAck
	return m
}

// GoDoneAck is the router's answer to GoSalvo.
func GoDoneAck(status, group int) []byte {
	return []byte{CmdGoDoneAck, byte(status), byte(group & 0x7f)}
}

// SalvoAck is a parsed 122 or 123. For 122 the crosspoint fields are set
// and Status is -1; for 123 Status is set and the crosspoint fields are -1.
type SalvoAck struct {
	Cmd           byte
	Matrix, Level int
	Dst, Src      int
	Group         int
	Status        int
}

// Staged is the crosspoint a 120 stages, or a 122 acknowledges, in the
// same form, so the two can be matched.
type Staged struct{ Matrix, Level, Dst, Src, Group int }

// Staged is the crosspoint a 122 acknowledges.
func (s SalvoAck) Staged() Staged { return Staged{s.Matrix, s.Level, s.Dst, s.Src, s.Group} }

// stagedOf decodes the 5 data bytes shared by 120 and 122.
func stagedOf(d []byte) Staged {
	return Staged{
		Matrix: int(d[0] >> 4),
		Level:  int(d[0] & 0x0f),
		Dst:    int(d[1]>>4&7)*128 + int(d[2]&0x7f),
		Src:    int(d[1]&7)*128 + int(d[3]&0x7f),
		Group:  int(d[4] & 0x7f),
	}
}

// StagedOf decodes a ConnectOnGo message; ok is false if m isn't one.
func StagedOf(m []byte) (Staged, bool) {
	if len(m) < 6 || (m[0] != CmdConnectOnGo && m[0] != CmdConnectOnGoAck) {
		return Staged{}, false
	}
	return stagedOf(m[1:]), true
}
