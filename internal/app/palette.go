package app

// The Palette's Panels rows (web/palettecatalogue.js). The socket carries the browser's own sign-in and never a panel
// key. So a panel page's Palette lists the panels of that person, not of the panel's API client.

type paletteName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type palettePanelsResult struct {
	opResult
	// Locked: the panel page lock is on and this browser does not open it, so the list is empty.
	Locked bool          `json:"locked,omitempty"`
	Panels []paletteName `json:"panels"`
}

// palettePanels lists the panels the connection's user can open, by the same reach as GET /api/v1/panels. A Guest gets
// the panels shared with Everyone.
func (a *App) palettePanels(c *client) (list []paletteName, locked bool) {
	a.mu.Lock()
	cfg := a.r().cfg
	allowed := a.pageAllowedMu(c, gatePanel)
	act := Actor{Kind: actorUser}
	if a.userOKMu(c, cfg) {
		u := c.u.user.Load()
		act.Name, act.User = u.Name, u.ID
	}
	a.mu.Unlock()
	list = []paletteName{}
	if !allowed {
		return list, true
	}
	cl := &apiCtx{Actor: act}
	for _, p := range a.panels.list() {
		if read, _ := a.panelReach(cl, p); read {
			list = append(list, paletteName{ID: p.ID, Name: p.Name, Type: p.Type})
		}
	}
	return list, false
}

func (a *App) servePalettePanels(c *client, m *inMsg) {
	list, locked := a.palettePanels(c)
	a.reply(c, palettePanelsResult{opResult: opResultFor("palettePanels", &m.ID, nil), Locked: locked, Panels: list})
}
