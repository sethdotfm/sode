package swp08

import (
	"bytes"
	"reflect"
	"testing"
)

func decodeAll(t *testing.T, p []byte) []Frame {
	t.Helper()
	var d Decoder
	var out []Frame
	d.Feed(p, func(f Frame) { out = append(out, f) })
	return out
}

func TestEncodeChecksum(t *testing.T) {
	// cmd 0x02, data 00 00 05 03 → BTC 5, sum 0x0F, CHK 0xF1
	got := Encode([]byte{0x02, 0x00, 0x00, 0x05, 0x03})
	want := []byte{DLE, STX, 0x02, 0x00, 0x00, 0x05, 0x03, 0x05, 0xF1, DLE, ETX}
	if !bytes.Equal(got, want) {
		t.Fatalf("got % X want % X", got, want)
	}
}

func TestRoundTripWithDLE(t *testing.T) {
	msg := ExtConnect(0, 0x10, 0x1010, 0x0110)
	enc := Encode(msg)
	fs := decodeAll(t, append(append([]byte{0xFF}, enc...), DLE, ACK))
	if len(fs) != 2 || fs[0].Kind != KindMsg || fs[1].Kind != KindAck {
		t.Fatalf("frames %+v", fs)
	}
	if fs[0].Err != nil || !bytes.Equal(fs[0].Msg, msg) {
		t.Fatalf("got % X err %v", fs[0].Msg, fs[0].Err)
	}
}

func TestSplitFeed(t *testing.T) {
	enc := Encode(ExtTally(1, 2, 300, 400))
	var d Decoder
	var got []Frame
	for _, b := range enc {
		d.Feed([]byte{b}, func(f Frame) { got = append(got, f) })
	}
	if len(got) != 1 || got[0].Kind != KindMsg {
		t.Fatalf("frames %+v", got)
	}
}

func TestBadChecksum(t *testing.T) {
	enc := Encode([]byte{0x01, 0x00, 0x00, 0x05})
	enc[len(enc)-3]++
	fs := decodeAll(t, enc)
	if len(fs) != 1 || fs[0].Kind != KindBad || fs[0].Err != ErrChecksum {
		t.Fatalf("frames %+v", fs)
	}
}

func TestParseStdConnected(t *testing.T) {
	// matrix 1 level 2, dst = 3*128+5 = 389, src = 2*128+7 = 263
	ev, err := Parse([]byte{CmdConnected, 0x12, 0x32, 0x05, 0x07})
	if err != nil {
		t.Fatal(err)
	}
	want := []Crosspoint{{Matrix: 1, Level: 2, Dst: 389, Src: 263, Cmd: CmdConnected}}
	if !reflect.DeepEqual(ev.Crosspoints, want) {
		t.Fatalf("got %+v", ev.Crosspoints)
	}
}

func TestParseExtDump(t *testing.T) {
	ev, err := Parse(ExtTallyDump(0, 3, 1000, []int{5, 1500}))
	if err != nil {
		t.Fatal(err)
	}
	want := []Crosspoint{{0, 3, 1000, 5, CmdExtTallyDump}, {0, 3, 1001, 1500, CmdExtTallyDump}}
	if !reflect.DeepEqual(ev.Crosspoints, want) {
		t.Fatalf("got %+v", ev.Crosspoints)
	}
}

func TestParseNames(t *testing.T) {
	for _, ext := range []bool{false, true} {
		ev, err := Parse(NamesResp(true, ext, 0, 0, 2, 300, []string{"DSM5", "Zoom Return 1"}))
		if err != nil {
			t.Fatal(err)
		}
		nb := ev.Names
		if nb == nil || !nb.Dest || nb.First != 300 || !reflect.DeepEqual(nb.Names, []string{"DSM5", "Zoom Return"}) {
			t.Fatalf("ext=%v got %+v", ext, nb)
		}
	}
}

func TestParseProtectDump(t *testing.T) {
	// Captured from a Ross Ultrix: level 0, dsts 64-65, 64 protected by device 9.
	ev, err := Parse([]byte{CmdExtProtectDump, 0x00, 0x00, 0x02, 0x00, 0x40, 0x20, 0x09, 0x00, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	want := []Protect{{Dst: 64, Protected: true, Device: 9}, {Dst: 65}}
	if !reflect.DeepEqual(ev.Protects, want) {
		t.Fatalf("got %+v", ev.Protects)
	}
}

// Frames captured from a Ross Ultrix on 2026-11-16. The 7-byte order is matrix, level, details, dst(2),
// device(2).
func TestParseProtectChangeRouterOrder(t *testing.T) {
	for _, c := range []struct {
		name string
		msg  []byte
		want Protect
	}{
		{"0x8D protect dst 26 level 0 device 126", []byte{0x8D, 0x00, 0x00, 0x01, 0x00, 0x1A, 0x00, 0x7E},
			Protect{Matrix: 0, Level: 0, Dst: 26, Protected: true, Device: 126}},
		{"0x8B dst 64 level 0 device 9", []byte{0x8B, 0x00, 0x00, 0x02, 0x00, 0x40, 0x00, 0x09},
			Protect{Matrix: 0, Level: 0, Dst: 64, Protected: true, Device: 9}},
		{"0x8B dst 64 level 16 device 9", []byte{0x8B, 0x00, 0x10, 0x02, 0x00, 0x40, 0x00, 0x09},
			Protect{Matrix: 0, Level: 16, Dst: 64, Protected: true, Device: 9}},
		{"0x8F clear dst 26 level 0", []byte{0x8F, 0x00, 0x00, 0x00, 0x00, 0x1A, 0x00, 0x00},
			Protect{Matrix: 0, Level: 0, Dst: 26}},
		{"0x8F with details set still clears", []byte{0x8F, 0x00, 0x05, 0x01, 0x00, 0x1A, 0x00, 0x7E},
			Protect{Matrix: 0, Level: 5, Dst: 26, Device: 126}},
	} {
		ev, err := Parse(c.msg)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(ev.Protects, []Protect{c.want}) {
			t.Errorf("%s: got %+v, want %+v", c.name, ev.Protects, c.want)
		}
	}
}

func TestExtProtectChangeRoundTrips(t *testing.T) {
	m := ExtProtectChange(CmdExtProtectConnected, 0, 16, 2, 0x140, 10)
	if want := []byte{0x8D, 0x00, 0x10, 0x02, 0x01, 0x40, 0x00, 0x0A}; !reflect.DeepEqual(m, want) {
		t.Fatalf("% X, want % X", m, want)
	}
}

func TestDescribeTx(t *testing.T) {
	for _, c := range []struct {
		m    []byte
		want Request
	}{
		{ExtConnect(1, 2, 700, 2600), Request{"connect", 1, 2, 700, 2600, -1, false}},
		{ExtInterrogate(0, 16, 300), Request{"interrogate", 0, 16, 300, -1, -1, false}},
		{ExtTallyDumpReq(0, 3), Request{"tallyDump", 0, 3, -1, -1, -1, false}},
		{ExtProtectDumpReq(0, 4, 5), Request{"protectDump", 0, 4, 5, -1, -1, false}},
		{NamesReq(false, true, 0, 0, 2), Request{"srcNames", 0, -1, -1, -1, -1, false}},
		{NamesReq(true, false, 1, 0, 2), Request{"dstNames", 1, -1, -1, -1, -1, false}},
		{[]byte{0x7F, 1}, Request{"other", -1, -1, -1, -1, -1, false}},
		{[]byte{CmdExtConnect, 0}, Request{"other", -1, -1, -1, -1, -1, false}},
		{ConnectOnGo(1, 2, 389, 263, 127), Request{"salvoStage", 1, 2, 389, 263, 127, false}},
		{GoSalvo(true, 127), Request{"salvoGo", -1, -1, -1, -1, 127, true}},
		{[]byte{CmdConnectOnGo, 0, 0}, Request{"other", -1, -1, -1, -1, -1, false}},
	} {
		if got := DescribeTx(c.m); got != c.want {
			t.Errorf("DescribeTx(% X) = %+v, want %+v", c.m, got, c.want)
		}
	}
}
