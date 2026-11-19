package app

import "github.com/sethdotfm/sode/internal/config"

// Len: whether a PIN is set (0 = none); never a hash.
type usersListMsg struct {
	T     string     `json:"t"` // "usersListResult"
	ID    int        `json:"id"`
	OK    bool       `json:"ok"`
	Users []userView `json:"users"`
	// Me is the id of the user asking, the default new owner of a removed user's panels.
	Me string `json:"me"`
	// Never before the PIN.
	APIClients  []config.APIClient `json:"apiClients"`
	GuestRights config.Rights      `json:"guestRights"`
	GuestScopes []string           `json:"guestScopes"`
	// Not in the hello (hiddenVersions).
	Versions      map[string]string  `json:"versions"`
	AccessPresets []accessPresetView `json:"accessPresets"`
}

type userView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Len        int    `json:"len"`
	Role       string `json:"role"`
	Persistent bool   `json:"persistent,omitempty"`
	NoAutoLock bool   `json:"noAutoLock,omitempty"`
	PinNeeded  bool   `json:"pinNeeded,omitempty"`
	// Rights has no omitempty: an unset list is "all", an empty one "none".
	Rights config.Rights `json:"rights"`
	// PanelClients are the panel clients this user's panels have, for the Rights editor to list.
	PanelClients []panelClientNote `json:"panelClients"`
	// Presets are the user's Me presets, so the Rights editor can turn their names into ids.
	Presets []presetNote `json:"presets"`
	Panels  int          `json:"panels,omitempty"`
	// Clients: the API clients the user owns, which go with them (dropOwnerless).
	Clients []string `json:"clients,omitempty"`
}

type presetNote struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *App) usersList(c *client, id int) usersListMsg {
	cfg := a.r().cfg.Clone()
	m := usersListMsg{T: "usersListResult", ID: id, OK: true, Me: c.userID(), Users: []userView{},
		APIClients: cfg.APIClients, GuestRights: cfg.GuestRights, GuestScopes: cfg.GuestAPIScopes, Versions: hiddenVersions(cfg), AccessPresets: accessPresetViews()}
	if m.APIClients == nil {
		m.APIClients = []config.APIClient{}
	}
	for i := range m.APIClients {
		m.APIClients[i].OwnerName = cfg.UserName(m.APIClients[i].Owner)
	}
	for _, u := range cfg.Users {
		pc := a.panelClientsOf(cfg, u)
		if pc == nil {
			pc = []panelClientNote{}
		}
		ps := []presetNote{}
		for _, p := range u.Profile.Presets {
			ps = append(ps, presetNote{p.ID, p.Name})
		}
		var own []string
		for _, cl := range cfg.APIClients {
			if cl.Owner == u.ID {
				own = append(own, cl.Name)
			}
		}
		m.Users = append(m.Users, userView{ID: u.ID, Name: u.Name, Len: u.Len, Role: u.Role, Persistent: u.Persistent,
			NoAutoLock: u.NoAutoLock, PinNeeded: u.PinNeeded, Rights: u.Rights, PanelClients: pc, Presets: ps, Panels: a.panels.ownedBy(u.ID),
			Clients: own})
	}
	return m
}
