package app

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// A panel's tie line setting (docs/tielines-design.md, Panels; GLOSSARY.md Reserved tie line). Off refuses every
// route across routers. Reserved names one tie line for each one-way link between two routers, and the panel always
// uses it. The server applies it when the Actor is a panel (panelTieRule). It is a field beside the layout, not in
// settings, so Push, templates, checkpoints and Checkpoint sets leave it out. Save a copy and an Inbox card drop it.

const (
	panelTiesOff      = "off"
	panelTiesReserved = "reserved"
)

// PanelTies is the setting. A panel with no setting is Off. Reserved holds tie line ids.
type PanelTies struct {
	Mode     string   `yaml:"mode" json:"mode"`
	Reserved []string `yaml:"reserved,omitempty" json:"reserved,omitempty"`
}

// reserving: the panel's setting is Reserved.
func (t *PanelTies) reserving() bool { return t != nil && t.Mode == panelTiesReserved }

// linkKey is one one-way link: the router of the upstream end, then the router of the downstream end. A blank
// router is the first router.
func linkKey(cfg config.Config, t config.TieLine) [2]string {
	uid := func(r string) string {
		if r == "" {
			return cfg.DefaultRouter().UID
		}
		return r
	}
	return [2]string{uid(t.From.Router), uid(t.To.Router)}
}

// cleanPanelTies checks the setting against the tie lines that Sode has. Off becomes no setting, and Reserved keeps
// its list in the order sent. An id that is no tie line goes. One link gets one tie line.
func cleanPanelTies(cfg config.Config, t *PanelTies) (*PanelTies, error) {
	if t == nil {
		return nil, nil
	}
	switch t.Mode {
	case "", panelTiesOff:
		return nil, nil
	case panelTiesReserved:
	default:
		return nil, fmt.Errorf("tieLines.mode must be %q or %q", panelTiesOff, panelTiesReserved)
	}
	out := &PanelTies{Mode: panelTiesReserved, Reserved: []string{}}
	links := map[[2]string]string{}
	for _, id := range t.Reserved {
		i := slices.IndexFunc(cfg.TieLines, func(l config.TieLine) bool { return l.ID == id })
		// A tie line that an Administrator deleted goes, so a panel read sent back still saves.
		if i < 0 || slices.Contains(out.Reserved, id) {
			continue
		}
		k := linkKey(cfg, cfg.TieLines[i])
		if other, ok := links[k]; ok {
			return nil, fmt.Errorf("tieLines.reserved: %s and %s join the same two routers in the same direction; Reserve one tie line for each link", tieLineName(cfg, other), cfg.TieLines[i].Name)
		}
		links[k] = id
		out.Reserved = append(out.Reserved, id)
	}
	return out, nil
}

// tieLineName is tieNameOf with the settings alone.
func tieLineName(cfg config.Config, id string) string { return tieNameOf(&runtime{cfg: cfg}, id) }

// errTieReserved: one tie line can be Reserved by one panel only (docs/tielines-design.md, Panels, rule 6).
func errTieReserved(tie, panel string) error {
	return &panelClientFail{http.StatusConflict, "conflict", fmt.Sprintf("%s is Reserved by panel %s; one tie line can be Reserved by one panel only", tie, panel)}
}

// tieClashLocked: the first tie line of p that another panel Reserves, with that panel. Caller holds s.mu.
func (s *panelStore) tieClashLocked(p Panel) (string, Panel, bool) {
	if !p.TieLines.reserving() {
		return "", Panel{}, false
	}
	for _, id := range p.TieLines.Reserved {
		for _, q := range s.by {
			if q.ID != p.ID && q.TieLines.reserving() && slices.Contains(q.TieLines.Reserved, id) {
				return id, q, true
			}
		}
	}
	return "", Panel{}, false
}

// tieClash is tieClashLocked as a save error. The store gives the id; checkPanel names the tie line first.
func (s *panelStore) tieClashErrLocked(p Panel) error {
	if id, q, ok := s.tieClashLocked(p); ok {
		return errTieReserved(id, q.Name)
	}
	return nil
}

// checkPanelTies cleans p's setting and refuses a tie line that another panel Reserves, with the names.
func (a *App) checkPanelTies(p *Panel) error {
	cfg := a.r().cfg
	t, err := cleanPanelTies(cfg, p.TieLines)
	if err != nil {
		return err
	}
	p.TieLines = t
	a.panels.mu.Lock()
	id, q, clash := a.panels.tieClashLocked(*p)
	a.panels.mu.Unlock()
	if clash {
		return errTieReserved(tieLineName(cfg, id), q.Name)
	}
	return nil
}

// dropTieClash removes from p each tie line that another panel Reserves, for an import that must not fail on it.
func (a *App) dropTieClash(p *Panel) {
	if !p.TieLines.reserving() {
		return
	}
	a.panels.mu.Lock()
	defer a.panels.mu.Unlock()
	keep := &PanelTies{Mode: panelTiesReserved, Reserved: []string{}}
	for _, id := range p.TieLines.Reserved {
		one := Panel{ID: p.ID, TieLines: &PanelTies{Mode: panelTiesReserved, Reserved: []string{id}}}
		if _, _, clash := a.panels.tieClashLocked(one); !clash {
			keep.Reserved = append(keep.Reserved, id)
		}
	}
	p.TieLines = keep
}

// tieReserves: tie line id → the name of the panel that Reserves it, for the planner and GET /tielines. Read it
// before mu: the panel store is never read under mu.
func (a *App) tieReserves() map[string]panelRef {
	out := map[string]panelRef{}
	for _, p := range a.panels.list() {
		if p.TieLines.reserving() {
			for _, id := range p.TieLines.Reserved {
				out[id] = panelRef{ID: p.ID, Name: p.Name}
			}
		}
	}
	return out
}

type panelRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func reserveNames(m map[string]panelRef) map[string]string {
	out := map[string]string{}
	for id, p := range m {
		out[id] = p.Name
	}
	return out
}

// panelTies is what a panel caller can use for routes across routers. only is nil for a caller that is no panel.
type panelTies struct {
	panel string // the name of the panel
	only  map[string]bool
}

// panelTieRule applies a panel's tie line setting to a take with a route across routers (docs/tielines-design.md,
// Panels, rule 2). A caller that is no panel gets nil. A panel at Off gets 403 tie_lines_off. A route with ties
// through a tie line that the panel does not Reserve gets 409 conflict, since the panel always uses its own.
func (a *App) panelTieRule(rt *runtime, cl *apiCtx, routes []tieRoute) (*panelTies, *apiProblem) {
	if cl == nil || cl.Actor.panel == "" {
		return nil, nil
	}
	p, ok := a.panels.get(cl.Actor.panel)
	if !ok || !p.TieLines.reserving() {
		name := cl.Actor.panel
		if ok {
			name = p.Name
		}
		return nil, problem(http.StatusForbidden, "tie_lines_off", "panel %s has tie lines Off, so Sode cannot send a route across routers", name)
	}
	pt := &panelTies{panel: p.Name, only: map[string]bool{}}
	for _, id := range p.TieLines.Reserved {
		pt.only[id] = true
	}
	for _, r := range routes {
		for _, id := range r.ties {
			if !pt.only[id] {
				return nil, problem(http.StatusConflict, "conflict", "%s is not Reserved by panel %s; the panel uses only its Reserved tie lines", tieNameOf(rt, id), p.Name)
			}
		}
	}
	return pt, nil
}

// panelNoPath is the message of a 409 no_path for a panel: why its Reserved tie lines make no path (rules 5, 9 and
// 10). reasons are the reasons of the first route with no usable path.
func panelNoPath(rt *runtime, panel string, reasons []tieplan.Reason) string {
	router := func(uid string) string {
		return rt.cfg.RouterName(rt.routerOf(uid))
	}
	for _, why := range reasons {
		switch why.Why {
		case "taken":
			return fmt.Sprintf("this press needs two sources through %s, which panel %s Reserves; Sode sent nothing", tieNameOf(rt, why.Tie), panel)
		case "protected":
			return fmt.Sprintf("%s is protected by %s; Sode sent nothing", tieNameOf(rt, why.Tie), why.By)
		}
	}
	for _, why := range reasons {
		switch why.Why {
		case "no_tie_line":
			return fmt.Sprintf("panel %s has no Reserved tie line from %s to %s; Sode sent nothing", panel, router(why.From), router(why.To))
		case "router_down":
			return fmt.Sprintf("%s is down; Sode sent nothing", router(why.Router))
		}
	}
	return fmt.Sprintf("the Reserved tie lines of panel %s make no path; Sode sent nothing", panel)
}
