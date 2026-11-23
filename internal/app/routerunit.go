package app

import (
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// routerUnit is everything Sode keeps for one router: its state, Router link, sender, take timers, External tracking,
// waiters, checks, caches, Protect waits, uptime and status. App holds one for each router by UID (App.routers).
// App embeds the first one, so the old App fields (a.router, a.sw, a.status and the others) still name it. Those
// aliases go when every caller names its router. The fields from router down are guarded by App.mu.
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
	u.sw = swp08.NewClient(r.cfg.Address, r.probe())
	u.sw.Debug = debug
	u.sw.OnUp = func() { a.onUnitUp(u) }
	u.sw.OnMsg = func(m []byte) { a.onUnitMsg(u, m) }
	u.sw.OnState = func(s swp08.State, err error) { a.onUnitState(u, s, err) }
	u.sw.OnTx = a.onTx
	u.sendMsg = func(m []byte, why swp08.Why) error {
		err := u.sw.Send(m, why)
		// Only a take's refusals are logged; refused re-reads while down would flood the log.
		if err != nil && !background[why.Reason] {
			a.onTx(swp08.TxEvent{Msg: m, Why: why, At: time.Now(), Result: "unsent", Err: err.Error()})
		}
		return err
	}
}

// unit gives the router unit for a router UID; blank is the first router.
func (a *App) unit(uid string) (*routerUnit, bool) {
	if uid == "" {
		return a.routerUnit, a.routerUnit != nil
	}
	u, ok := a.routers[uid]
	return u, ok
}

// The old router callbacks: each one acts on the first router.
func (a *App) onRouterUp()                            { a.onUnitUp(a.routerUnit) }
func (a *App) onRouterMsg(msg []byte)                 { a.onUnitMsg(a.routerUnit, msg) }
func (a *App) onRouterState(s swp08.State, err error) { a.onUnitState(a.routerUnit, s, err) }
