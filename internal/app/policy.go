package app

import (
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// Message table: what each browser message needs and what runs for it, in one place. The
// read loop looks the type up, checks access, then calls the handler; an unlisted type is
// dropped (TestPolicyCoversEveryMessage: every rule has a handler). A handler picks inline
// or goroutine itself: unlock and userUnlock stay inline so a following lock wins.
// Locks cover actions, not data: hello, names, tally, History stream to a locked
// browser (the lock screen is not a secret). Settings changes need the Advanced Settings PIN
// (needAdmin) or, for low-risk ones, a trusted user (needTrusted), never the main lock: a
// locked wall panel must still save with the PIN. Locks require an Advanced Settings PIN (Validate).
//
//	open: hi userUnlock userLock userSignOut userSetPin unlock lock history
//	needMain: `batch` `refreshNames`; needMain+Admin: clearHistory; needLog: sent uptime
//	needTrusted: config (Admin when it sets or clears the PIN), filterSets, presets
//	needAdmin: users userSignOutAll configExport configImportBegin configImport
//	profileSave: the user's own unlock or needTrusted; profileGet needTrusted; both re-checked by guardProfile.
type need uint8

const (
	needMain need = 1 << iota
	needLog
	// needAdmin: Advanced Settings PIN unlocked here (when set); re-checked under cfgMu by guardSave.
	needAdmin
	// needTrusted: the PIN, or unlocked as a Trusted user. Weaker than needAdmin; a rule
	// with both means the PIN.
	needTrusted
)

type rule struct {
	need need
	// op: failed refusals answer as "<op>Result" (locked) with the request id; "" = none.
	op   string
	noID bool
	why  string
	// strict: when true for a message, needTrusted becomes needAdmin (a save that sets or
	// clears the PIN).
	strict func(m *inMsg) bool
	// selfOf: the user a message is about; that user's own connection passes needTrusted.
	// forMsg fills self.
	selfOf func(m *inMsg) string
	self   string
	// admitted runs under mu on admit, so a lock can't slip between check and effect.
	admitted func(c *client)
	// handle runs after admit passed; connCtx is the read loop's per-connection state.
	handle func(a *App, c *client, m *inMsg, cc *connCtx)
	// onRefuse: extra replies after a refusal is logged (a take fails each route).
	onRefuse func(a *App, c *client, m *inMsg, err error)
}

// connCtx: per-connection state the read loop owns.
type connCtx struct {
	budget *readBudget
}

var policy = map[string]rule{
	// declares the page type and session token; answered with the lock state
	"hi": {handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.Hi(c, m.Page, m.Token) }},
	// the way out of the lock; counted in the user PIN pool. In line: a later lock is
	// handled after.
	"userUnlock": {handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveUserUnlock(c, m) }},
	// locking and signing out are never refused; with nobody signed in the Lock is page-only
	// and the server does nothing
	"userLock": {handle: func(a *App, c *client, _ *inMsg, _ *connCtx) {
		a.setHeld(c, true)
		a.command(c, "userLock", "ok", "", nil)
	}},
	"userSignOut": {handle: func(a *App, c *client, _ *inMsg, _ *connCtx) {
		a.SignOut(c)
		a.command(c, "userSignOut", "ok", "", nil)
	}},
	"userSetPin": {handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveUserSetPin(c, m) }},
	// counted in the settings pool. In line: a later lock is handled after, and one
	// connection checks one PIN at a time.
	"unlock": {handle: func(a *App, c *client, m *inMsg, _ *connCtx) {
		err := a.Unlock(c, m.Pin)
		a.command(c, "unlock", map[bool]string{true: "ok", false: "refused"}[err == nil], "", err)
		a.reply(c, opResultFor("unlock", &m.ID, err))
	}},
	// forgetting an unlock is never refused
	"lock": {handle: func(a *App, c *client, _ *inMsg, _ *connCtx) {
		a.Lock(c)
		a.command(c, "lock", "ok", "", nil)
	}},
	// Own keys: a signed-in user only (checked in the handler). Creating asks the user's own
	// PIN, in the change-PIN pool. Inline, like userSetPin.
	"keyCreate":  {handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveKeyCreate(c, m) }},
	"keyRevoke":  {handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveKeyRevoke(c, m) }},
	"keyListOwn": {handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveKeyListOwn(c, m) }},
	"history":    {handle: func(a *App, c *client, m *inMsg, _ *connCtx) { go a.history(c, m.Before, m.N) }},

	"batch": {need: needMain, why: "take",
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveTake(c, m) },
		onRefuse: func(a *App, c *client, m *inMsg, err error) {
			for _, it := range m.Items {
				a.reply(c, routeResult{T: "takeResult", ID: it.ID, Err: err.Error()})
			}
		}},
	"refreshNames": {need: needMain, handle: func(a *App, c *client, _ *inMsg, _ *connCtx) {
		a.command(c, "refreshNames", "sent", "", nil)
		go a.RefreshNames()
	}},
	// Erasing history is more than a take.
	"clearHistory": {need: needMain | needAdmin, handle: func(a *App, c *client, _ *inMsg, _ *connCtx) {
		a.command(c, "clearHistory", "ok", "", nil)
		go a.ClearHistory(c.who())
	}},

	// Admitting sent subscribes to the feed in the same step, so a lock can't slip in.
	"sent": {need: needLog, admitted: func(c *client) { c.sent = true },
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { go a.sentHistory(c, int64(m.Before), m.N) }},
	"uptime": {need: needLog, handle: func(a *App, c *client, m *inMsg, _ *connCtx) {
		a.reply(c, a.uptimeReply(m.ID, m.From, m.To, m.Slot))
	}},

	"config": {need: needTrusted, op: "config", noID: true,
		strict: func(m *inMsg) bool { return m.Pin != "" || m.ClearPin },
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveConfig(c, m) }},
	"filterSets": {need: needTrusted, op: "filterSets", handle: func(a *App, c *client, m *inMsg, _ *connCtx) {
		go func(sets []config.FilterSet, id int) {
			a.replyOp(c, "filterSets", id, len(sets), "set", a.SaveFilterSets(c, sets))
		}(m.Sets, m.ID)
	}},
	"presets": {need: needTrusted, op: "presets", handle: func(a *App, c *client, m *inMsg, _ *connCtx) {
		go func(presets []config.Preset, id int) {
			a.replyOp(c, "presets", id, len(presets), "preset", a.SavePresets(c, presets))
		}(m.Presets, m.ID)
	}},
	// A user saves their own profile with no PIN; others need a trusted user or the PIN.
	"profileSave": {need: needTrusted, op: "profileSave", selfOf: func(m *inMsg) string { return m.Name },
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { go a.serveProfileSave(c, m) }},
	"profileGet": {need: needTrusted, op: "profileGet",
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveProfileGet(c, m) }},
	// PIN only, trusted or not: holds users, locks, router, profiles and PIN hashes. Begin
	// raises the read limit for the next message, so it must never be reachable without
	// the PIN.
	"configExport": {need: needAdmin, op: "configExport",
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { go a.serveConfigExport(c, m) }},
	"configImportBegin": {need: needAdmin, op: "configImportBegin", handle: func(a *App, c *client, m *inMsg, cc *connCtx) {
		if cc.budget.Grant(m.Size) {
			// Answered either way after the limit is raised: the page sends the file
			// only after this, so a refusal is a normal result, not a too-big message.
			a.reply(c, opResult{T: "configImportBeginResult", OK: true, ID: &m.ID})
		} else {
			a.replyOp(c, "configImportBegin", m.ID, 0, "", errImportTooBig)
		}
	}},
	"configImport": {need: needAdmin, op: "configImport",
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveConfigImport(c, m) }},
	"userSignOutAll": {need: needAdmin, op: "userSignOutAll", handle: func(a *App, c *client, m *inMsg, _ *connCtx) {
		go func(name string, id int) {
			a.replyOp(c, "userSignOutAll", id, 0, "", a.SignOutAll(c, name))
		}(m.Name, m.ID)
	}},
	// Any key: the Advanced Settings PIN, even for a trusted user.
	"keyListAll":   {need: needAdmin, op: "keyListAll", handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveKeyListAll(c, m) }},
	"keyCreateFor": {need: needAdmin, op: "keyCreateFor", handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveKeyCreateFor(c, m) }},
	"keyRevokeAny": {need: needAdmin, op: "keyRevokeAny", handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveKeyRevokeAny(c, m) }},
	"keyRevokeAll": {need: needAdmin, op: "keyRevokeAll", handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.serveKeyRevokeAll(c, m) }},
	"usersList": {need: needAdmin, op: "usersList",
		handle: func(a *App, c *client, m *inMsg, _ *connCtx) { a.reply(c, a.usersList(m.ID)) }},
	// and SaveUsers wants an Advanced Settings PIN to exist
	"users": {need: needAdmin, op: "users", handle: func(a *App, c *client, m *inMsg, _ *connCtx) {
		if m.UsersReq == nil {
			return
		}
		go func(r usersReq, id int) {
			err := a.SaveUsers(c, r)
			a.replyOp(c, "users", id, 0, "", err)
		}(*m.UsersReq, m.ID)
	}},
}

func (r rule) forMsg(m *inMsg) rule {
	if r.selfOf != nil {
		r.self = r.selfOf(m)
	}
	if r.strict != nil && r.strict(m) && r.need&needTrusted != 0 {
		r.need = r.need&^needTrusted | needAdmin
	}
	return r
}

// admit: the page lock is reported ahead of the PIN (the lock screen must show).
func (a *App) admit(c *client, r rule) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case r.need&needMain != 0 && !a.pageAllowedMu(c, gateMain),
		r.need&needLog != 0 && !a.pageAllowedMu(c, gateLog):
		return errPageLocked
	case r.need&(needAdmin|needTrusted) != 0 && a.heldMu(c, a.r().cfg):
		return errPageLocked
	case r.need&needAdmin != 0 && !a.settingsUnlockedMu(c):
		return errLocked
	case r.need&needTrusted != 0 && !a.trustedMu(c) && !a.signedInAsMu(c, r.self):
		return errLocked
	}
	if r.admitted != nil {
		r.admitted(c)
	}
	return nil
}

// refuse: logs a "locked" command; a page lock answers with the lock state, a PIN with
// "locked"; a result type also gets a failed result so the page isn't left waiting.
func (a *App) refuse(c *client, t string, r rule, m *inMsg, err error) {
	why := r.why
	if why == "" {
		why = t
	}
	switch {
	case r.op != "":
		id := m.ID
		if r.noID {
			id = -1
		}
		a.replyOp(c, r.op, id, 0, "", err) // logs the command too
	default:
		a.command(c, why, outcome(err), "", err)
		if r.onRefuse != nil {
			r.onRefuse(a, c, m, err)
		}
	}
	if errors.Is(err, errPageLocked) {
		a.syncAccess(c)
	} else if r.op == "" {
		a.reply(c, simpleMsg{T: "locked"})
	}
}
