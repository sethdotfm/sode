package swp08

import (
	"bytes"
	"testing"
)

func TestConnectOnGoLayout(t *testing.T) {
	// The probe's tested layout: matrix<<4|level, dst/128<<4 | src/128,
	// dst%128, src%128, group. dst 389 = 3*128+5, src 263 = 2*128+7.
	got := ConnectOnGo(1, 2, 389, 263, 127)
	want := []byte{120, 0x12, 0x32, 5, 7, 127}
	if !bytes.Equal(got, want) {
		t.Fatalf("got % X want % X", got, want)
	}
	x, ok := StagedOf(got)
	if !ok || x != (Staged{1, 2, 389, 263, 127}) {
		t.Fatalf("staged %+v %v", x, ok)
	}
	// Largest numbers.
	x, _ = StagedOf(ConnectOnGo(15, 15, 1023, 1023, 0))
	if x != (Staged{15, 15, 1023, 1023, 0}) {
		t.Fatalf("max %+v", x)
	}
}

func TestGoSalvoLayout(t *testing.T) {
	if got := GoSalvo(false, 127); !bytes.Equal(got, []byte{121, 0, 127}) {
		t.Fatalf("set % X", got)
	}
	if got := GoSalvo(true, 5); !bytes.Equal(got, []byte{121, 1, 5}) {
		t.Fatalf("clear % X", got)
	}
}

func TestSalvoFits(t *testing.T) {
	for _, c := range []struct {
		mx, l, d, s int
		ok          bool
	}{
		{0, 0, 0, 0, true}, {15, 15, 1023, 1023, true},
		{16, 0, 0, 0, false}, {0, 16, 0, 0, false}, {0, 0, 1024, 0, false}, {0, 0, 0, 1024, false}, {0, -1, 0, 0, false},
	} {
		if got := SalvoFits(c.mx, c.l, c.d, c.s); got != c.ok {
			t.Errorf("SalvoFits(%d,%d,%d,%d) = %v", c.mx, c.l, c.d, c.s, got)
		}
	}
}

func TestParseSalvoAcks(t *testing.T) {
	ev, err := Parse(ConnectOnGoAck(0, 3, 700, 1000, 127))
	if err != nil || ev.Salvo == nil || ev.Salvo.Cmd != CmdConnectOnGoAck || ev.Salvo.Staged() != (Staged{0, 3, 700, 1000, 127}) || ev.Salvo.Status != -1 {
		t.Fatalf("122 %+v %v", ev.Salvo, err)
	}
	if len(ev.Crosspoints) != 0 {
		t.Fatal("a staged crosspoint is not a route")
	}
	ev, err = Parse(GoDoneAck(GoNothing, 127))
	if err != nil || ev.Salvo == nil || ev.Salvo.Cmd != CmdGoDoneAck || ev.Salvo.Status != GoNothing || ev.Salvo.Group != 127 {
		t.Fatalf("123 %+v %v", ev.Salvo, err)
	}
	if _, err := Parse([]byte{CmdGoDoneAck, 0}); err == nil {
		t.Fatal("short 123 parsed")
	}
	if _, err := Parse([]byte{CmdConnectOnGoAck, 0, 0, 0}); err == nil {
		t.Fatal("short 122 parsed")
	}
}

func TestDescribeSalvo(t *testing.T) {
	r := DescribeTx(ConnectOnGo(0, 4, 300, 12, 127))
	if r.Kind != "salvoStage" || r.Level != 4 || r.Dst != 300 || r.Src != 12 || r.Group != 127 {
		t.Fatalf("stage %+v", r)
	}
	r = DescribeTx(GoSalvo(false, 127))
	if r.Kind != "salvoGo" || r.Clear || r.Group != 127 || r.Dst != -1 || r.Level != -1 {
		t.Fatalf("go %+v", r)
	}
	if r = DescribeTx(GoSalvo(true, 9)); r.Kind != "salvoGo" || !r.Clear || r.Group != 9 {
		t.Fatalf("clear %+v", r)
	}
	if r = DescribeTx(ExtConnect(0, 1, 2, 3)); r.Group != -1 {
		t.Fatalf("connect group %+v", r)
	}
}
