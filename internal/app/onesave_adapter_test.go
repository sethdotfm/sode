package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Older tests build the whole user list. These helpers make one Save fields from it, with live
// versions, so the tests use the real save path.

type userEdit struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Pin        string         `json:"pin"`
	ClearPin   bool           `json:"clearPin"`
	Role       string         `json:"role"`
	Persistent *bool          `json:"persistent"`
	Rights     *config.Rights `json:"rights"`
	NoAutoLock *bool          `json:"noAutoLock"`
}

type usersReq struct {
	Users       []userEdit          `json:"users"`
	MaxPin      int                 `json:"maxPinLen"`
	GuestRole   json.RawMessage     `json:"guestRole"`
	APIClients  *[]config.APIClient `json:"apiClients"`
	GuestRights *config.Rights      `json:"guestRights"`
	PanelOwners map[string]string   `json:"panelOwners"`
	Locks       json.RawMessage     `json:"locks"`
	AutoLock    json.RawMessage     `json:"autoLockMinutes"`
}

type apiSettingsReq struct {
	RateLimit    *int      `json:"rateLimit"`
	TakeCap      *int      `json:"takeCap"`
	GuestEnabled *bool     `json:"guestEnabled"`
	LogReads     *bool     `json:"logReads"`
	GuestScopes  *[]string `json:"guestScopes"`
}

func jsonOf(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// usersFields: the one Save fields of a whole-list users save against live.
func usersFields(live config.Config, req usersReq) (map[string]fieldEdit, error) {
	if present(req.Locks) || present(req.AutoLock) || present(req.GuestRole) {
		return nil, errors.New("the lock settings and Guest role moved to Settings > Access > Locks: reload this page")
	}
	if len(req.Users) == 0 && live.Locks.Any() {
		return nil, errors.New("a page lock is on: turn it off in Settings > Access before you remove the last user")
	}
	vers := hiddenVersions(live)
	for k, v := range fieldVersions(live) {
		vers[k] = v
	}
	out := map[string]fieldEdit{}
	put := func(k string, v any) { out[k] = fieldEdit{Value: jsonOf(v), Version: vers[k]} }
	if req.MaxPin != 0 {
		put("maxPinLen", req.MaxPin)
	}
	if req.GuestRights != nil {
		put("guestRights", req.GuestRights)
	}
	kept := map[string]bool{}
	for i, e := range req.Users {
		if e.ID == "" {
			put(fmt.Sprintf("user/new:%d/add", i), newUser{Name: e.Name, Pin: e.Pin, Role: e.Role, Persistent: e.Persistent,
				NoAutoLock: e.NoAutoLock, Rights: e.Rights})
			continue
		}
		if _, ok := live.FindUserByID(e.ID); !ok {
			return nil, errors.New(e.Name + " was removed meanwhile: reload the user list")
		}
		if kept[e.ID] {
			return nil, errors.New("two rows are the same user")
		}
		kept[e.ID] = true
		k := "user/" + e.ID + "/"
		put(k+"name", e.Name)
		put(k+"role", e.Role)
		if e.Persistent != nil {
			put(k+"persistent", *e.Persistent)
		}
		if e.Rights != nil {
			put(k+"rights", e.Rights)
		}
		if e.NoAutoLock != nil {
			put(k+"noAutoLock", *e.NoAutoLock)
		}
		if e.Pin != "" {
			put(k+"pin", e.Pin)
		} else if e.ClearPin {
			put(k+"pin", "")
		}
	}
	for _, u := range live.Users {
		if !kept[u.ID] {
			put("user/"+u.ID+"/remove", map[string]string{"owner": req.PanelOwners[u.ID]})
		}
	}
	if req.APIClients != nil {
		listed := map[string]bool{}
		for _, cl := range *req.APIClients {
			listed[strings.ToLower(cl.Name)] = true
			o, ok := live.FindAPIClient(cl.Name)
			if !ok {
				put("client/"+cl.Name+"/add", newClient{Scopes: cl.Scopes, Rights: cl.Rights})
				continue
			}
			put("client/"+o.Name+"/scopes", cl.Scopes)
			put("client/"+o.Name+"/rights", cl.Rights)
		}
		for _, cl := range live.APIClients {
			if !listed[strings.ToLower(cl.Name)] && cl.Panel == "" && cl.Owner == "" {
				put("client/"+cl.Name+"/remove", true)
			}
		}
	}
	return out, nil
}

// saveErr: the first refusal as the old saves reported it: Role, then invalid, then recheck.
func saveErr(res map[string]fieldResult, _ []interruption, err error) error {
	if err != nil {
		return err
	}
	order := []string{reasonRole, reasonApproval, reasonInvalid, reasonPrereq, reasonConflict, reasonRecheck}
	for _, why := range order {
		for _, k := range slices.Sorted(maps.Keys(res)) {
			r := res[k]
			if r.Status != "refused" || r.Reason != why {
				continue
			}
			switch why {
			case reasonRole, reasonApproval:
				return roleErr{need: config.RoleAdministrator}
			case reasonRecheck:
				return recheckErr{}
			}
			for _, e := range []error{errNoAdministrator, errFirstUser} {
				if r.Err == e.Error() {
					return e
				}
			}
			return errors.New(r.Err)
		}
	}
	return nil
}

// SaveUsers is all or nothing, as these tests expect: a partly refused save is put back.
func (a *App) SaveUsers(c *client, req usersReq) error {
	before := a.r().cfg
	fields, err := usersFields(before, req)
	if err != nil {
		return err
	}
	err = saveErr(a.saveSettingsFields(c, fields, nil, "", ""))
	if err != nil {
		a.cfgMu.Lock()
		_ = a.applyConfig(func(cur *config.Config) error { *cur = before; return nil })
		a.cfgMu.Unlock()
	}
	return err
}

func (a *App) SaveAPISettings(c *client, req apiSettingsReq) error {
	return saveErr(a.saveSettingsFields(c, apiFields(a.r().cfg, req), nil, "", ""))
}

// askUsers sends a whole-list users save as one Save over p and answers in the old result's
// shape: ok, recheck and err.
func (p *wsPeer) askUsers(a *App, id int, req map[string]any) map[string]any {
	p.t.Helper()
	var r usersReq
	if err := json.Unmarshal(jsonOf(req), &r); err != nil {
		p.t.Fatal(err)
	}
	fields, err := usersFields(a.r().cfg, r)
	if err != nil {
		return map[string]any{"ok": false, "err": err.Error()}
	}
	return p.askFields(id, fields)
}

func (p *wsPeer) askAPI(a *App, id int, req map[string]any) map[string]any {
	p.t.Helper()
	var r apiSettingsReq
	if err := json.Unmarshal(jsonOf(req), &r); err != nil {
		p.t.Fatal(err)
	}
	return p.askFields(id, apiFields(a.r().cfg, r))
}

func (p *wsPeer) askFields(id int, fields map[string]fieldEdit) map[string]any {
	p.t.Helper()
	m := p.ask("settingsSave", id, map[string]any{"fields": fields})
	if m["ok"] != true {
		return m
	}
	b, _ := json.Marshal(m["results"])
	var res map[string]fieldResult
	json.Unmarshal(b, &res)
	if err := saveErr(res, nil, nil); err != nil {
		out := map[string]any{"ok": false, "err": err.Error(), "recheck": isRecheckErr(err)}
		for _, k := range slices.Sorted(maps.Keys(res)) {
			if r := res[k]; r.Status == "refused" && r.Err != "" {
				out["err"] = r.Err
				break
			}
		}
		return out
	}
	return map[string]any{"ok": true}
}

func apiFields(live config.Config, req apiSettingsReq) map[string]fieldEdit {
	api := live.API
	if req.RateLimit != nil {
		api.RateLimit = *req.RateLimit
	}
	if req.TakeCap != nil {
		api.TakeCap = *req.TakeCap
	}
	if req.GuestEnabled != nil {
		api.GuestEnabled = *req.GuestEnabled
	}
	if req.LogReads != nil {
		api.LogReads = *req.LogReads
	}
	fields := map[string]fieldEdit{"api": {Value: jsonOf(api), Version: fieldVersion(live, "api")}}
	if req.GuestScopes != nil {
		fields["guestApiScopes"] = fieldEdit{Value: jsonOf(*req.GuestScopes), Version: fieldVersion(live, "guestApiScopes")}
	}
	return fields
}
