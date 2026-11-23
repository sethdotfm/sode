package app

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// Protects that Sode sets (GLOSSARY.md Protect, ADR 0012). Sode sets and frees a Protect on every level of a
// destination with its one device number for the router. The router knows only that number, so Sode keeps its own
// record of who set each one (protects.json beside the config).

// protHold is one Protected level as the router reports it: the device that set it and the kind.
type protHold struct{ dev, kind int }

func (h protHold) sodes(dev int) bool { return h.kind == swp08.KindProtect && dev > 0 && h.dev == dev }

// sodesHold: the one rule for a Protect that is Sode's. dev is Sode's device number now; recDev is the device in
// Sode's record (0 with no record), for a Protect set before the number changed. Keep in step with holdsOn in
// web/protectrules.js.
func sodesHold(h protHold, dev, recDev int) bool { return h.sodes(dev) || h.sodes(recDev) }

// protectErr is a refusal. Code and status are for /api/v1; the text is for the page and the Log row.
type protectErr struct {
	status int
	code   string
	msg    string
	// admin: the Protect is another Sode user's, so an Administrator or an Approval frees it.
	admin bool
}

func (e protectErr) Error() string { return e.msg }

func refusal(status int, code, format string, args ...any) protectErr {
	return protectErr{status: status, code: code, msg: fmt.Sprintf(format, args...)}
}

// protectOutcome: a refusal (Err with no Entry), or the History entry of what Sode did.
type protectOutcome struct {
	Entry *Entry
	Err   error
}

// protWait waits for the router to report each level in the state that the messages ask for.
type protWait struct {
	dst, dev int
	set      bool
	left     map[int]bool // UI level → still waiting
	done     chan struct{}
}

// Caller holds mu.
func (w *protWait) see(ui int, h protHold, held bool) {
	if !w.left[ui] {
		return
	}
	if (w.set && held && h.kind == swp08.KindProtect && h.dev == w.dev) || (!w.set && !held) {
		delete(w.left, ui)
		if len(w.left) == 0 {
			close(w.done)
		}
	}
}

// protectSend sends one 0x8C or 0x8E for each level and waits for its 0x8D or 0x8F. It returns the levels the
// router confirmed and the ones it did not.
func (a *App) protectSend(r *runtime, dst int, devOf func(ui int) int, set bool, levels []int, why swp08.Why) (ok, failed []int) {
	if len(levels) == 0 {
		return nil, nil
	}
	waits := map[int]*protWait{}
	a.mu.Lock()
	for _, ui := range levels {
		dev := devOf(ui)
		w := waits[dev]
		if w == nil {
			w = &protWait{dst: dst, dev: dev, set: set, left: map[int]bool{}, done: make(chan struct{})}
			waits[dev] = w
			a.protWaits[w] = struct{}{}
		}
		w.left[ui] = true
	}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		for _, w := range waits {
			delete(a.protWaits, w)
		}
		a.mu.Unlock()
	}()
	var last <-chan struct{}
	unsent := map[int]bool{}
	for _, ui := range levels {
		l, found := r.byUI[ui]
		if !found {
			unsent[ui] = true
			continue
		}
		msg := swp08.ExtProtectDisconnect(r.cfg.Matrix, l.Wire, dst, devOf(ui))
		if set {
			msg = swp08.ExtProtectConnect(r.cfg.Matrix, l.Wire, dst, devOf(ui))
		}
		w := why
		w.Done = make(chan struct{})
		if err := a.sendMsg(msg, w); err != nil {
			unsent[ui] = true
			continue
		}
		last = w.Done
	}
	a.routerUnit.leftQueue(last)
	deadline := time.After(a.routeTimeout)
	for _, w := range waits {
		select {
		case <-w.done:
		case <-deadline:
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, ui := range levels {
		if w := waits[devOf(ui)]; w.left[ui] || unsent[ui] {
			failed = append(failed, ui)
		} else {
			ok = append(ok, ui)
		}
	}
	return ok, failed
}

// sodeHoldsProtLocked: Sode's record of dst (nil when it has none), then dst's holds as protectLevelsMu splits them.
// Caller holds protMu. It takes mu itself after it reads the record, since the lock order is protMu, then mu.
func (a *App) sodeHoldsProtLocked(r *runtime, dst int) (rec *protectRec, own []int, others map[int]protHold) {
	if got, ok := a.protects.get(r.cfg.UID, dst); ok {
		rec = &got
	}
	a.mu.Lock()
	own, others = a.protectLevelsMu(r, dst, rec)
	a.mu.Unlock()
	return rec, own, others
}

// protectLevels: dst's holds, Sode's own levels (by its device or the record's) and the others. Caller holds mu.
func (a *App) protectLevelsMu(r *runtime, dst int, rec *protectRec) (own []int, others map[int]protHold) {
	others = map[int]protHold{}
	recDev := 0
	if rec != nil {
		recDev = rec.Device
	}
	for _, l := range r.levels {
		h, held := a.router.protect[dst][l.UI]
		switch {
		case !held:
		case sodesHold(h, r.cfg.ProtectDevice, recDev):
			own = append(own, l.UI)
		default:
			others[l.UI] = h
		}
	}
	return own, others
}

func holdText(r *runtime, hs map[int]protHold) string {
	byKind := map[string][]int{}
	for ui, h := range hs {
		k := fmt.Sprintf("Protected by another system (device %d)", h.dev)
		if h.kind == swp08.KindRouterLock {
			k = fmt.Sprintf("Router lock by another system (device %d)", h.dev)
		}
		byKind[k] = append(byKind[k], ui)
	}
	var parts []string
	for _, k := range sortedKeys(byKind) {
		ls := byKind[k]
		sort.Ints(ls)
		parts = append(parts, fmt.Sprintf("%s on levels %s", k, levelList(r, ls)))
	}
	return strings.Join(parts, "; ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// protectRights: nil when rt has the Protect right and dst. Every level must be in rt, since a Protect covers them
// all.
func protectRights(r *runtime, rt config.Rights, dst int) error {
	off := r.cfg.NumberOffset
	switch {
	case !rt.AllowsProtect():
		return refusal(403, "outside_rights", "Protect is outside your Rights")
	case !rt.AllowsDst(config.Ref{N: dst}):
		if rt.Hidden() {
			return refusal(403, "outside_rights", "destination is outside your rights")
		}
		return refusal(403, "outside_rights", "destination %d is outside your rights", dst+off)
	}
	for _, l := range r.levels {
		if !rt.AllowsLevel(l.UI) {
			return refusal(403, "outside_rights", "a Protect covers every level, and level %s is outside your rights", r.cfg.LevelLabel(l.UI))
		}
	}
	return nil
}

// protectReady: the checks before anything is sent.
func (a *App) protectReady(r *runtime, dst int) error {
	a.mu.Lock()
	up := a.status.Router == string(swp08.StateUp)
	_, named := a.router.dstNames[dst]
	a.mu.Unlock()
	switch {
	case r.cfg.ProtectDevice <= 0:
		return refusal(409, "conflict", "set Sode's device number in Settings > Routers > Protects first")
	case dst < 0 || (!named && dst > 0xFFFF):
		return refusal(400, "bad_request", "no such destination")
	case !up:
		return refusal(503, "router_down", "Sode is not connected to the router")
	}
	return nil
}

func (a *App) protectEntry(r *runtime, c *client, set bool, dst int, levels []int, undo *int) *Entry {
	e := &Entry{Time: time.Now(), Client: c.who(), Src: -1, Dst: dst, Levels: levels, Prev: [][2]int{}, Undo: undo,
		Protect: "clear"}
	if set {
		e.Protect = "set"
	}
	a.mu.Lock()
	e.DstName = a.router.dstNames[dst]
	// The routes on dst when it ran, so History can show what the Protect holds there.
	for _, u := range levels {
		if s, ok := a.router.tally[dst][u]; ok {
			e.Prev = append(e.Prev, [2]int{u, s})
		}
	}
	a.mu.Unlock()
	e.BrowserOf = a.browserOf(c)
	if c.api != nil {
		e.setActor(c.api.Actor)
		// No PanelID: a panel's Undo cell undoes takes only (panelundo.go).
		e.Panel, e.Instance = a.panelNames(c.api.Actor)
	}
	return e
}

// undoOf checks that an undo names an entry of the other kind on the same destination.
func (a *App) undoOf(undo *int, dst int, want string) error {
	if undo == nil {
		return nil
	}
	e, ok := a.hist.Get(*undo)
	if !ok || e.Ext || e.Protect != want || e.Dst != dst {
		return refusal(400, "bad_request", "that History entry is not a Protect to undo on this destination")
	}
	return nil
}

// protGroup is the History Group of one Protect or Free Protect on many destinations, as a take's Group is
// (docs/agents/history.md). The zero value is no Group. n counts only the entries the action writes, since the page
// waits for that many.
type protGroup struct {
	id string
	n  int
}

// The "p" keeps it apart from a take's Group and from the ext "x" and "f" Groups made in the same nanosecond.
func newProtGroup(n int) protGroup {
	if n < 2 {
		return protGroup{}
	}
	return protGroup{id: "p" + strconv.FormatInt(time.Now().UnixNano(), 36), n: n}
}

func (g protGroup) mark(e *Entry) { e.Group, e.GroupN = g.id, g.n }

// protectSet sets a Protect on every level of dst. A level the router does not confirm rolls back the levels that
// were set. What the rollback cannot free stays in Sode's record and in the entry's Still.
func (a *App) protectSet(c *client, dst int, undo *int) protectOutcome {
	return a.protectMany(c, []protectDst{{Dst: dst, Undo: undo}}, true, nil)[0]
}

// protectClear frees Sode's Protect on dst. Another Sode user's Protect, or one with no record, needs an
// Administrator: the caller's Role or the approver's.
func (a *App) protectClear(c *client, dst int, undo *int, approver *config.User) protectOutcome {
	return a.protectMany(c, []protectDst{{Dst: dst, Undo: undo}}, false, approver)[0]
}

// protRun is one destination of protectMany that passed every check, with what the checks read.
type protRun struct {
	i         int
	rec       *protectRec
	own, todo []int
}

// protectMany sets (set) or frees each destination of reqs, one outcome for each in order. It checks them all under
// protMu before it sends anything, so the ones that run share one Group of the right size.
func (a *App) protectMany(c *client, reqs []protectDst, set bool, approver *config.User) []protectOutcome {
	r := a.r()
	outs := make([]protectOutcome, len(reqs))
	act := a.caller(c)
	admin := !set && protectAdmin(r.cfg, act, approver)
	want := "set" // the kind an Undo names
	if set {
		want = "clear"
	}
	for i, q := range reqs {
		if err := a.protectReady(r, q.Dst); err != nil {
			outs[i].Err = err
			continue
		}
		if err := protectRights(r, act.rights, q.Dst); err != nil && !admin {
			outs[i].Err = err
			continue
		}
		outs[i].Err = a.undoOf(q.Undo, q.Dst, want)
	}
	a.protMu.Lock()
	defer a.protMu.Unlock()
	var runs []protRun
	for i, q := range reqs {
		if outs[i].Err != nil {
			continue
		}
		rec, own, others := a.sodeHoldsProtLocked(r, q.Dst)
		var err error
		var todo []int
		if set {
			todo, err = setCheck(r, rec, own, others)
		} else {
			err = clearCheck(r, act.Actor, rec, own, admin)
		}
		if err != nil {
			outs[i].Err = err
			continue
		}
		runs = append(runs, protRun{i: i, rec: rec, own: own, todo: todo})
	}
	g := newProtGroup(len(runs))
	for _, p := range runs {
		q := reqs[p.i]
		if set {
			outs[p.i].Entry = a.setOne(r, c, act.Actor, q.Dst, p.rec, p.own, p.todo, q.Undo, g)
		} else {
			outs[p.i].Entry = a.clearOne(r, c, q.Dst, p.rec, p.own, q.Undo, approver, g)
		}
	}
	return outs
}

// setCheck: the levels a Protect sets on a destination, or why it refuses.
func setCheck(r *runtime, rec *protectRec, own []int, others map[int]protHold) ([]int, error) {
	if len(others) > 0 {
		return nil, refusal(409, "protected", "%s", holdText(r, others))
	}
	if len(own) == len(r.levels) {
		return nil, refusal(409, "conflict", "Sode has a Protect on this destination already")
	}
	if rec != nil && rec.Device != r.cfg.ProtectDevice {
		return nil, refusal(409, "protected", "Sode holds some levels with device %d; free them first", rec.Device)
	}
	var todo []int
	for _, l := range r.levels {
		if !slices.Contains(own, l.UI) {
			todo = append(todo, l.UI)
		}
	}
	return todo, nil
}

// clearCheck: why a Free Protect refuses, or nil.
func clearCheck(r *runtime, act Actor, rec *protectRec, own []int, admin bool) error {
	if rec == nil && len(own) == 0 {
		return refusal(404, "not_found", "Sode has no Protect on this destination")
	}
	user, client := recOwner(r.cfg, act)
	if !rec.ownedBy(user, client, act) && !admin {
		who := "a Sode user"
		if rec != nil {
			who = rec.By
		}
		pe := refusal(403, "administrator_only", "%s set this Protect: freeing it needs an Administrator", who)
		pe.admin = true
		return pe
	}
	return nil
}

// setOne sets the todo levels of dst after setCheck. Caller holds protMu.
func (a *App) setOne(r *runtime, c *client, act Actor, dst int, rec *protectRec, own, todo []int, undo *int, g protGroup) *Entry {
	reason := swp08.Reason("protect")
	if undo != nil {
		reason = "protectUndo"
	}
	dev := r.cfg.ProtectDevice
	devOf := func(int) int { return dev }
	e := a.protectEntry(r, c, true, dst, uiLevels(r), undo)
	g.mark(e)
	user, client := recOwner(r.cfg, act)
	setOK, failed := a.protectSend(r, dst, devOf, true, todo, swp08.Why{Reason: reason, By: c.who()})
	held := append(slices.Clone(own), setOK...)
	if len(failed) > 0 {
		e.Failed = failed
		_, stuck := a.protectSend(r, dst, devOf, false, setOK, swp08.Why{Reason: "protectRollback", By: c.who()})
		held = append(slices.Clone(own), stuck...)
		e.Still = slices.Clone(held)
		sort.Ints(e.Still)
		if len(e.Still) == 0 {
			e.Err = fmt.Sprintf("the router did not confirm levels %s; Sode freed the other levels again", levelList(r, failed))
		} else {
			e.Err = fmt.Sprintf("the router did not confirm levels %s, and levels %s are still Protected", levelList(r, failed), levelList(r, e.Still))
		}
	}
	e.OK = len(failed) == 0
	next := protectRec{Router: r.cfg.UID, Dst: dst, DstName: e.DstName, Device: dev, Levels: held, By: c.who(), User: user, Client: client, At: e.Time}
	if rec != nil && len(failed) > 0 {
		next.By, next.User, next.Client, next.At = rec.By, rec.User, rec.Client, rec.At
	}
	a.protectDone(next, e)
	return e
}

// protectAdmin: the caller or the approver is an Administrator. An API caller has no Role.
func protectAdmin(cfg config.Config, act acting, approver *config.User) bool {
	if approver != nil {
		return approverRole(cfg, approver) == config.RoleAdministrator
	}
	return act.role == config.RoleAdministrator
}

// clearOne frees the levels Sode holds on dst, each with the device that holds it. Caller holds protMu.
func (a *App) clearOne(r *runtime, c *client, dst int, rec *protectRec, own []int, undo *int, approver *config.User, g protGroup) *Entry {
	devs := map[int]int{}
	a.mu.Lock()
	for _, ui := range own {
		devs[ui] = a.router.protect[dst][ui].dev
	}
	a.mu.Unlock()
	levels := slices.Clone(own)
	if rec != nil {
		for _, ui := range rec.Levels {
			if _, ok := devs[ui]; !ok {
				devs[ui] = rec.Device
				levels = append(levels, ui)
			}
		}
	}
	sort.Ints(levels)
	reason := swp08.Reason("protectClear")
	if undo != nil {
		reason = "protectUndo"
	}
	why := swp08.Why{Reason: reason, By: c.who()}
	why.Note = approvedNote(approver, "")
	e := a.protectEntry(r, c, false, dst, uiLevels(r), undo)
	g.mark(e)
	_, failed := a.protectSend(r, dst, func(ui int) int { return devs[ui] }, false, levels, why)
	e.OK = len(failed) == 0
	if !e.OK {
		e.Failed, e.Still = failed, failed
		e.Err = fmt.Sprintf("the router did not confirm levels %s; they are still Protected", levelList(r, failed))
	}
	next := protectRec{Router: r.cfg.UID, Dst: dst, DstName: e.DstName, Device: r.cfg.ProtectDevice, By: c.who(), At: e.Time}
	if rec != nil {
		next = *rec
	}
	next.Levels = failed
	a.protectDone(next, e)
	return e
}

// clearSodeProtects frees every Protect the router reports with Sode's device number, and every one in Sode's
// record, with or without the other. One History entry for each destination, all in one Group.
func (a *App) clearSodeProtects(c *client) (cleared int, still []int, err error) {
	r := a.r()
	if err := a.protectReady(r, 0); err != nil {
		var pe protectErr
		if errors.As(err, &pe) && pe.code == "router_down" {
			return 0, nil, err
		}
	}
	a.protMu.Lock()
	defer a.protMu.Unlock()
	dsts := map[int]bool{}
	for _, rec := range a.protects.list(r.cfg.UID) {
		dsts[rec.Dst] = true
	}
	a.mu.Lock()
	for d, t := range a.router.protect {
		for _, h := range t {
			if h.sodes(r.cfg.ProtectDevice) {
				dsts[d] = true
			}
		}
	}
	a.mu.Unlock()
	order := make([]int, 0, len(dsts))
	for d := range dsts {
		order = append(order, d)
	}
	sort.Ints(order)
	g := newProtGroup(len(order))
	for _, d := range order {
		rec, own, _ := a.sodeHoldsProtLocked(r, d)
		e := a.clearOne(r, c, d, rec, own, nil, nil, g)
		if e.OK {
			cleared++
		} else {
			still = append(still, d)
		}
	}
	return cleared, still, nil
}

// protectDone writes Sode's record, tells the pages, then writes the History entry. The pages get the new state
// first, so the entry's row draws with its Undo right.
func (a *App) protectDone(rec protectRec, e *Entry) {
	if err := a.protects.put(rec); err != nil {
		log.Printf("protects: %v", err)
	}
	a.mu.Lock()
	a.broadcastProtectsMu()
	a.mu.Unlock()
	a.record(e)
}

func uiLevels(r *runtime) []int {
	out := make([]int, 0, len(r.levels))
	for _, l := range r.levels {
		out = append(out, l.UI)
	}
	return out
}
