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

// fastTimers shortens the client's timings for the test.
func fastTimers(t *testing.T) {
	t.Helper()
	ack, probe, dead := ackTimeout, probeIdle, deadIdle
	ackTimeout, probeIdle, deadIdle = 50*time.Millisecond, 300*time.Millisecond, time.Second
	t.Cleanup(func() { ackTimeout, probeIdle, deadIdle = ack, probe, dead })
}

// eventually polls cond for up to two seconds.
func eventually(cond func() bool) bool {
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return true
		}
	}
	return cond()
}

func runClient(t *testing.T, addr string, onUp func(*Client)) *stateLog {
	t.Helper()
	fastTimers(t)
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
	if !eventually(func() bool { return log.has(StateDown) }) {
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
	if !eventually(func() bool { return log.has(StateUp) }) {
		t.Fatal("router never came up")
	}
	// The ignored send's tries, its probe, and a couple of idle probes.
	time.Sleep(3*ackTimeout + 3*probeIdle)
	if log.has(StateDown) {
		t.Fatal("link dropped over one ignored command")
	}
}

// Until the router first answers, Send refuses rather than queueing behind
// a probe that may never be answered.
func TestSendRefusedUntilHeard(t *testing.T) {
	addr := fakeRouter(t, func([]byte) bool { return false })
	fastTimers(t)
	c := NewClient(addr, ExtInterrogate(0, 0, 0))
	var log stateLog
	c.OnState = log.add
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go c.Run(ctx)
	if !eventually(func() bool { return log.has(StateConnecting) }) {
		t.Fatal("never started connecting")
	}
	time.Sleep(20 * time.Millisecond) // connected, waiting on the probe
	if err := c.Send([]byte{0x7F}); err != ErrDown {
		t.Fatalf("Send before the first reply = %v, want ErrDown", err)
	}
}

// New settings reconnect without a down state, so the browser shows no
// router-lost alarm for a deliberate change.
func TestRetargetIsNotDown(t *testing.T) {
	addr := fakeRouter(t, func([]byte) bool { return true })
	fastTimers(t)
	c := NewClient(addr, ExtInterrogate(0, 0, 0))
	var log stateLog
	c.OnState = log.add
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go c.Run(ctx)
	if !eventually(func() bool { return log.has(StateUp) }) {
		t.Fatal("router never came up")
	}
	c.SetTarget(addr, ExtInterrogate(0, 0, 0))
	time.Sleep(200 * time.Millisecond)
	if log.has(StateDown) {
		t.Fatal("retarget reported the router down")
	}
}
