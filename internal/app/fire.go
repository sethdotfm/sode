package app

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// Native fire mode: a confirm's takes are staged in a SW-P-08 group salvo (120, answered
// 122) and set with one Go (121, answered 123). On the Ultrix (2026-11-02) a Go's
// crosspoints are announced ~4 ms apart vs ~16 ms for ordinary connects; true simultaneity
// is unproven.
// The Ultrix sometimes announces a Go's crosspoints (0x84), sometimes not: a staged level
// has an own key and is confirmed by its Connected, else by the tally of the interrogate
// each take sends after the Go; failure and late handling are the sequential take's. A
// tally-confirmed key lives salvoAnnounceFor after the Go.
// Staging: levels the standard layout carries (wire level 0-15, numbers 0-1023) use 120,
// the rest 248 (extended). A destination level taken twice in a batch is not staged. Fewer
// than two staged takes: sequential throughout.
// The staged list is shared by all clients: fire holds fireMu first clear to last, one
// salvo command at a time, and clears the group before staging and after every fire. A
// failure before the Go (no 122, no 123 for the first clear, link refusal) clears the
// group and sends the batch as ordinary connects. After the Go is sent it is never re-sent
// and never followed by connects; the takes' interrogates say what happened.

// salvoReply: acknowledge wait after the command was ACKed; the Ultrix answers in 5-7 ms.
const salvoReply = time.Second

// salvoAnnounceFor: a Connected for a tally-confirmed staged level still counts this long
// after the Go (Ultrix: 100-340 ms for ~50 crosspoints).
const salvoAnnounceFor = 2 * time.Second

// stageChunk: caps queued stage messages so a big salvo doesn't fill the send queue.
const stageChunk = 64

// fireState collects acknowledges for the fire in progress (a.fire, guarded by mu); others
// are dropped.
type fireState struct {
	acks chan swp08.SalvoAck
}

// Caller holds mu.
func (a *App) onSalvoAck(x swp08.SalvoAck) {
	if a.fire == nil {
		return
	}
	select {
	case a.fire.acks <- x:
	default:
		log.Printf("salvo: acknowledge dropped, collector full (cmd %d)", x.Cmd)
	}
}

func (fs *fireState) await(d time.Duration, ok func(swp08.SalvoAck) bool) (swp08.SalvoAck, bool) {
	t := time.NewTimer(d)
	defer t.Stop()
	for {
		select {
		case x := <-fs.acks:
			if ok(x) {
				return x, true
			}
		case <-t.C:
			return swp08.SalvoAck{}, false
		}
	}
}

// markStages sets Stage per item (a destination level taken twice is staged in neither:
// order would be up to the router) and returns how many items stage something.
func markStages(r *runtime, items []takeReq) int {
	type dl struct{ dst, wire int }
	uses := map[dl]int{}
	for _, it := range items {
		seen := map[int]bool{}
		for _, ui := range it.Levels {
			if l, ok := r.byUI[ui]; ok && !seen[l.Wire] {
				seen[l.Wire] = true
				uses[dl{it.Dst, l.Wire}]++
			}
		}
	}
	n := 0
	for i := range items {
		it := &items[i]
		it.Stage = nil
		for _, ui := range it.Levels {
			l, ok := r.byUI[ui]
			if !ok || !swp08.ExtSalvoFits(r.cfg.Matrix, l.Wire, it.Dst, it.Src) || uses[dl{it.Dst, l.Wire}] != 1 {
				continue
			}
			if it.Stage == nil {
				it.Stage = map[int]bool{}
			}
			it.Stage[l.Wire] = true
		}
		if len(it.Stage) > 0 {
			n++
		}
	}
	return n
}

func (a *App) batchNative(r *runtime, c *client, items []takeReq) {
	runs := make([]*takeRun, len(items))
	runLimited(takeLimit, len(items), func(i int) { runs[i] = a.prepare(r, c, items[i]) })
	var live, staged []*takeRun
	for _, t := range runs {
		if t == nil {
			continue // answered already: bad request or protected
		}
		live = append(live, t)
		if t.w.hasNative() {
			staged = append(staged, t)
		}
	}
	if len(staged) >= 2 {
		start := time.Now()
		sent, err := a.fireSalvo(r, staged)
		n := 0
		for _, t := range staged {
			n += len(t.w.filter(func(l *takeLevel) bool { return l.native }))
		}
		switch {
		case !sent:
			log.Printf("salvo: not fired (%v); sending %d takes one after another", err, len(live))
			a.command(c, "salvo", "fallback", fmt.Sprintf("%d crosspoints; sent one after another", n), err)
		case err != nil:
			log.Printf("salvo: Go sent for %d crosspoints, %v; the takes' interrogates will tell", n, err)
			a.command(c, "salvo", "error", fmt.Sprintf("%d crosspoints", n), err)
		default:
			a.command(c, "salvo", "ok", fmt.Sprintf("%d crosspoints set in %d ms", n, time.Since(start).Milliseconds()), nil)
		}
		if !sent {
			for _, t := range staged {
				a.unNative(t.w)
			}
		}
	} else {
		for _, t := range staged {
			a.unNative(t.w)
		}
	}
	runLimited(takeLimit, len(live), func(i int) { a.complete(r, live[i], a.sendTake(r, live[i].w)) })
	a.logSalvoTiming(c, staged)
}

func (a *App) logSalvoTiming(c *client, staged []*takeRun) {
	a.mu.Lock()
	var goAt, last time.Time
	n, byConn, byTally := 0, 0, 0
	for _, t := range staged {
		if !t.w.goAt.IsZero() {
			goAt = t.w.goAt
		}
		for _, l := range t.w.levels {
			if !l.native {
				continue
			}
			n++
			if !l.confirmed() {
				continue
			}
			if l.tallied() {
				byTally++
			} else {
				byConn++
			}
			if l.at.After(last) {
				last = l.at
			}
		}
	}
	a.mu.Unlock()
	if goAt.IsZero() {
		return // not fired, or a single take: sequential
	}
	note := fmt.Sprintf("%d of %d crosspoints confirmed", byConn+byTally, n)
	if !last.IsZero() {
		note += fmt.Sprintf(", the last %d ms after the Go", max(last.Sub(goAt), 0).Milliseconds())
	}
	note += fmt.Sprintf(" (%d by Connected, %d by interrogate)", byConn, byTally)
	result := "ok"
	if byConn+byTally < n {
		result = "error"
	}
	log.Printf("salvo: %s", note)
	a.command(c, "salvo", result, note, nil)
}

// Only the batch goroutine writes native.
func (w *waiter) hasNative() bool {
	for _, l := range w.levels {
		if l.native {
			return true
		}
	}
	return false
}

func (a *App) unNative(w *waiter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, l := range w.levels {
		l.native = false
	}
	if a.ext == w.ext && !w.cut {
		w.ownKeys(a)
	}
}

var (
	errNoClear = errors.New("the router did not acknowledge clearing the salvo group (no 123)")
	errNoStage = errors.New("the router did not acknowledge every staged crosspoint (no 122 or 250)")
	errNoGo    = errors.New("the router did not acknowledge the Go (no 123)")
)

// fireSalvo: sent reports whether the Go was sent (if not, takes may go as connects); err
// with sent means the Go's acknowledge was missing or odd.
func (a *App) fireSalvo(r *runtime, staged []*takeRun) (sent bool, err error) {
	a.fireMu.Lock()
	defer a.fireMu.Unlock()
	n := 0
	for _, t := range staged {
		n += len(t.w.levels)
	}
	fs := &fireState{acks: make(chan swp08.SalvoAck, n+16)}
	a.mu.Lock()
	a.fire = fs
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.fire = nil
		a.mu.Unlock()
	}()
	mx, g := r.cfg.Matrix, r.cfg.SalvoGroup
	ext := staged[0].w.ext
	why := staged[0].w.why

	// Clear anything staged that isn't ours; this also proves group salvos work.
	if err := a.clearSalvo(ext, fs, g, why); err != nil {
		return false, err
	}
	// Cleared after every fire; on failure the router may still hold some of ours.
	defer func() {
		if err := a.clearSalvo(ext, fs, g, why); err != nil {
			log.Printf("salvo: clearing group %d after the fire: %v", g, err)
		}
	}()

	want := map[swp08.Staged]int{}
	missing := 0
	var last <-chan struct{}
	queued := 0
	for _, t := range staged {
		for _, l := range t.w.levels {
			if !l.native {
				continue
			}
			m := swp08.ConnectOnGo(mx, l.wire, t.w.dst, t.w.src, g)
			if !swp08.SalvoFits(mx, l.wire, t.w.dst, t.w.src) {
				m = swp08.ExtConnectOnGo(mx, l.wire, t.w.dst, t.w.src, g)
			}
			done, err := a.sendTracked(ext, m, t.w.why)
			if err != nil {
				return false, fmt.Errorf("staging: %w", err)
			}
			x, _ := swp08.StagedOf(m)
			want[x]++
			missing++
			last = done
			if queued++; queued%stageChunk == 0 {
				leftQueue(last)
			}
		}
	}
	leftQueue(last)
	// Every crosspoint must be staged before the Go.
	if missing > 0 {
		if _, ok := fs.await(salvoReply, func(x swp08.SalvoAck) bool {
			if (x.Cmd != swp08.CmdConnectOnGoAck && x.Cmd != swp08.CmdExtConnectOnGoAck) || want[x.Staged()] == 0 {
				return false
			}
			want[x.Staged()]--
			missing--
			return missing == 0
		}); !ok {
			return false, fmt.Errorf("%w: %d of %d", errNoStage, missing, queued)
		}
	}

	done, err := a.sendTracked(ext, swp08.GoSalvo(false, g), why)
	if err != nil {
		return false, fmt.Errorf("go: %w", err)
	}
	// From here the routes may have changed: never again as connects.
	leftQueue(done)
	wrote := time.Now() // the Go is written and ACKed
	x, ok := fs.await(salvoReply, func(x swp08.SalvoAck) bool { return x.Cmd == swp08.CmdGoDoneAck && x.Group == g })
	at := time.Now()
	a.mu.Lock()
	for _, t := range staged {
		w := t.w
		w.goAt = wrote
		w.cause.sent = at
		changed := false
		for _, l := range w.levels {
			if l.native {
				l.seen, l.now = false, 0
				changed = changed || !l.same
			}
		}
		if changed {
			w.cause.saw(at) // what follows a destination changes with it, now
		}
	}
	a.mu.Unlock()
	switch {
	case !ok:
		return true, errNoGo
	case x.Status == swp08.GoNothing:
		return true, errors.New("the router had nothing staged to set (123 status 02)")
	case x.Status != swp08.GoSet:
		return true, fmt.Errorf("the Go was answered with 123 status %02X", x.Status)
	}
	return true, nil
}

// Caller holds fireMu, with fs as a.fire.
func (a *App) clearSalvo(ext *extTracker, fs *fireState, g int, why swp08.Why) error {
	for drained := false; !drained; {
		select {
		case <-fs.acks:
		default:
			drained = true
		}
	}
	done, err := a.sendTracked(ext, swp08.GoSalvo(true, g), why)
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	leftQueue(done)
	if _, ok := fs.await(salvoReply, func(x swp08.SalvoAck) bool { return x.Cmd == swp08.CmdGoDoneAck && x.Group == g }); !ok {
		return errNoClear
	}
	return nil
}

// clearStaleSalvo: staged crosspoints outlive a dropped link. Native mode only.
func (a *App) clearStaleSalvo() {
	r := a.r()
	if r.cfg.FireMode != "native" {
		return
	}
	a.fireMu.Lock()
	defer a.fireMu.Unlock()
	fs := &fireState{acks: make(chan swp08.SalvoAck, 16)}
	a.mu.Lock()
	a.fire = fs
	ext := a.ext
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.fire = nil
		a.mu.Unlock()
	}()
	if err := a.clearSalvo(ext, fs, r.cfg.SalvoGroup, swp08.Why{Reason: swp08.ReasonResync}); err != nil {
		log.Printf("salvo: clearing group %d on connect: %v", r.cfg.SalvoGroup, err)
	}
}

// sawOwn: a native salvo's Connecteds arrive after the Go, sometimes after the take
// finished; the latest take of the route gets it. Caller holds mu.
func (a *App) sawOwn(xp swp08.Crosspoint) {
	for i := len(a.ext.causes) - 1; i >= 0; i-- {
		if c := a.ext.causes[i]; c.src == xp.Src && c.dst == xp.Dst {
			c.saw(time.Now())
			return
		}
	}
}
