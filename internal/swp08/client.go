package swp08

import (
	"context"
	"errors"
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

const (
	ackTimeout  = time.Second
	probeIdle   = 10 * time.Second // send the probe after this long without rx
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

type Reason string

const (
	ReasonTake      Reason = "take"
	ReasonUndo      Reason = "undo"
	ReasonRetry     Reason = "retry"
	ReasonCheck     Reason = "check"
	ReasonConfirm   Reason = "confirm"
	ReasonResync    Reason = "resync"
	ReasonNames     Reason = "names"
	ReasonKeepalive Reason = "keepalive"
)

type Why struct {
	Reason Reason
	By     string
	// Note goes in the Communication Log row's Note, after any of its own (a panel page's tag).
	Note string
	// Done, if set, is closed once the message's fate is known, so callers count timeouts from when the router could have seen it.
	Done chan struct{}
}

const (
	TxAck     = "ack"
	TxNak     = "nak"
	TxNoAck   = "noack" // written, nothing back; gave up
	TxWrite   = "write"
	TxDropped = "dropped" // queued, but the connection ended before it was written
)

type TxEvent struct {
	Msg    []byte
	Why    Why
	At     time.Time // first write (when queued, for dropped)
	Tries  int
	Result string
	Err    string
	Ack    time.Duration // from the last write to its ACK; 0 when there was none
}

type out struct {
	msg []byte
	why Why
	at  time.Time
}

type Client struct {
	Debug bool
	OnUp  func()
	// OnMsg runs on the reader goroutine for each valid message; keep it quick.
	OnMsg   func(msg []byte)
	OnState func(s State, err error)
	// OnTx runs on the writer goroutine, which waits for ACKs: it must not block.
	OnTx func(TxEvent)

	ackTimeout, probeIdle time.Duration
	dial                  func(ctx context.Context, addr string) (net.Conn, error)

	mu     sync.Mutex
	addr   string
	probe  []byte   // sent on connect and when the link is quiet; must provoke a reply
	q      chan out // nil until the router has answered
	cancel context.CancelFunc
	kick   chan struct{}

	wmu sync.Mutex
}

var (
	errNoRouter = errors.New("no router configured")
	errRetarget = errors.New("reconnecting with new settings")
)

func NewClient(addr string, probe []byte) *Client {
	d := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 10 * time.Second}
	return &Client{addr: addr, probe: probe, kick: make(chan struct{}, 1),
		ackTimeout: ackTimeout, probeIdle: probeIdle,
		dial: func(ctx context.Context, addr string) (net.Conn, error) { return d.DialContext(ctx, "tcp", addr) }}
}

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

// Messages queued on a connection that drops are discarded, never replayed: a stale take must not fire later.
// The push happens under mu, so once a session has cleared c.q nothing more lands in its queue.
func (c *Client) Send(msg []byte, why Why) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.q == nil {
		return ErrDown
	}
	select {
	case c.q <- out{msg, why, time.Now()}:
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
		case <-c.kick:
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

// A SetTarget at any point (mid-dial too) returns errRetarget whatever error surfaced: a reconnect, not a failure.
func (c *Client) session(ctx context.Context, addr string) (err error) {
	sctx, cancel := context.WithCancel(ctx)
	var q chan out
	var wdone chan struct{}
	defer func() {
		// A retarget is told by sctx ending first, so before cancel.
		if ctx.Err() == nil && sctx.Err() != nil {
			err = errRetarget
		}
		c.mu.Lock()
		c.q = nil
		c.cancel = nil
		c.mu.Unlock()
		cancel()
		if wdone != nil {
			<-wdone
		}
		// Whatever the writer didn't get to is never sent.
		for q != nil {
			select {
			case m := <-q:
				c.tx(TxEvent{Msg: m.msg, Why: m.why, At: m.at, Result: TxDropped})
			default:
				q = nil
			}
		}
	}()
	c.mu.Lock()
	c.cancel = cancel
	stale := c.addr != addr // SetTarget ran before this session could be cancelled
	c.mu.Unlock()
	if stale {
		return errRetarget
	}

	conn, err := c.dial(sctx, addr)
	if err != nil {
		return err
	}
	log.Printf("swp08: connected to %s", addr)
	go func() {
		<-sctx.Done()
		conn.Close()
	}()

	q = make(chan out, 4096)
	acks := make(chan bool, 1)
	rx := newLastRx()

	errc := make(chan error, 2)
	go func() { errc <- c.reader(conn, acks, rx) }()
	wdone = make(chan struct{})
	go func() { errc <- c.writer(sctx, conn, q, acks, rx); close(wdone) }()

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

	// Up only once the router answers (a hung one still accepts TCP); until then Send says ErrDown.
	if _, probe := c.target(); probe != nil {
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

func (c *Client) tx(e TxEvent) {
	if c.OnTx != nil {
		c.OnTx(e)
	}
	if e.Why.Done != nil {
		close(e.Why.Done)
	}
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

// Monotonic offset from the session's start, so a wall-clock step can't fake or hide a reply.
type lastRx struct {
	start time.Time
	at    atomic.Int64
	heard chan struct{}
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

// A probe with no reply at all (router off, cable pulled, hung) drops the connection; one unanswered
// message only costs a probe.
func (c *Client) writer(ctx context.Context, conn net.Conn, q chan out, acks chan bool, rx *lastRx) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	check := func() error {
		_, probe := c.target()
		if probe == nil {
			return nil
		}
		heard, err := c.transmit(ctx, conn, out{msg: probe, why: Why{Reason: ReasonKeepalive}}, acks, rx)
		if err == nil && !heard {
			err = errors.New("no reply from router")
		}
		return err
	}
	// The first probe, sent at once: the session waits for its reply.
	if err := check(); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m := <-q:
			if ctx.Err() != nil { // picked over Done: it won't be written
				c.tx(TxEvent{Msg: m.msg, Why: m.why, At: m.at, Result: TxDropped})
				return ctx.Err()
			}
			heard, err := c.transmit(ctx, conn, m, acks, rx)
			if err != nil {
				return err
			}
			if !heard {
				if err := check(); err != nil {
					return err
				}
			}
		case <-tick.C:
			if rx.idle() > c.probeIdle {
				if err := check(); err != nil {
					return err
				}
			}
		}
	}
}

func (c *Client) transmit(ctx context.Context, conn net.Conn, o out, acks chan bool, rx *lastRx) (heard bool, err error) {
	m := o.msg
	frame := Encode(m)
	sent := rx.now()
	e := TxEvent{Msg: m, Why: o.why, At: time.Now(), Result: TxNoAck}
	defer func() { c.tx(e) }()
	for try := 1; try <= sendTries; try++ {
		select {
		case <-acks:
		default:
		}
		e.Tries = try
		wrote := time.Now()
		if err := c.write(conn, frame); err != nil {
			e.Result, e.Err = TxWrite, err.Error()
			return false, err
		}
		select {
		case ok := <-acks:
			if ok {
				e.Result = TxAck
				e.Ack = time.Since(wrote)
				return true, nil
			}
			e.Result = TxNak
			log.Printf("swp08: NAK for cmd 0x%02X (try %d/%d)", m[0], try, sendTries)
		case <-time.After(c.ackTimeout):
			e.Result = TxNoAck
			log.Printf("swp08: no ACK for cmd 0x%02X (try %d/%d)", m[0], try, sendTries)
		case <-ctx.Done():
			e.Result = TxNoAck
			return false, ctx.Err()
		}
	}
	log.Printf("swp08: gave up on cmd 0x%02X after %d tries", m[0], sendTries)
	return rx.since(sent), nil
}
