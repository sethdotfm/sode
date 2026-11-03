package app

import "errors"

// The access policy: what each browser-to-server message needs, in one table,
// checked once before anything is dispatched. A message type that is not in
// the table is dropped, so a new case in the read loop cannot go live
// unguarded (TestPolicyCoversEveryMessage fails until it is listed).
//
// What the locks cover is actions, not data. The main lock guards what
// changes things (batch, refreshNames, clearHistory); the log lock guards
// reading the communication log (sent, uptime). Data the page needs to look
// alive (hello, names, tally, recent history, history reads) keeps streaming
// to a locked browser by design: the lock screen covers the page, it is not a
// secret. Settings changes are guarded by the settings PIN (needAdmin) or,
// for the low-risk ones, a trusted user (needTrusted); never by the main
// lock: a locked-out wall panel (which has no lock screen) must still be able
// to save with the PIN. Locks can't be turned on without a settings PIN
// (Validate), so everything an admin message changes stays guarded.
//
// The table, one line per type:
//
//	open:           hi userUnlock userLock userSetPin unlock lock history
//	needTake:       batch
//	needMain:       refreshNames
//	needMain+Admin: clearHistory
//	needLog:        sent uptime
//	needTrusted:    config (needAdmin when it sets or clears the settings
//	                PIN), filterSets, salvos
//	needAdmin:      users
type need uint8

const (
	// needTake: the main lock.
	needTake need = 1 << iota
	needMain
	needLog
	// needAdmin: the settings PIN unlocked on this connection (when one is
	// set). Re-checked under cfgMu by guardSave, since a PIN can change
	// between this check and the save.
	needAdmin
	// needTrusted: the settings PIN, or this connection unlocked as a user
	// marked Trusted (still existing, same PIN). Weaker than needAdmin; a
	// rule holding both means the PIN.
	needTrusted
)

// rule is what one message type needs, and how a refusal is answered.
type rule struct {
	need need
	// op names the save a refusal answers as an opResult failure
	// ("<op>Result", locked) with the request's id; "" has none.
	op string
	// noID: the reply carries no id (config's never has).
	noID bool
	// why is the command logged on a refusal, when not the message type.
	why string
	// strict, when it says yes for a message, raises needTrusted to
	// needAdmin: a Settings save that sets or clears the settings PIN.
	strict func(m *inMsg) bool
	// admitted runs under mu when the message is let through, so the check and
	// what depends on it can't be pulled apart by a lock in between.
	admitted func(c *client)
}

var policy = map[string]rule{
	// Open: the lock screen itself. Without these a locked browser could
	// never say what page it is or unlock.
	"hi":         {}, // declares the page type and session token; answered with the lock state
	"userUnlock": {}, // the way out of the lock; counted in the user PIN pool
	"userLock":   {}, // locking is never refused
	// userSetPin: open, but the handler wants this connection unlocked as a
	// user and the old PIN (counted in the user pool). The settings PIN is
	// not involved: it is the user's own credential.
	"userSetPin": {},
	// Open: the settings PIN dialog's own messages, checked by the PIN.
	"unlock": {}, // counted in the settings pool
	"lock":   {}, // forgetting an unlock is never refused
	// Open: data. Recent keeps filling for a locked page (see above).
	"history": {},

	// Actions behind the main lock.
	"batch":        {need: needTake, why: "take"},
	"refreshNames": {need: needMain},
	// Also settings-PIN guarded: erasing history is more than a take.
	"clearHistory": {need: needMain | needAdmin},

	// Reads behind the log lock. Admitting a sent request subscribes the
	// connection to the feed in the same step, so a lock can't slip in
	// between the check and the feed starting.
	"sent":   {need: needLog, admitted: func(c *client) { c.sent = true }},
	"uptime": {need: needLog},

	// Low-risk saves: the settings PIN or a trusted user. A Settings save
	// that carries a PIN change needs the PIN itself.
	"config": {need: needTrusted, op: "config", noID: true,
		strict: func(m *inMsg) bool { return m.Pin != "" || m.ClearPin }},
	"filterSets": {need: needTrusted, op: "filterSets"},
	"salvos":     {need: needTrusted, op: "salvos"},
	// Users, locks and the settings PIN's own users: the PIN only.
	"users": {need: needAdmin, op: "users"}, // and SaveUsers wants a settings PIN to exist
}

// forMsg is r as it applies to m (strict raises the need to the PIN).
func (r rule) forMsg(m *inMsg) rule {
	if r.strict != nil && r.strict(m) && r.need&needTrusted != 0 {
		r.need = r.need&^needTrusted | needAdmin
	}
	return r
}

// admit checks c against r, once, and runs r.admitted if it passes. The page
// lock is reported ahead of the PIN, since the lock screen is what a locked
// page needs to show.
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
	case r.need&needTrusted != 0 && !a.trustedLocked(c):
		return errLocked
	}
	if r.admitted != nil {
		r.admitted(c)
	}
	return nil
}

// refuse answers a message admit turned away. The log gets a "locked" command
// row; a page lock is answered with the lock state (so the page shows its lock
// screen), a settings PIN with the "locked" message that asks for it; and a
// message with a result type also gets that, failed, so the page that sent it
// isn't left waiting.
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
