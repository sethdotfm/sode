package app

import (
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// Message table: what each browser message needs and what runs for it, in one place. The
// read loop looks the type up, checks access, then calls the handler; an unlisted type is
// dropped (TestPolicyCoversEveryMessage: every rule has a handler). A handler picks inline
// or goroutine itself: userSignIn stays inline so a following lock wins.
// Locks cover actions, not data: hello, names, tally, History stream to a locked
// browser (the lock screen is not a secret). Every rule names the Role it needs (roles.go,
// TestPolicyCoversEveryMessage); settings changes need a Role, never the main lock.
// Administrator rules need an Administrator signed in on that connection. Save handlers
// check the Role again under cfgMu (guardSave, guardProfile).
type need uint8

const (
	needMain need = 1 << iota
	needLog
)

type rule struct {
	need need
	// role: roleAnyone or a config Role; the zero value is a test failure.
	role string
	// crit: whether this is a Critical action (recheck.go); the zero value is a test failure.
	crit crit
	// op: failed refusals answer as "<op>Result" (locked) with the request id; "" = none.
	op   string
	noID bool
	why  string
	// selfOf: the user a message is about; that user's own connection passes the role.
	// forMsg fills self.
	selfOf func(m *inMsg) string
	self   string
	// admitted runs under mu on admit, so a lock cannot slip between check and effect.
	admitted func(c *client)
	// handle runs after admit passed.
	handle func(a *App, c *client, m *inMsg)
	// write: a take or save. The read loop counts it so shutdown waits for it, and refuses it
	// after shutdown began; its handler calls m.finish when done, on every path.
	write bool
	// onRefuse: extra replies after a refusal is logged (a take fails each route).
	onRefuse func(a *App, c *client, m *inMsg, err error)
	// approvable: an Approval can stand in for the Role (approval.go). approver: the checked
	// approver of this one message; forMsg fills it.
	approvable bool
	approver   *config.User
	// setup: allowed while setup is pending (setup.go); every other message is refused.
	setup bool
}

var policy = map[string]rule{
	// declares the page type and session token; answered with the lock state
	"hi": {crit: critNever, role: roleAnyone, setup: true, handle: func(a *App, c *client, m *inMsg) { a.Hi(c, m.Page, m.Token) }},
	// the way out of the lock; counted in the user PIN pool. In line: a later lock is
	// handled after.
	"userSignIn": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveUserUnlock(c, m) }},
	// locking and signing out are never refused; with nobody signed in the Lock is page-only
	// and the server does nothing
	"userLock": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, _ *inMsg) {
		a.setHeld(c, true)
		a.command(c, "userLock", "ok", "", nil)
	}},
	"userSignOut": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, _ *inMsg) {
		a.SignOut(c)
		a.command(c, "userSignOut", "ok", "", nil)
	}},
	// the page's ping while its user is active (idle.go); never refused, never logged
	"activity":   {crit: critNever, role: roleAnyone, setup: true, handle: func(a *App, c *client, _ *inMsg) { a.Activity(c) }},
	"userSetPin": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveUserSetPin(c, m) }},
	// Own keys: a signed-in user only (checked in the handler). Creating asks the user's own
	// PIN, in the change-PIN pool. Inline, like userSetPin.
	"keyCreate":  {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveKeyCreate(c, m) }},
	"keyRevoke":  {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveKeyRevoke(c, m) }},
	"sessionKey": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveSessionKey(c, m) }},
	// the banner's Clear menu; a signed-in user only, checked in the handler
	"panelAlarm": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveAlarm(c, m) }},
	"keyListOwn": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveKeyListOwn(c, m) }},
	// API clients from My user (ownclients.go): any Role, a signed-in user only, checked in the
	// handler. Within the user's own Rights, so no Administrator. Inline, like keyCreate.
	"myClients":         {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveMyClients(c, m) }},
	"myClientSave":      {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveMyClientSave(c, m) }},
	"myClientDelete":    {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveMyClientDelete(c, m) }},
	"myClientKey":       {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveMyClientKey(c, m) }},
	"myClientKeyRevoke": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveMyClientKeyRevoke(c, m) }},
	// PIN recheck (recheck.go): the signed-in user's own PIN, in that name's pool. In line,
	// like userSignIn.
	"pinRecheck": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.servePinRecheck(c, m) }},
	// Step in and Step out (stepin.go): in line, like userSignIn.
	"stepIn":  {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveStepIn(c, m) }},
	"stepOut": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, _ *inMsg) { a.StepOut(c) }},
	// Request approval (approvalrequest.go). The handlers check who can ask, answer and see cards;
	// the frozen action is checked again when it runs. Answer is in line, like pinRecheck.
	"approvalAsk":    {crit: critNever, role: roleAnyone, op: "approvalAsk", handle: func(a *App, c *client, m *inMsg) { a.serveApprovalAsk(c, m) }},
	"approvalAnswer": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveApprovalAnswer(c, m) }},
	"approvalCancel": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, _ *inMsg) { a.serveApprovalCancel(c) }},
	"approvalCards":  {crit: critNever, role: roleAnyone, setup: true, handle: func(a *App, c *client, m *inMsg) { a.serveApprovalCards(c, m) }},
	"history":        {crit: critNever, role: roleAnyone, setup: true, handle: func(a *App, c *client, m *inMsg) { go a.history(c, m.Before, m.N) }},

	// The setup wizard (setup.go). Finish is a write: shutdown waits for it.
	"setupClaim":      {crit: critNever, role: roleAnyone, setup: true, handle: func(a *App, c *client, m *inMsg) { go a.serveSetupClaim(c, m) }},
	"setupRouter":     {crit: critNever, role: roleAnyone, setup: true, handle: func(a *App, c *client, m *inMsg) { go a.serveSetupRouter(c, m) }},
	"setupRestoreKey": {crit: critNever, role: roleAnyone, setup: true, handle: func(a *App, c *client, m *inMsg) { go a.serveSetupRestoreKey(c, m) }},
	"setupFinish":     {crit: critNever, role: roleAnyone, setup: true, write: true, handle: func(a *App, c *client, m *inMsg) { go a.serveSetupFinish(c, m) }},

	"take": {crit: critNever, role: config.RoleStandard, need: needMain, why: "take", write: true,
		handle: func(a *App, c *client, m *inMsg) { a.serveTake(c, m) },
		onRefuse: func(a *App, c *client, m *inMsg, err error) {
			for _, it := range m.Items {
				a.reply(c, routeResult{T: "takeResult", ID: it.ID, Err: err.Error()})
			}
		}},
	"rereadNames": {crit: critNever, role: config.RoleStandard, need: needMain, handle: func(a *App, c *client, _ *inMsg) {
		a.command(c, "rereadNames", "sent", "", nil)
		go a.RefreshNames()
	}},
	// Erasing history is more than a take.
	"clearHistory": {crit: critAlways, role: config.RoleAdministrator, need: needMain, write: true, approvable: true, handle: func(a *App, c *client, m *inMsg) {
		a.command(c, "clearHistory", "ok", approvedNote(m.approver, ""), nil)
		go func() {
			defer m.finish()
			a.ClearHistory(c.who())
		}()
	},
		// The page waits for a result; a page lock answers with the lock state instead.
		onRefuse: func(a *App, c *client, _ *inMsg, err error) {
			if !errors.Is(err, errPageLocked) {
				a.reply(c, opResultFor("clearHistory", nil, err))
			}
		}},

	// Admitting log subscribes to the feed in the same step, so a lock cannot slip in.
	"log": {crit: critNever, role: config.RoleStandard, need: needLog, admitted: func(c *client) { c.sent = true },
		handle: func(a *App, c *client, m *inMsg) { go a.sentHistory(c, int64(m.Before), m.N) }},
	"uptime": {crit: critNever, role: config.RoleStandard, need: needLog, handle: func(a *App, c *client, m *inMsg) {
		a.reply(c, a.uptimeReply(m.ID, m.From, m.To, m.Slot))
	}},

	"config": {crit: critByFields, role: config.RoleTrusted, op: "config", noID: true, write: true,
		handle: func(a *App, c *client, m *inMsg) { a.serveConfig(c, m) }},
	// the one Save: a Standard user passes here and gets a result per field (onesave.go)
	"settingsSave": {crit: critByFields, role: config.RoleStandard, op: "settingsSave", write: true, approvable: true,
		handle: func(a *App, c *client, m *inMsg) { a.serveSettingsSave(c, m) }},
	// the review's Interruptions (interruptions.go): it writes nothing, and the save checks again
	"settingsReview": {crit: critNever, role: config.RoleStandard, op: "settingsReview",
		handle: func(a *App, c *client, m *inMsg) { a.serveSettingsReview(c, m) }},
	"filterSets": {crit: critNever, role: config.RoleTrusted, op: "filterSets", write: true, handle: func(a *App, c *client, m *inMsg) {
		go func(sets []config.FilterSet, id int) {
			defer m.finish()
			a.replyOp(c, "filterSets", id, len(sets), "set", a.SaveFilterSets(c, sets))
		}(m.Sets, m.ID)
	}},
	"presets": {crit: critNever, role: config.RoleTrusted, op: "presets", write: true, handle: func(a *App, c *client, m *inMsg) {
		go func(presets []config.Preset, id int) {
			defer m.finish()
			a.replyOp(c, "presets", id, len(presets), "preset", a.SavePresets(c, presets))
		}(m.Presets, m.ID)
	}},
	// A user saves their own profile; another's needs a Trusted user.
	"profileSave": {crit: critNever, role: config.RoleTrusted, op: "profileSave", selfOf: func(m *inMsg) string { return m.Name }, write: true,
		handle: func(a *App, c *client, m *inMsg) {
			go func() {
				defer m.finish()
				a.serveProfileSave(c, m)
			}()
		}},
	// Copy this browser to My user: the connection's own user only, checked in the handler.
	"browserCopy": {crit: critNever, role: roleAnyone, op: "browserCopy", write: true,
		handle: func(a *App, c *client, m *inMsg) {
			go func() {
				defer m.finish()
				a.serveBrowserCopy(c, m)
			}()
		}},
	"profileGet": {crit: critNever, role: config.RoleTrusted, op: "profileGet",
		handle: func(a *App, c *client, m *inMsg) { a.serveProfileGet(c, m) }},
	// A session token for the export routes (exportitems.go, ownexport.go), which take it in a header and check the Role. A Guest gets none (ExportSession).
	"exportSession": {crit: critNever, role: config.RoleStandard, op: "exportSession",
		handle: func(a *App, c *client, m *inMsg) { a.serveExportSession(c, m) }},
	"userSignOutAll": {crit: critAlways, role: config.RoleAdministrator, op: "userSignOutAll", write: true, handle: func(a *App, c *client, m *inMsg) {
		go func(name string, id int) {
			defer m.finish()
			a.replyOp(c, "userSignOutAll", id, 0, "", a.SignOutAll(c, name))
		}(m.Name, m.ID)
	}},
	// Any key: Administrator only.
	"keyListAll":   {crit: critNever, role: config.RoleAdministrator, op: "keyListAll", handle: func(a *App, c *client, m *inMsg) { a.serveKeyListAll(c, m) }},
	"keyCreateFor": {crit: critNever, role: config.RoleAdministrator, op: "keyCreateFor", handle: func(a *App, c *client, m *inMsg) { a.serveKeyCreateFor(c, m) }},
	"keyRevokeAny": {crit: critNever, role: config.RoleAdministrator, op: "keyRevokeAny", handle: func(a *App, c *client, m *inMsg) { a.serveKeyRevokeAny(c, m) }},
	"keyRevokeAll": {crit: critAlways, role: config.RoleAdministrator, op: "keyRevokeAll", handle: func(a *App, c *client, m *inMsg) { a.serveKeyRevokeAll(c, m) }},
	// Send a copy (inboxsend.go): any signed-in user, with or without a PIN, checked in the handler.
	// Accepting writes only the user's own items, so it is never Critical.
	"inboxList":     {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { a.serveInboxList(c, m) }},
	"inboxSend":     {crit: critNever, role: roleAnyone, write: true, handle: func(a *App, c *client, m *inMsg) { go a.serveInboxSend(c, m) }},
	"inboxReview":   {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { go a.serveInboxReview(c, m) }},
	"inboxAccept":   {crit: critNever, role: roleAnyone, write: true, handle: func(a *App, c *client, m *inMsg) { go a.serveInboxAccept(c, m) }},
	"inboxDecline":  {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { go a.serveInboxDecline(c, m) }},
	"inboxWithdraw": {crit: critNever, role: roleAnyone, handle: func(a *App, c *client, m *inMsg) { go a.serveInboxWithdraw(c, m) }},
	// Swap (swap.go): everyone's list is Administrator only, checked in the handler. swapMine writes only the
	// user's own items, so it is never Critical.
	"swapList": {crit: critNever, role: config.RoleStandard, op: "swapList", handle: func(a *App, c *client, m *inMsg) { go a.serveSwapList(c, m) }},
	"swapMine": {crit: critNever, role: config.RoleStandard, op: "swapMine", write: true, handle: func(a *App, c *client, m *inMsg) { go a.serveSwapMine(c, m) }},
	"usersList": {crit: critNever, role: config.RoleAdministrator, op: "usersList",
		handle: func(a *App, c *client, m *inMsg) { a.reply(c, a.usersList(c, m.ID)) }},
}

func (r rule) forMsg(m *inMsg) rule {
	if r.selfOf != nil {
		r.self = r.selfOf(m)
	}
	r.approver = m.approver
	return r
}

// admit: the page lock is reported ahead of the PIN (the lock screen must show).
func (a *App) admit(c *client, r rule) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !r.setup && a.setupPending() {
		return errSetup
	}
	switch {
	case r.need&needMain != 0 && !a.pageAllowedMu(c, gateMain),
		r.need&needLog != 0 && !a.pageAllowedMu(c, gateLog):
		return errPageLocked
	case r.self != "" && a.signedInAsMu(c, r.self):
	case r.approver != nil:
		if r.role != roleAnyone && !config.RoleAtLeast(approverRole(a.r().cfg, r.approver), r.role) {
			return cannotApprove(r.approver, r.role)
		}
	default:
		if err := a.needRoleMu(c, r.role); err != nil {
			return err
		}
	}
	// An Administrator's Approval counts as their PIN recheck for this one action.
	adminApproved := r.approver != nil && approverRole(a.r().cfg, r.approver) == config.RoleAdministrator
	if r.crit == critAlways && !a.recheckedMu(c) && !adminApproved {
		return recheckErr{}
	}
	if r.admitted != nil {
		r.admitted(c)
	}
	return nil
}

// refuse: logs a "locked" command. A page lock answers with the lock state. A result type
// also gets a failed result so the page is not left waiting.
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
	}
}
