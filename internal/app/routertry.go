package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// The Address field of Settings > Routers tries a typed address before Save (ALPHA-BUGS AB-23). The page sends
// `routerTry {id, try: {address, matrix, nameRequests, nameLength}}` after a pause in typing. The server dials the address
// on a connection of its own, apart from every Router link, and reads the source and destination names. A TCP connect
// alone is no proof, since a wrong port can accept it. Each page has one try at a time: the next routerTry cancels the
// one that runs, and a blank address only cancels. The reply `routerTryResult {id, ok, state, sources, destinations,
// err}` has state connected (names came back), failed (err says why) or cancelled. The try writes no Communication Log
// rows: it is not the router's link, and each pause in typing would add rows.

// routerTryReq: the Connection values of the router in view, as the page has them now (saved or not).
type routerTryReq struct {
	Address      string `json:"address"`
	Matrix       int    `json:"matrix"`
	NameRequests string `json:"nameRequests"`
	NameLength   int    `json:"nameLength"`
}

type routerTryResult struct {
	opResult
	State        string `json:"state,omitempty"`
	Sources      int    `json:"sources"`
	Destinations int    `json:"destinations"`
}

const (
	tryConnected = "connected"
	tryFailed    = "failed"
	tryCancelled = "cancelled"
)

// tryHold: the try that runs for one page, so the next one can cancel it.
type tryHold struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	n      int
}

// start cancels the try that runs and gives the context of the next one; done ends it when it is still the newest.
func (h *tryHold) start() (ctx context.Context, done func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		h.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.n++
	n := h.n
	h.cancel = cancel
	return ctx, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		cancel()
		if h.n == n {
			h.cancel = nil
		}
	}
}

func (h *tryHold) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}

var (
	tryTimeout = 6 * time.Second // a dial that hangs times out after 5 s (swp08.NewClient)
	tryQuiet   = namesQuiet      // the names are all in when they stop for this long
)

var (
	errTryNoAnswer = errors.New("no answer from the router: check the address and the port")
	errTryNoNames  = errors.New("the router answers but sent no names: check Matrix and Name requests")
)

func (a *App) serveRouterTry(c *client, m *inMsg) {
	var req routerTryReq
	if m.Try != nil {
		req = *m.Try
	}
	ctx, done := c.try.start()
	defer done()
	res := routerTryResult{opResult: opResult{T: "routerTryResult", OK: true, ID: &m.ID}}
	if strings.TrimSpace(req.Address) == "" {
		res.State = tryCancelled
		a.reply(c, res)
		return
	}
	srcs, dsts, err := tryRouter(ctx, req)
	switch {
	case ctx.Err() != nil:
		res.State = tryCancelled
	case err != nil:
		res.State, res.Err = tryFailed, err.Error()
	default:
		res.State, res.Sources, res.Destinations = tryConnected, srcs, dsts
	}
	a.reply(c, res)
}

// tryRouter dials req.Address with a link of its own, asks for the source and destination names and counts them. It
// ends when the names stop, on the first failure to connect, or after tryTimeout.
func tryRouter(parent context.Context, req routerTryReq) (srcs, dsts int, err error) {
	addr := strings.TrimSpace(req.Address)
	code, err := swp08.NameLenCode(req.NameLength)
	if err != nil {
		return 0, 0, err
	}
	ext := req.NameRequests == "ext"
	if req.Matrix < 0 || req.Matrix > 255 || (!ext && req.Matrix > 15) {
		return 0, 0, errors.New("the Matrix does not suit the Name requests")
	}
	ctx, cancel := context.WithTimeout(parent, tryTimeout)
	defer cancel()

	var mu sync.Mutex
	names := map[bool]map[int]bool{false: {}, true: {}}
	var acked bool
	var failed error
	last := make(chan struct{}, 1)
	poke := func() {
		select {
		case last <- struct{}{}:
		default:
		}
	}
	// No probe: the name requests are the test, and a probe would hold Up back until the router answered it.
	sw := swp08.NewClient(addr, nil)
	sw.Name = func() string { return "address try " + addr }
	sw.OnUp = func() {
		for _, dest := range []bool{false, true} {
			sw.Send(swp08.NamesReq(dest, ext, req.Matrix, 0, code), swp08.Why{Reason: swp08.ReasonNames})
		}
	}
	sw.OnTx = func(e swp08.TxEvent) {
		if e.Result == swp08.TxAck {
			mu.Lock()
			acked = true
			mu.Unlock()
		}
	}
	sw.OnMsg = func(m []byte) {
		ev, err := swp08.Parse(m)
		if err != nil || ev.Names == nil || ev.Names.Matrix != req.Matrix {
			return
		}
		mu.Lock()
		for i := range ev.Names.Names {
			names[ev.Names.Dest][ev.Names.First+i] = true
		}
		mu.Unlock()
		poke()
	}
	sw.OnState = func(s swp08.State, err error) {
		if s == swp08.StateDown && err != nil {
			mu.Lock()
			if failed == nil {
				failed = err
			}
			mu.Unlock()
			poke()
		}
	}
	runCtx, stop := context.WithCancel(ctx)
	ran := make(chan struct{})
	go func() { defer close(ran); sw.Run(runCtx) }()
	defer func() { stop(); <-ran }()

	quiet := time.NewTimer(time.Hour)
	defer quiet.Stop()
	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			defer mu.Unlock()
			if parent.Err() != nil {
				return 0, 0, parent.Err()
			}
			if len(names[false])+len(names[true]) > 0 {
				return len(names[false]), len(names[true]), nil
			}
			if acked {
				return 0, 0, errTryNoNames
			}
			return 0, 0, errTryNoAnswer
		case <-last:
			mu.Lock()
			f, got := failed, len(names[false])+len(names[true])
			mu.Unlock()
			if got == 0 && f != nil {
				return 0, 0, f
			}
			quiet.Reset(tryQuiet)
		case <-quiet.C:
			mu.Lock()
			defer mu.Unlock()
			return len(names[false]), len(names[true]), nil
		}
	}
}
