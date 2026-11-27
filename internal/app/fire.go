package app

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// Native fire mode: docs/agents/salvos.md. All clients share the staged list, so fire holds fireMu from the first
// clear to the last. After Sode sends the Go, it never sends it again and never follows it with connects.

// salvoReply: acknowledge wait after the command was ACKed; the Ultrix answers in 5-7 ms.
const salvoReply = time.Second

// salvoAnnounceFor: a Connected for a tally-confirmed staged level still counts this long after the Go (Ultrix:
// 100-340 ms for about 50 crosspoints).
const salvoAnnounceFor = 2 * time.Second

// stageChunk: caps queued stage messages so a big salvo does not fill the send queue.
const stageChunk = 64

// fireState collects acknowledges for the fire in progress on a router (routerUnit.fire, guarded by mu); others
// are dropped.
type fireState struct {
	acks chan swp08.SalvoAck
}

// onSalvoAck gives an acknowledge from router u to the fire in progress there. Caller holds mu.
func (u *routerUnit) onSalvoAck(x swp08.SalvoAck) {
	if u.fire == nil {
		return
	}
	select {
	case u.fire.acks <- x:
	default:
		log.Printf("salvo: %s: acknowledge dropped, collector full (cmd %d)", u.name(), x.Cmd)
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

// markStages sets Stage per item and returns how many items stage something. A destination
// level used twice is staged in neither, since the order would be up to the router.
func markStages(r *runtime, items []routeReq) int {
	type dl struct{ dst, wire int }
	uses := map[dl]int{}
	for _, it := range items {
		seen := map[int]bool{}
		for _, ui := range it.Levels {
			if l, ok := r.levelsOf(it.Router).byUI[ui]; ok && !seen[l.Wire] {
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
			lv := r.levelsOf(it.Router)
			l, ok := lv.byUI[ui]
			if !ok || !swp08.ExtSalvoFits(lv.rc.Matrix, l.Wire, it.Dst, it.Src) || uses[dl{it.Dst, l.Wire}] != 1 {
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

// takeNative runs a take whose routes on router u stage levels in a native group salvo. takeOnRouter read the routes
// first.
func (a *App) takeNative(r *runtime, u *routerUnit, c *client, items []routeReq, ext *extTracker) {
	runs := make([]*routeRun, len(items))
	runLimited(routeLimit, len(items), func(i int) { runs[i] = a.prepare(r, u, c, items[i], ext) })
	var live, staged []*routeRun
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
		got, sent, err := a.fireSalvo(r, u, c, staged)
		n := 0
		for _, t := range staged {
			n += len(t.w.filter(func(l *routeLevel) bool { return l.native }))
		}
		switch {
		case !sent:
			log.Printf("salvo: %s: not fired (%v); sending %d takes one after another", a.routerName(u), err, len(live))
			a.command(c, "salvo", "fallback", fmt.Sprintf("%d crosspoints; sent one after another", n), err)
		case err != nil:
			log.Printf("salvo: %s: Go sent for %d crosspoints, %v; the takes' interrogates will tell", a.routerName(u), n, err)
			a.command(c, "salvo", "error", fmt.Sprintf("%d crosspoints", n), err)
		default:
			a.command(c, "salvo", "ok", fmt.Sprintf("%d crosspoints set in %d ms", n, time.Since(got).Milliseconds()), nil)
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
	runLimited(routeLimit, len(live), func(i int) { a.complete(r, live[i], a.sendRoute(r, live[i].w)) })
	a.logSalvoTiming(u, c, staged)
}

func (a *App) logSalvoTiming(u *routerUnit, c *client, staged []*routeRun) {
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
	log.Printf("salvo: %s: %s", a.routerName(u), note)
	a.command(c, "salvo", result, note, nil)
}

// Only the take goroutine writes native.
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
	if w.u.ext == w.ext && !w.cut {
		w.ownKeys()
	}
}

var (
	errNoClear = errors.New("the router did not acknowledge clearing the salvo group (no 123)")
	errNoStage = errors.New("the router did not acknowledge every staged crosspoint (no 122 or 250)")
	errNoGo    = errors.New("the router did not acknowledge the Go (no 123)")
)

// fireSalvo fires on router u. sent reports whether the Go was sent. If not, the takes can go as connects. err
// with sent means that the Go's acknowledge was missing or odd. got is when the fire got fireMu.
// A salvo command row by c marks when the fire gets fireMu and when it lets go, so a wait for another fire shows.
func (a *App) fireSalvo(r *runtime, u *routerUnit, c *client, staged []*routeRun) (got time.Time, sent bool, err error) {
	waited, holder := a.lockFire(u, "a fire by "+c.who())
	got = time.Now()
	note := "fire lock: no wait"
	if holder != "" {
		note = fmt.Sprintf("fire lock: waited %d ms for %s", waited.Milliseconds(), holder)
	}
	a.command(c, "salvo", "lock", note, nil)
	// Deferred first, so it runs after the last Go-clear.
	defer func() {
		a.command(c, "salvo", "release", fmt.Sprintf("fire lock: held %d ms", time.Since(got).Milliseconds()), nil)
		a.unlockFire(u)
	}()
	sent, err = a.fireLocked(r, u, staged)
	return got, sent, err
}

// lockFire takes the fireMu of router u for what. When it had to wait, holder names what held the lock then.
func (a *App) lockFire(u *routerUnit, what string) (waited time.Duration, holder string) {
	if !u.fireMu.TryLock() {
		a.mu.Lock()
		holder = u.fireHolder
		a.mu.Unlock()
		if holder == "" {
			holder = "another fire" // it let go between the two reads
		}
		start := time.Now()
		u.fireMu.Lock()
		waited = time.Since(start)
	}
	a.mu.Lock()
	u.fireHolder = what
	a.mu.Unlock()
	return waited, holder
}

func (a *App) unlockFire(u *routerUnit) {
	a.mu.Lock()
	u.fireHolder = ""
	a.mu.Unlock()
	u.fireMu.Unlock()
}

// setFire makes fs the fire in progress on router u, or none for nil.
func (a *App) setFire(u *routerUnit, fs *fireState) {
	a.mu.Lock()
	u.fire = fs
	a.mu.Unlock()
}

// fireLocked is the fire itself, on router u. Caller holds u.fireMu.
func (a *App) fireLocked(r *runtime, u *routerUnit, staged []*routeRun) (sent bool, err error) {
	n := 0
	for _, t := range staged {
		n += len(t.w.levels)
	}
	fs := &fireState{acks: make(chan swp08.SalvoAck, n+16)}
	a.setFire(u, fs)
	defer a.setFire(u, nil)
	rc := r.routerOf(u.uid())
	mx, g := rc.Matrix, rc.SalvoGroup
	ext := staged[0].w.ext
	why := staged[0].w.why

	// Clear anything staged that is not ours; this also proves group salvos work.
	if err := a.clearSalvo(u, ext, fs, g, why); err != nil {
		return false, err
	}
	// Cleared after every fire; on failure, it is possible that the router still holds some of ours.
	defer func() {
		if err := a.clearSalvo(u, ext, fs, g, why); err != nil {
			log.Printf("salvo: %s: clearing group %d after the fire: %v", a.routerName(u), g, err)
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
			done, err := a.sendTracked(u, ext, m, t.w.why)
			if err != nil {
				return false, fmt.Errorf("staging: %w", err)
			}
			x, _ := swp08.StagedOf(m)
			want[x]++
			missing++
			last = done
			if queued++; queued%stageChunk == 0 {
				u.leftQueue(last)
			}
		}
	}
	u.leftQueue(last)
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

	done, err := a.sendTracked(u, ext, swp08.GoSalvo(false, g), why)
	if err != nil {
		return false, fmt.Errorf("go: %w", err)
	}
	// From here, it is possible that the routes changed: never again as connects.
	u.leftQueue(done)
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

// Caller holds u.fireMu, with fs as u.fire.
func (a *App) clearSalvo(u *routerUnit, ext *extTracker, fs *fireState, g int, why swp08.Why) error {
	for drained := false; !drained; {
		select {
		case <-fs.acks:
		default:
			drained = true
		}
	}
	done, err := a.sendTracked(u, ext, swp08.GoSalvo(true, g), why)
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	u.leftQueue(done)
	if _, ok := fs.await(salvoReply, func(x swp08.SalvoAck) bool { return x.Cmd == swp08.CmdGoDoneAck && x.Group == g }); !ok {
		return errNoClear
	}
	return nil
}

// clearStaleSalvo: staged crosspoints outlive a dropped link of router u. Native mode only.
func (a *App) clearStaleSalvo(u *routerUnit) {
	rc := a.r().routerOf(u.uid())
	if rc.FireMode != "native" {
		return
	}
	a.lockFire(u, "the salvo clear after a connect")
	defer a.unlockFire(u)
	fs := &fireState{acks: make(chan swp08.SalvoAck, 16)}
	a.mu.Lock()
	u.fire = fs
	ext := u.ext
	a.mu.Unlock()
	defer a.setFire(u, nil)
	if err := a.clearSalvo(u, ext, fs, rc.SalvoGroup, swp08.Why{Reason: swp08.ReasonResync}); err != nil {
		log.Printf("salvo: %s: clearing group %d on connect: %v", a.routerName(u), rc.SalvoGroup, err)
	}
}

// sawOwn: a native salvo's Connecteds arrive after the Go, sometimes after the take
// finished; the latest take of the route on router u gets it. Caller holds mu.
func (u *routerUnit) sawOwn(xp swp08.Crosspoint) {
	for i := len(u.ext.causes) - 1; i >= 0; i-- {
		if c := u.ext.causes[i]; c.src == xp.Src && c.dst == xp.Dst {
			c.saw(time.Now())
			return
		}
	}
}
