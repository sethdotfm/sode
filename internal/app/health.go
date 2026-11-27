package app

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// The state of Sode as a whole (GLOSSARY.md: Healthy, Degraded, Down). Only a link that is up counts as up, so a
// router that is still connecting counts as down. docs/tielines-design.md, "Health and status".
const (
	stateHealthy  = "healthy"
	stateDegraded = "degraded"
	stateDown     = "down"
)

// wholeState gives the state of Sode from the link state of each router.
func wholeState(links []statusMsg) string {
	up := 0
	for _, s := range links {
		if s.Router == string(swp08.StateUp) {
			up++
		}
	}
	switch {
	case up > 0 && up == len(links):
		return stateHealthy
	case up > 0:
		return stateDegraded
	}
	return stateDown
}

// routerLink is the link of one router with the router's identity, as the settings give it now.
type routerLink struct {
	UID, Prefix, Title string
	Status             statusMsg
}

// routerLinks gives the link of each router in the order of the settings; the first router comes first. Caller
// holds mu.
func (a *App) routerLinks() []routerLink {
	r := a.r()
	var out []routerLink
	for _, u := range a.unitsInOrder(r) {
		rc := r.routerOf(u.uid())
		out = append(out, routerLink{UID: u.uid(), Prefix: rc.Prefix, Title: rc.Title, Status: u.status})
	}
	return out
}

func linkStates(links []routerLink) []statusMsg {
	out := make([]statusMsg, len(links))
	for i, l := range links {
		out[i] = l.Status
	}
	return out
}

// statusBody is the body of GET /api/v1/status and of each frame of the status topic. It has the state as a whole and
// one element for each router. Caller holds mu.
func (a *App) statusBody(withDetail bool) map[string]any {
	links := a.routerLinks()
	m := map[string]any{}
	routers := make([]map[string]any, len(links))
	for i, l := range links {
		e := linkStatus(l.Status, withDetail)
		e["uid"], e["prefix"], e["title"] = l.UID, l.Prefix, l.Title
		routers[i] = e
	}
	m["state"], m["routers"] = wholeState(linkStates(links)), routers
	m["server"], m["version"], m["uptimeSeconds"] = "up", VersionLabel(), int64(time.Since(a.started).Seconds())
	return m
}

// healthRouter is one router in the body of /api/health.
type healthRouter struct {
	UID    string `json:"uid"`
	Prefix string `json:"prefix"`
	Title  string `json:"title"`
	Router string `json:"router"`
	Addr   string `json:"targetAddress"`
	Err    string `json:"err,omitempty"`
}

// healthMsg: a monitor reads state, and routers for each link.
type healthMsg struct {
	Server        string         `json:"server"`
	State         string         `json:"state"`
	Routers       []healthRouter `json:"routers"`
	Version       string         `json:"version"`
	UptimeSeconds int64          `json:"uptimeSeconds"`
	Browsers      int            `json:"browsers"`
}

// serveHealth answers 200 only when Sode is Healthy, so a plain monitor alerts when any router is lost.
func (a *App) serveHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	links := a.routerLinks()
	browsers := len(a.clients)
	a.mu.Unlock()
	m := healthMsg{Server: "up", State: wholeState(linkStates(links)),
		Version: VersionLabel(), UptimeSeconds: int64(time.Since(a.started).Seconds()), Browsers: browsers}
	for _, l := range links {
		m.Routers = append(m.Routers, healthRouter{l.UID, l.Prefix, l.Title, l.Status.Router, l.Status.Addr, l.Status.Err})
	}
	code := http.StatusOK
	if m.State != stateHealthy {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(m)
}
