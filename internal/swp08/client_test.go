package swp08

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeRouter accepts connections and ACKs only the messages answer picks.
// The connection stays open either way, like a hung router's would.
func fakeRouter(t *testing.T, answer func(msg []byte) bool) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				var d Decoder
				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					d.Feed(buf[:n], func(f Frame) {
						if f.Kind == KindMsg && answer(f.Msg) {
							conn.Write(AckFrame)
						}
					})
				}
			}()
		}
	}()
	return ln.Addr().String()
}

type stateLog struct {
	mu  sync.Mutex
	got []State
}

func (l *stateLog) add(s State, _ error) {
	l.mu.Lock()
	l.got = append(l.got, s)
	l.mu.Unlock()
}

func (l *stateLog) has(s State) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, g := range l.got {
		if g == s {
			return true
		}
	}
	return false
}

func runClient(t *testing.T, addr string, onUp func(*Client)) *stateLog {
	t.Helper()
	c := NewClient(addr, ExtInterrogate(0, 0, 0))
	var log stateLog
	c.OnState = log.add
	if onUp != nil {
		c.OnUp = func() { onUp(c) }
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go c.Run(ctx)
	return &log
}

// A router that takes the TCP connection but never answers is never shown
// as up, and the session is dropped.
func TestSilentRouterNeverUp(t *testing.T) {
	addr := fakeRouter(t, func([]byte) bool { return false })
	log := runClient(t, addr, nil)
	deadline := time.Now().Add(10 * time.Second)
	for !log.has(StateDown) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if !log.has(StateDown) {
		t.Fatal("silent router never dropped")
	}
	if log.has(StateUp) {
		t.Fatal("silent router was reported up")
	}
}

// A router that ignores one command but answers the probe stays up.
func TestIgnoredCommandKeepsLink(t *testing.T) {
	addr := fakeRouter(t, func(m []byte) bool { return m[0] == CmdExtInterrogate })
	log := runClient(t, addr, func(c *Client) { c.Send([]byte{0x7F, 0x00}) })
	time.Sleep(8 * time.Second) // the ignored send (3s) plus its probe, with room
	if !log.has(StateUp) {
		t.Fatal("router never came up")
	}
	if log.has(StateDown) {
		t.Fatal("link dropped over one ignored command")
	}
}
