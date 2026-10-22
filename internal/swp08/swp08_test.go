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
	enc[len(enc)-3]++ // corrupt CHK
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
	want := []Crosspoint{{Matrix: 1, Level: 2, Dst: 389, Src: 263}}
	if !reflect.DeepEqual(ev.Crosspoints, want) {
		t.Fatalf("got %+v", ev.Crosspoints)
	}
}

func TestParseExtDump(t *testing.T) {
	ev, err := Parse(ExtTallyDump(0, 3, 1000, []int{5, 1500}))
	if err != nil {
		t.Fatal(err)
	}
	want := []Crosspoint{{0, 3, 1000, 5}, {0, 3, 1001, 1500}}
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
