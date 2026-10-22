package swp08

import (
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

const (
	ackTimeout  = time.Second
	sendTries   = 3
	probeIdle   = 15 * time.Second // send Probe after this long without rx
	deadIdle    = 30 * time.Second // drop the connection after this long without rx
	maxBackoff  = 10 * time.Second
	stableAfter = 30 * time.Second // a session this long resets the backoff
)

type State string

const (
	StateConnecting State = "connecting"
	StateUp         State = "up"
	StateDown       State = "down"
)

// Client keeps one TCP connection to a router alive and serialises sends,
// waiting for each ACK before the next message as SW-P-08 requires.
type Client struct {
	Addr  string
	Debug bool
	// Probe is sent when the link has been quiet; it must provoke a reply.
	Probe []byte
	// OnUp runs once per connection, after Send is usable.
	OnUp func()
	// OnMsg runs on the reader goroutine for each valid message; keep it quick.
	OnMsg func(msg []byte)
	// OnState reports connection changes; err explains a drop.
	OnState func(s State, err error)

	mu sync.Mutex
	q  chan []byte // nil while disconnected

	wmu sync.Mutex
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
		c.state(StateConnecting, nil)
		start := time.Now()
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		c.state(StateDown, err)
		log.Printf("swp08: %s: %v (retry in %s)", c.Addr, err, backoff)
		if time.Since(start) > stableAfter {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
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

func (c *Client) session(ctx context.Context) error {
	d := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return err
	}
	log.Printf("swp08: connected to %s", c.Addr)

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-sctx.Done()
		conn.Close()
	}()

	q := make(chan []byte, 4096)
	acks := make(chan bool, 1)
	var rx atomic.Int64
	rx.Store(time.Now().UnixNano())

	c.mu.Lock()
	c.q = q
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.q = nil
		c.mu.Unlock()
	}()

	errc := make(chan error, 2)
	go func() { errc <- c.reader(conn, acks, &rx) }()
	go func() { errc <- c.writer(sctx, conn, q, acks, &rx) }()

	c.state(StateUp, nil)
	if c.OnUp != nil {
		c.OnUp()
	}
	return <-errc
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

func (c *Client) reader(conn net.Conn, acks chan bool, rx *atomic.Int64) error {
	var dec Decoder
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		rx.Store(time.Now().UnixNano())
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

func (c *Client) writer(ctx context.Context, conn net.Conn, q chan []byte, acks chan bool, rx *atomic.Int64) error {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m := <-q:
			if err := c.transmit(ctx, conn, m, acks); err != nil {
				return err
			}
		case <-tick.C:
			idle := time.Since(time.Unix(0, rx.Load()))
			if idle > deadIdle {
				return fmt.Errorf("no data from router for %s", idle.Round(time.Second))
			}
			if idle > probeIdle && c.Probe != nil {
				if err := c.transmit(ctx, conn, c.Probe, acks); err != nil {
					return err
				}
			}
		}
	}
}

func (c *Client) transmit(ctx context.Context, conn net.Conn, m []byte, acks chan bool) error {
	frame := Encode(m)
	for try := 1; try <= sendTries; try++ {
		select { // discard any stray ACK left from before
		case <-acks:
		default:
		}
		if err := c.write(conn, frame); err != nil {
			return err
		}
		select {
		case ok := <-acks:
			if ok {
				return nil
			}
			log.Printf("swp08: NAK for cmd 0x%02X (try %d/%d)", m[0], try, sendTries)
		case <-time.After(ackTimeout):
			log.Printf("swp08: no ACK for cmd 0x%02X (try %d/%d)", m[0], try, sendTries)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	log.Printf("swp08: gave up on cmd 0x%02X after %d tries", m[0], sendTries)
	return nil
}
