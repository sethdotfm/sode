package swp08

// Group salvos: SW-P-88 Issue 4b 8.1.4 (120), 8.1.5 (121), 8.2.5 (122), 8.2.6 (123), standard layout (4-bit matrix and level,
// 10-bit numbers, group 0-127). Verified on a Ross Ultrix (2026-11-02): 120 is answered by 122 echoing its data, 121 by 123, in 5-7 ms.
// A Go's crosspoints are sometimes announced as ordinary Connecteds (~4 ms apart, from ~100 ms after the Go), sometimes not.
// Extended stage 248 (layout F8 matrix level dstH dstL srcH srcL group, answered by 250 echoing it) is fired by the same Go; a
// Go-clear clears both kinds. It carries levels past 15 and numbers past 1023.
const (
	CmdConnectOnGo    = 120
	CmdGoSalvo        = 121
	CmdConnectOnGoAck = 122
	CmdGoDoneAck      = 123

	CmdExtConnectOnGo    = 248
	CmdExtConnectOnGoAck = 250
)

const (
	SalvoMaxMatrix = 15
	SalvoMaxLevel  = 15
	SalvoMaxNum    = 1023
	SalvoMaxGroup  = 127
)

const (
	GoSet     = 0
	GoCleared = 1
	GoNothing = 2
)

func SalvoFits(matrix, level, dst, src int) bool {
	return matrix >= 0 && matrix <= SalvoMaxMatrix && level >= 0 && level <= SalvoMaxLevel &&
		dst >= 0 && dst <= SalvoMaxNum && src >= 0 && src <= SalvoMaxNum
}

func ExtSalvoFits(matrix, level, dst, src int) bool {
	return matrix >= 0 && matrix <= 255 && level >= 0 && level <= 255 &&
		dst >= 0 && dst <= 0xFFFF && src >= 0 && src <= 0xFFFF
}

func ExtConnectOnGo(matrix, level, dst, src, group int) []byte {
	return []byte{CmdExtConnectOnGo, byte(matrix), byte(level), byte(dst >> 8), byte(dst), byte(src >> 8), byte(src), byte(group & 0x7f)}
}

func StageMsg(x Staged) []byte {
	if x.Ext {
		return ExtConnectOnGo(x.Matrix, x.Level, x.Dst, x.Src, x.Group)
	}
	return ConnectOnGo(x.Matrix, x.Level, x.Dst, x.Src, x.Group)
}

func StageAck(x Staged) []byte {
	m := StageMsg(x)
	if x.Ext {
		m[0] = CmdExtConnectOnGoAck
	} else {
		m[0] = CmdConnectOnGoAck
	}
	return m
}

// Out-of-range bits are masked off: it must fit (SalvoFits).
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

func GoSalvo(clear bool, group int) []byte {
	b := byte(0)
	if clear {
		b = 1
	}
	return []byte{CmdGoSalvo, b, byte(group & 0x7f)}
}

func ConnectOnGoAck(matrix, level, dst, src, group int) []byte {
	m := ConnectOnGo(matrix, level, dst, src, group)
	m[0] = CmdConnectOnGoAck
	return m
}

func GoDoneAck(status, group int) []byte {
	return []byte{CmdGoDoneAck, byte(status), byte(group & 0x7f)}
}

// Unset fields are -1 (the crosspoint for a 123, Status for a 122).
type SalvoAck struct {
	Cmd           byte
	Matrix, Level int
	Dst, Src      int
	Group         int
	Status        int
	Ext           bool
}

type Staged struct {
	Matrix, Level, Dst, Src, Group int
	Ext                            bool
}

func (s SalvoAck) Staged() Staged {
	return Staged{s.Matrix, s.Level, s.Dst, s.Src, s.Group, s.Ext}
}

func stagedOf(d []byte) Staged {
	return Staged{
		Matrix: int(d[0] >> 4),
		Level:  int(d[0] & 0x0f),
		Dst:    int(d[1]>>4&7)*128 + int(d[2]&0x7f),
		Src:    int(d[1]&7)*128 + int(d[3]&0x7f),
		Group:  int(d[4] & 0x7f),
	}
}

func extStagedOf(d []byte) Staged {
	return Staged{
		Matrix: int(d[0]),
		Level:  int(d[1]),
		Dst:    int(d[2])<<8 | int(d[3]),
		Src:    int(d[4])<<8 | int(d[5]),
		Group:  int(d[6] & 0x7f),
		Ext:    true,
	}
}

func StagedOf(m []byte) (Staged, bool) {
	if len(m) >= 8 && (m[0] == CmdExtConnectOnGo || m[0] == CmdExtConnectOnGoAck) {
		return extStagedOf(m[1:]), true
	}
	if len(m) < 6 || (m[0] != CmdConnectOnGo && m[0] != CmdConnectOnGoAck) {
		return Staged{}, false
	}
	return stagedOf(m[1:]), true
}
