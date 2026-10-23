package swp08

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrDown = errors.New("router not connected")
	ErrBusy = errors.New("router send queue full")
)

// Timings; vars so tests can shorten them.
var (
	ackTimeout = time.Second
	probeIdle  = 10 * time.Second // send the probe after this long without rx
	// deadIdle drops a quiet connection, for a client with no probe; with
	// one, an unanswered probe drops it first.
	deadIdle = 20 * time.Second
)

const (
	sendTries   = 3
	maxBackoff  = 10 * time.Second
	stableAfter = 30 * time.Second // a session this long resets the backoff
)

type State string

const (
	StateConnecting State = "connecting"
	StateUp         State = "up"
	StateDown       State = "down"
)

// Client keeps one TCP connection to a router alive and serializes sends,
// waiting for each ACK before the next message as SW-P-08 requires.
type Client struct {
	Debug bool
	// OnUp runs once per connection, after Send is usable.
	OnUp func()
	// OnMsg runs on the reader goroutine for each valid message; keep it quick.
	OnMsg func(msg []byte)
	// OnState reports connection changes; err explains a drop.
	OnState func(s State, err error)

	mu     sync.Mutex
	addr   string
	probe  []byte             // sent when the link is quiet; must provoke a reply
	q      chan []byte        // nil while disconnected
	cancel context.CancelFunc // ends the current session
	kick   chan struct{}

	wmu sync.Mutex
}

var (
	errNoRouter = errors.New("no router configured")
	errRetarget = errors.New("reconnecting with new settings")
)

func NewClient(addr string, probe []byte) *Client {
	return &Client{addr: addr, probe: probe, kick: make(chan struct{}, 1)}
}

// SetTarget switches router and reconnects at once. An empty addr idles.
func (c *Client) SetTarget(addr string, probe []byte) {
	c.mu.Lock()
	c.addr, c.probe = addr, probe
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

func (c *Client) target() (string, []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.addr, c.probe
}

// Send queues a message. Messages queued on a connection that drops are
// discarded, never replayed on the next one: a stale take must not fire later.
func (c *Client) Send(msg []byte) error {
	c.mu.Lock()
	q := c.q
	c.mu.Unlock()
	if q == nil {
		return ErrDown
	}
	select {
	case q <- msg:
		return nil
	default:
		return ErrBusy
	}
}

func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		select { // a kick from before this attempt is already honored
		case <-c.kick:
		default:
		}
		addr, _ := c.target()
		if addr == "" {
			c.state(StateDown, errNoRouter)
			select {
			case <-ctx.Done():
				return
			case <-c.kick:
			}
			continue
		}
		c.state(StateConnecting, nil)
		start := time.Now()
		err := c.session(ctx, addr)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errRetarget) {
			// New settings, not a failure: straight back to connecting.
			log.Printf("swp08: %s: %v", addr, err)
			backoff = time.Second
			continue
		}
		c.state(StateDown, err)
		if time.Since(start) > stableAfter {
			backoff = time.Second
		}
		log.Printf("swp08: %s: %v (retry in %s)", addr, err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-c.kick: // new target: retry now
			backoff = time.Second
			continue
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (c *Client) state(s State, err error) {
	if c.OnState != nil {
		c.OnState(s, err)
	}
}

func (c *Client) session(ctx context.Context, addr string) error {
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	d := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 10 * time.Second}
	conn, err := d.DialContext(sctx, "tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("swp08: connected to %s", addr)
	go func() {
		<-sctx.Done()
		conn.Close()
	}()

	q := make(chan []byte, 4096)
	acks := make(chan bool, 1)
	rx := newLastRx()

	defer func() {
		c.mu.Lock()
		c.q = nil
		c.cancel = nil
		c.mu.Unlock()
	}()

	errc := make(chan error, 2)
	go func() { errc <- c.reader(conn, acks, rx) }()
	go func() { errc <- c.writer(sctx, conn, q, acks, rx) }()

	// wait blocks until ch is closed (nil: forever) or the session ends;
	// nil means ch closed.
	wait := func(ch <-chan struct{}) error {
		select {
		case <-ch:
			return nil
		case err := <-errc:
			return err
		case <-sctx.Done():
			if ctx.Err() == nil {
				return errRetarget
			}
			return ctx.Err()
		}
	}

	// Up only once the router answers: a hung one still accepts TCP. The
	// probe provokes that first reply. Until then Send says ErrDown, so
	// nothing else queues behind a probe that may never be answered.
	if _, probe := c.target(); probe != nil {
		q <- probe
		if err := wait(rx.heard); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.q = q
	c.mu.Unlock()
	c.state(StateUp, nil)
	if c.OnUp != nil {
		c.OnUp()
	}
	return wait(nil)
}

func (c *Client) write(conn net.Conn, b []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.Debug {
		log.Printf("swp08: tx % X", b)
	}
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err := conn.Write(b)
	return err
}

// lastRx tracks when the router last sent anything, as a monotonic offset
// from the session's start, so a wall-clock step can't fake or hide a reply.
type lastRx struct {
	start time.Time
	at    atomic.Int64  // time.Duration since start of the latest rx
	heard chan struct{} // closed on the first rx
	once  sync.Once
}

func newLastRx() *lastRx {
	return &lastRx{start: time.Now(), heard: make(chan struct{})}
}

func (l *lastRx) now() time.Duration { return time.Since(l.start) }

func (l *lastRx) mark() {
	l.at.Store(int64(l.now()))
	l.once.Do(func() { close(l.heard) })
}

// since reports whether anything arrived after t (from now()).
func (l *lastRx) since(t time.Duration) bool { return time.Duration(l.at.Load()) > t }

func (l *lastRx) idle() time.Duration { return l.now() - time.Duration(l.at.Load()) }

func (c *Client) reader(conn net.Conn, acks chan bool, rx *lastRx) error {
	var dec Decoder
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		rx.mark()
		if c.Debug {
			log.Printf("swp08: rx % X", buf[:n])
		}
		var werr error
		dec.Feed(buf[:n], func(f Frame) {
			switch f.Kind {
			case KindAck, KindNak:
				select {
				case acks <- f.Kind == KindAck:
				default:
				}
			case KindBad:
				log.Printf("swp08: %v, sending NAK", f.Err)
				werr = c.write(conn, NakFrame)
			case KindMsg:
				if f.Err != nil && c.Debug {
					log.Printf("swp08: %v (accepted)", f.Err)
				}
				werr = c.write(conn, AckFrame)
				if c.OnMsg != nil {
					c.OnMsg(f.Msg)
				}
			}
		})
		if werr != nil {
			return werr
		}
	}
}

// writer sends queued messages one at a time and keeps the link checked.
// The router going quiet with the TCP session still open (powered off,
// cable pulled, hung) shows up as a probe that gets nothing back at all,
// and drops the connection so it shows as down and reconnects. A message
// that alone goes unanswered (a command this router ignores) only costs a
// probe: if that's answered, the router is fine.
func (c *Client) writer(ctx context.Context, conn net.Conn, q chan []byte, acks chan bool, rx *lastRx) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	errSilent := errors.New("no reply from router")
	check := func() error {
		_, probe := c.target()
		if probe == nil {
			return nil
		}
		heard, err := c.transmit(ctx, conn, probe, acks, rx)
		if err == nil && !heard {
			err = errSilent
		}
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m := <-q:
			heard, err := c.transmit(ctx, conn, m, acks, rx)
			if err != nil {
				return err
			}
			if !heard {
				if _, probe := c.target(); bytes.Equal(m, probe) {
					return errSilent // that was the probe
				}
				if err := check(); err != nil {
					return err
				}
			}
		case <-tick.C:
			idle := rx.idle()
			if idle > deadIdle {
				return fmt.Errorf("no data from router for %s", idle.Round(time.Second))
			}
			if idle > probeIdle {
				if err := check(); err != nil {
					return err
				}
			}
		}
	}
}

// transmit sends m and waits for its ACK, retrying on NAK or silence.
// heard reports whether anything at all came back from the router while
// it tried.
func (c *Client) transmit(ctx context.Context, conn net.Conn, m []byte, acks chan bool, rx *lastRx) (heard bool, err error) {
	frame := Encode(m)
	sent := rx.now()
	for try := 1; try <= sendTries; try++ {
		select { // discard any stray ACK left from before
		case <-acks:
		default:
		}
		if err := c.write(conn, frame); err != nil {
			return false, err
		}
		select {
		case ok := <-acks:
			if ok {
				return true, nil
			}
			log.Printf("swp08: NAK for cmd 0x%02X (try %d/%d)", m[0], try, sendTries)
		case <-time.After(ackTimeout):
			log.Printf("swp08: no ACK for cmd 0x%02X (try %d/%d)", m[0], try, sendTries)
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	log.Printf("swp08: gave up on cmd 0x%02X after %d tries", m[0], sendTries)
	return rx.since(sent), nil
}
