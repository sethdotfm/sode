package app

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// routerUnit is everything Sode keeps for one router: its state, Router link, sender, take timers, External tracking,
// waiters, checks, caches, Protect waits, uptime and status. App holds one for each router by UID (App.routers).
// App.first is the first router's unit; every caller names the unit it acts on. The fields from router down are
// guarded by App.mu.
type routerUnit struct {
	uid string

	sw      *swp08.Client
	sendMsg func([]byte, swp08.Why) error // sw.Send; tests answer as the router would
	up      *Uptime

	router   *routerState // replaced as a whole on a router change (applyConfig with reconnect)
	ext      *extTracker
	status   statusMsg
	waiters  map[*waiter]struct{}
	checks   map[*tallyCheck]struct{}
	linkSeen string
	linkErr  string
	fire     *fireState

	// Take timers; only tests shorten them (shortTimers).
	checkTimeout, routeTimeout, routeGrace, queueMax time.Duration

	nameReqAt time.Time
	nameRxAt  time.Time
	namesDirt bool
	dumpAt    time.Time

	tallyCache tallyBody // apireads.go
	namesCache namesBody // apireads.go

	protWaits map[*protWait]struct{}

	fireMu sync.Mutex // one native salvo at a time: the router's staged list is shared (fire.go)
	// fireHolder: what holds fireMu now, for the wait in the next fire's lock row. Guarded by App.mu.
	fireHolder string
	// stop ends the link and the uptime loop that startUnitMu began; nil before.
	stop func()
}

// The uptime is in memory until SetUptime gives the file.
func newRouterUnit(r config.Router) *routerUnit {
	up, _ := openUptime("", time.Now)
	return &routerUnit{
		up:           up,
		uid:          r.UID,
		router:       newRouterState(),
		ext:          newExtTracker(),
		status:       statusMsg{T: "status", Router: string(swp08.StateConnecting), Addr: r.Address},
		waiters:      map[*waiter]struct{}{},
		checks:       map[*tallyCheck]struct{}{},
		checkTimeout: defaultCheckTimeout,
		routeTimeout: defaultRouteTimeout,
		routeGrace:   defaultRouteGrace,
		queueMax:     defaultQueueMax,
		protWaits:    map[*protWait]struct{}{},
	}
}

// link makes the unit's Router link. Its callbacks name the unit, so a message changes only this router's state.
func (a *App) link(u *routerUnit, r *runtime, debug bool) {
	rc := r.routerOf(u.uid)
	u.sw = swp08.NewClient(rc.Address, probe(rc))
	u.sw.Debug = debug
	u.sw.OnUp = func() { a.onUnitUp(u) }
	u.sw.OnMsg = func(m []byte) { a.onUnitMsg(u, m) }
	u.sw.OnState = func(s swp08.State, err error) { a.onUnitState(u, s, err) }
	// Every row the link logs names this router.
	u.sw.OnTx = func(e swp08.TxEvent) {
		e.Why.Router = u.uid
		a.onTx(e)
	}
	u.sendMsg = func(m []byte, why swp08.Why) error {
		why.Router = u.uid
		err := u.sw.Send(m, why)
		// Only a take's refusals are logged; refused re-reads while down would flood the log.
		if err != nil && !background[why.Reason] {
			a.onTx(swp08.TxEvent{Msg: m, Why: why, At: time.Now(), Result: "unsent", Err: err.Error()})
		}
		return err
	}
}

// routerOf gives the settings of the router with this UID. Blank, or a UID that the settings do not have (a unit
// made before an import changed the UID), gives the first router.
func (r *runtime) routerOf(uid string) config.Router {
	if rc, ok := r.cfg.RouterByUID(uid); ok {
		return rc
	}
	return r.cfg.Router
}

// unit gives the router unit for a router UID; blank is the first router.
func (a *App) unit(uid string) (*routerUnit, bool) {
	if uid == "" {
		return a.first, a.first != nil
	}
	u, ok := a.units()[uid]
	return u, ok
}

// startUnitMu runs the link and the uptime loop of u on the links' context. Caller holds mu.
func (a *App) startUnitMu(u *routerUnit) {
	ctx, cancel := context.WithCancel(a.linkCtx)
	u.stop = cancel
	go u.sw.Run(ctx)
	a.unitRuns.Add(1)
	go func() { defer a.unitRuns.Done(); u.up.Run(ctx) }()
}

// syncUnitsMu makes a unit for each router that the settings add, and starts its link while Run runs. It takes the
// unit of each router that they remove out of the list, and cuts its takes. The first router's unit always stays.
// The caller ends the gone units' links (endUnit) after it releases mu. Caller holds mu.
func (a *App) syncUnitsMu(r *runtime) (added, gone []*routerUnit) {
	old := a.units()
	next := map[string]*routerUnit{a.first.uid: a.first}
	for _, rc := range r.cfg.MoreRouters {
		if u, ok := old[rc.UID]; ok {
			next[rc.UID] = u
			continue
		}
		u := newRouterUnit(rc)
		a.link(u, r, a.debug)
		if a.linkCtx != nil {
			a.startUnitMu(u)
		}
		next[u.uid] = u
		added = append(added, u)
	}
	for uid, u := range old {
		if _, ok := next[uid]; !ok {
			u.cutRoutes()
			gone = append(gone, u)
		}
	}
	if len(added) > 0 || len(gone) > 0 {
		a.routers.Store(&next)
	}
	return added, gone
}

// endUnit stops the link and the uptime of a unit that syncUnitsMu took out.
func endUnit(u *routerUnit) {
	if u.stop != nil {
		u.stop()
	}
	u.up.Stop()
}

// gone: u is no longer a router of Sode, so its link's late callbacks change nothing.
func (a *App) gone(u *routerUnit) bool {
	_, ok := a.units()[u.uid]
	return !ok
}

// units gives every router unit by router UID. The map is never changed: syncUnitsMu swaps in a new one.
func (a *App) units() map[string]*routerUnit { return *a.routers.Load() }

// firstUnit gives the first router's unit. The flat hello fields, the protected message and the uptime file name it.
func (a *App) firstUnit() *routerUnit { return a.first }

// unitOrFirst gives the unit for a router UID. Blank, or a UID with no unit (the settings UID after an import changed
// it), gives the first router.
func (a *App) unitOrFirst(uid string) *routerUnit {
	if u, ok := a.unit(uid); ok {
		return u
	}
	return a.first
}

// unitOfRouter gives the unit of a router in cfg. The first router's unit can carry an older UID (an import changed it).
func (a *App) unitOfRouter(cfg config.Config, uid string) (*routerUnit, bool) {
	if uid == cfg.UID {
		return a.first, a.first != nil
	}
	return a.unit(uid)
}

// knownRouter: uid is blank, the first router's UID in the settings, or the UID of a router unit.
func (a *App) knownRouter(uid string) bool {
	_, ok := a.unit(uid)
	return ok || uid == a.r().cfg.UID
}

// apiUnit gives the unit of the router parameter of an API read; routerOK has refused an unknown one. Blank, or the
// UID of the first router in the settings, is the first router.
func (a *App) apiUnit(r *http.Request) *routerUnit { return a.unitOrFirst(r.URL.Query().Get("router")) }

// apiReadUnit is apiUnit for a read about one router. With two or more routers, a read with no router parameter
// gets 400 router_needed. Thus a client made for one router never reads the wrong one.
func (a *App) apiReadUnit(w http.ResponseWriter, r *http.Request) (*routerUnit, bool) {
	if r.URL.Query().Get("router") == "" && len(a.units()) > 1 {
		apiFail(w, http.StatusBadRequest, "router_needed", routerNeeded(len(a.units())))
		return nil, false
	}
	return a.apiUnit(r), true
}

func routerNeeded(n int) string {
	return fmt.Sprintf("Sode has %d routers: add ?router= with a uid from GET %s/routers", n, apiBase)
}

// statusOut is the unit's link status as it goes to browsers and API streams. Caller holds mu.
func (u *routerUnit) statusOut() statusMsg {
	s := u.status
	s.from, s.RouterUID = u, u.uid
	return s
}

// unitsInOrder gives the unit of each router in the order of the settings. The first router comes first, also when
// the settings no longer name its unit's UID.
func (a *App) unitsInOrder(r *runtime) []*routerUnit {
	var out []*routerUnit
	add := func(u *routerUnit, ok bool) {
		if ok && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	add(a.unit(""))
	for _, rc := range r.cfg.Routers() {
		add(a.unit(rc.UID))
	}
	return out
}
