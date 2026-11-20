package main

import (
	"bytes"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

type simConn struct {
	t   *testing.T
	c   net.Conn
	dec swp08.Decoder
	got [][]byte
}

func startSim(t *testing.T) (dial func() *simConn) {
	t.Helper()
	s := newSim(20, 30, 16)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return func() *simConn {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return &simConn{t: t, c: c}
	}
}

func (sc *simConn) send(msg []byte) {
	sc.t.Helper()
	if _, err := sc.c.Write(swp08.Encode(msg)); err != nil {
		sc.t.Fatal(err)
	}
}

// next returns the next message, skipping ACKs, or nil if none comes within wait.
func (sc *simConn) next(wait time.Duration) []byte {
	sc.t.Helper()
	deadline := time.Now().Add(wait)
	buf := make([]byte, 1024)
	for len(sc.got) == 0 {
		sc.c.SetReadDeadline(deadline)
		n, err := sc.c.Read(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return nil
		}
		if err != nil {
			sc.t.Fatal(err)
		}
		sc.dec.Feed(buf[:n], func(f swp08.Frame) {
			if f.Kind == swp08.KindMsg {
				sc.got = append(sc.got, f.Msg)
			}
		})
	}
	m := sc.got[0]
	sc.got = sc.got[1:]
	return m
}

func (sc *simConn) want(desc string, msg []byte) {
	sc.t.Helper()
	if got := sc.next(time.Second); !bytes.Equal(got, msg) {
		sc.t.Fatalf("%s: got % X, want % X", desc, got, msg)
	}
}

// The sim answers protect messages in the Ultrix's 7-byte order and sends 0x8D and 0x8F only to clients that have
// sent a message.
func TestSimProtectMessagesLikeUltrix(t *testing.T) {
	dial := startSim(t)
	a, quiet, other := dial(), dial(), dial()

	a.send([]byte{0x8A, 0x00, 0x00, 0x00, 0x0C})
	a.want("0x8B for dst 12", []byte{0x8B, 0x00, 0x00, 0x02, 0x00, 0x0C, 0x00, 0x09})
	other.send([]byte{0x8A, 0x00, 0x10, 0x00, 0x1A})
	other.want("0x8B for dst 26 level 16", []byte{0x8B, 0x00, 0x10, 0x00, 0x00, 0x1A, 0x00, 0x00})

	a.send([]byte{0x8C, 0x00, 0x00, 0x00, 0x1A, 0x00, 0x7E})
	set := []byte{0x8D, 0x00, 0x00, 0x01, 0x00, 0x1A, 0x00, 0x7E}
	a.want("0x8D to the sender", set)
	other.want("0x8D to a client that has sent", set)
	if m := quiet.next(200 * time.Millisecond); m != nil {
		t.Fatalf("a client that sent nothing got % X", m)
	}

	// Only the device that protected the level can clear it.
	a.send([]byte{0x8E, 0x00, 0x00, 0x00, 0x1A, 0x00, 0x05})
	a.send([]byte{0x8A, 0x00, 0x00, 0x00, 0x1A})
	a.want("0x8B after a clear from another device", []byte{0x8B, 0x00, 0x00, 0x01, 0x00, 0x1A, 0x00, 0x7E})
	if m := other.next(100 * time.Millisecond); m != nil {
		t.Fatalf("a clear from another device sent % X", m)
	}

	a.send([]byte{0x93, 0x00, 0x00, 0x00, 0x00})
	dump, err := swp08.Parse(a.next(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if p := dump.Protects[26]; !p.Protected || p.Device != 126 {
		t.Fatalf("dump dst 26: %+v", p)
	}

	a.send([]byte{0x8E, 0x00, 0x00, 0x00, 0x1A, 0x00, 0x7E})
	cleared := []byte{0x8F, 0x00, 0x00, 0x00, 0x00, 0x1A, 0x00, 0x00}
	a.want("0x8F to the sender", cleared)
	other.want("0x8F to a client that has sent", cleared)
	if m := quiet.next(100 * time.Millisecond); m != nil {
		t.Fatalf("a client that sent nothing got % X", m)
	}
}

// A connect to a Protected level gets an ACK and no other reply, also from the session that set the Protect; the
// route stays. The other levels connect.
func TestSimRefusesConnectToProtectedLevel(t *testing.T) {
	dial := startSim(t)
	a := dial()
	a.send(swp08.ExtProtectConnect(0, 0, 3, 126))
	a.want("0x8D", swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 0, 0, 1, 3, 126))
	a.send(swp08.ExtConnect(0, 0, 3, 7))
	if m := a.next(150 * time.Millisecond); m != nil {
		t.Fatalf("a connect to a Protected level got % X", m)
	}
	a.send(swp08.ExtInterrogate(0, 0, 3))
	a.want("the old route", swp08.ExtTally(0, 0, 3, 3))
	a.send(swp08.ExtConnect(0, 1, 3, 7))
	a.want("level 1 is free", swp08.ExtConnected(0, 1, 3, 7))
}

func TestSimProtectsFromEnv(t *testing.T) {
	s := newSim(20, 30, 2)
	for _, f := range []string{"20:1:112", "12:0:0", "bad", "5:4:1"} {
		if dst, kind, dev, ok := protectSpec(f); ok {
			s.holdAll(dst, kind, dev)
		}
	}
	if h := s.protect[[2]int{2, 20}]; h != (hold{details: 1, device: 112}) {
		t.Fatalf("dst 20 level 2: %+v", h)
	}
	if s.held(0, 12) || s.held(0, 5) {
		t.Fatal("kind 0 must clear dst 12, and kind 4 is not a kind")
	}
}

// Not proved on the Ultrix: the sim's Go sets the free crosspoints and leaves the Protected ones.
func TestSimGoLeavesProtectedCrosspoints(t *testing.T) {
	dial := startSim(t)
	a := dial()
	a.send(swp08.ExtProtectConnect(0, 0, 4, 126))
	a.next(time.Second)
	a.send(swp08.ConnectOnGo(0, 0, 4, 9, 127))
	a.next(time.Second)
	a.send(swp08.ConnectOnGo(0, 0, 5, 9, 127))
	a.next(time.Second)
	a.send(swp08.GoSalvo(false, 127))
	a.want("Go done", swp08.GoDoneAck(swp08.GoSet, 127))
	a.send(swp08.ExtInterrogate(0, 0, 4))
	a.want("dst 4 kept", swp08.ExtTally(0, 0, 4, 4))
	a.send(swp08.ExtInterrogate(0, 0, 5))
	a.want("dst 5 set", swp08.ExtTally(0, 0, 5, 9))
}
