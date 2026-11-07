package app

import "errors"

// Access policy: what each browser message needs, in one table checked before dispatch. An
// unlisted type is dropped (TestPolicyCoversEveryMessage).
// Locks cover actions, not data: hello, names, tally, recent history stream to a locked
// browser (the lock screen is not a secret). Settings changes need the settings PIN
// (needAdmin) or, for low-risk ones, a trusted user (needTrusted), never the main lock: a
// locked wall panel must still save with the PIN. Locks require a settings PIN (Validate).
//
//	open: hi userUnlock userLock userSetPin unlock lock history
//	needTake: batch; needMain: refreshNames; needMain+Admin: clearHistory; needLog: sent uptime
//	needTrusted: config (Admin when it sets or clears the PIN), filterSets, presets
//	needAdmin: users userSignOutAll configExport configImportBegin configImport
//	profileSave: the user's own unlock or needTrusted; profileGet needTrusted; both re-checked by guardProfile.
type need uint8

const (
	needTake need = 1 << iota
	needMain
	needLog
	// needAdmin: settings PIN unlocked here (when set); re-checked under cfgMu by guardSave.
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
}

var policy = map[string]rule{
	"hi":         {}, // declares the page type and session token; answered with the lock state
	"userUnlock": {}, // the way out of the lock; counted in the user PIN pool
	"userLock":   {}, // locking is never refused
	"userSetPin": {},
	"unlock":     {}, // counted in the settings pool
	"lock":       {}, // forgetting an unlock is never refused
	"history":    {},

	"batch":        {need: needTake, why: "take"},
	"refreshNames": {need: needMain},
	// Erasing history is more than a take.
	"clearHistory": {need: needMain | needAdmin},

	// Admitting sent subscribes to the feed in the same step, so a lock can't slip in.
	"sent":   {need: needLog, admitted: func(c *client) { c.sent = true }},
	"uptime": {need: needLog},

	"config": {need: needTrusted, op: "config", noID: true,
		strict: func(m *inMsg) bool { return m.Pin != "" || m.ClearPin }},
	"filterSets": {need: needTrusted, op: "filterSets"},
	"presets":    {need: needTrusted, op: "presets"},
	// A user saves their own profile with no PIN; others need a trusted user or the PIN.
	"profileSave": {need: needTrusted, op: "profileSave", selfOf: func(m *inMsg) string { return m.Name }},
	"profileGet":  {need: needTrusted, op: "profileGet"},
	// PIN only, trusted or not: holds users, locks, router, profiles and PIN hashes. Begin
	// raises the read limit for the next message, so it must never be reachable without
	// the PIN.
	"configExport":      {need: needAdmin, op: "configExport"},
	"configImportBegin": {need: needAdmin, op: "configImportBegin"},
	"configImport":      {need: needAdmin, op: "configImport"},
	"userSignOutAll":    {need: needAdmin, op: "userSignOutAll"},
	"usersList":         {need: needAdmin, op: "usersList"},
	"users":             {need: needAdmin, op: "users"}, // and SaveUsers wants a settings PIN to exist
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
	case r.need&needTake != 0 && (!a.pageAllowedLocked(c, gateMain)),
		r.need&needMain != 0 && !a.pageAllowedLocked(c, gateMain),
		r.need&needLog != 0 && !a.pageAllowedLocked(c, gateLog):
		return errPageLocked
	case r.need&needAdmin != 0 && !a.settingsUnlockedLocked(c):
		return errLocked
	case r.need&needTrusted != 0 && !a.trustedLocked(c) && !a.isUserLocked(c, r.self):
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
	case t == "batch":
		a.command(c, why, outcome(err), "", err)
		for _, it := range m.Items {
			a.reply(c, takeResult{T: "takeResult", ID: it.ID, Err: err.Error()})
		}
	default:
		a.command(c, why, outcome(err), "", err)
	}
	if errors.Is(err, errPageLocked) {
		a.syncAccess(c)
	} else if r.op == "" {
		a.reply(c, simpleMsg{T: "locked"})
	}
}
