package app

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// Native fire mode (#154): the takes of one confirm are staged in a SW-P-08
// group salvo (120, each answered by 122) and set with one Go (121, answered
// by 123), so every crosspoint lands on the same frame. A Go announces
// nothing (no Connected), so each take then interrogates its levels and the
// tallies confirm it, through the same waiter, failure and late-answer
// handling as a sequential take.
//
// Only crosspoints the standard layout can carry are staged: matrix and wire
// level 0-15, destination and source 0-1023 (markStages). A take's other
// levels (level 17 with the usual offset) are sent as ordinary connects right
// after the Go; a take with a number past 1023 is sent as today. A batch with
// fewer than two takes to stage is sequential throughout.
//
// The router's staged list is shared by every client, so a fire holds
// fireMu from the first clear to the last, sends one salvo command at a time
// and clears the group before staging and after every fire, set or not.
// Anything going wrong before the Go (no 122 for a crosspoint, no 123 for
// the first clear, the link refusing a message) clears the group and sends
// the whole batch as ordinary connects. Once the Go has been sent it is
// never sent again and never followed by the connects: the takes'
// interrogates say what happened.

// salvoReply is how long a salvo command's acknowledge (122 or 123) may take
// once the command has been written and ACKed. The Ultrix answers in 5-7 ms.
const salvoReply = time.Second

// stageChunk caps how many stage messages are queued at once, so a big
// salvo doesn't fill the router link's send queue.
const stageChunk = 64

// fireState collects the salvo acknowledges for the fire in progress. It is
// a.fire while one runs (guarded by mu); acknowledges with no fire running
// are dropped.
type fireState struct {
	acks chan swp08.SalvoAck
}

// onSalvoAck hands a 122 or 123 to the fire in progress. Caller holds mu.
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

// await waits up to d for an acknowledge that ok accepts; others are dropped.
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

// markStages sets each item's Stage: the wire levels that can go in a group
// salvo. A destination level taken twice in the batch is staged in neither
// (which lands last would be up to the router). It returns how many items
// have something to stage.
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
			if !ok || !swp08.SalvoFits(r.cfg.Matrix, l.Wire, it.Dst, it.Src) || uses[dl{it.Dst, l.Wire}] != 1 {
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

// batchNative runs a batch with markStages done: every take is prepared
// (protects, the pre-take check, registered), those with staged levels are
// fired together, and then each take sends what is left and waits for the
// router as a sequential take does.
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
}

// hasNative reports whether any level is staged. Only the batch goroutine
// writes native.
func (w *waiter) hasNative() bool {
	for _, l := range w.levels {
		if l.native {
			return true
		}
	}
	return false
}

// unNative turns a take's staged levels back into ordinary connects, with
// the own keys their Connecteds will use up.
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
	errNoStage = errors.New("the router did not acknowledge every staged crosspoint (no 122)")
	errNoGo    = errors.New("the router did not acknowledge the Go (no 123)")
)

// fireSalvo stages the native levels of staged and sets them with one Go.
// sent reports whether the Go was sent: if not, nothing was set and the
// takes may go as ordinary connects. err with sent says the Go's
// acknowledge was missing or odd.
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

	// Whatever is staged in the group now isn't ours: clear it, and know the
	// router does group salvos before anything is staged.
	if err := a.clearSalvo(ext, fs, g, why); err != nil {
		return false, err
	}
	// The group is cleared after every fire, set or not; on the way out of a
	// failure, the router may still hold some of ours.
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
	// Every crosspoint must be staged before the Go: one the link gave up on
	// would be left out of the salvo.
	if missing > 0 {
		if _, ok := fs.await(salvoReply, func(x swp08.SalvoAck) bool {
			if x.Cmd != swp08.CmdConnectOnGoAck || want[x.Staged()] == 0 {
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
	x, ok := fs.await(salvoReply, func(x swp08.SalvoAck) bool { return x.Cmd == swp08.CmdGoDoneAck && x.Group == g })
	at := time.Now()
	a.mu.Lock()
	for _, t := range staged {
		w := t.w
		w.cause.sent = at
		changed := false
		for _, l := range w.levels {
			if l.native {
				// Reports before the Go were of the old route.
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

// clearSalvo clears group g and waits for the router to say so. Caller
// holds fireMu, with fs as a.fire.
func (a *App) clearSalvo(ext *extTracker, fs *fireState, g int, why swp08.Why) error {
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

// clearStaleSalvo clears Sode's salvo group on a new session: staged
// crosspoints outlive a dropped link, and a message the old session dropped
// is never resent. Only in native mode: sequential mode sends no salvo
// command at all.
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
