package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Request approval (GLOSSARY.md, docs/agents/users-locks.md). Cards follow state: cardsSyncMu runs after every access
// push, so a late page or a Step in gets the card with the time left.

const defaultApprovalWait = 2 * time.Minute

type approvalRequest struct {
	id     string
	from   *client
	userID string // the requester's sign-in when sent; a change cancels (a Lock keeps it)
	who    string
	ip     string
	role   string // the requester's Role when sent; the action runs with it under a Lock
	need   string // the Role an approver needs
	what   string
	msg    inMsg // frozen; the result goes back with its ID
	// card content, worked out one time when sent
	changes []SavedField
	stops   []string
	until   time.Time
	timer   *time.Timer
	cards   map[*client]string // the user each card went to; a Step in or Step out sends it again
}

type approvalCardMsg struct {
	T             string       `json:"t"`
	Request       string       `json:"request"`
	What          string       `json:"what"`
	From          string       `json:"from"`
	Browser       string       `json:"browser"`
	Changes       []SavedField `json:"changes,omitempty"`
	Interruptions []string     `json:"interruptions,omitempty"`
	Left          int64        `json:"left"` // ms
}

type approvalWaitingMsg struct {
	T       string `json:"t"`
	Request string `json:"request"`
	What    string `json:"what"`
	Left    int64  `json:"left"` // ms
}

// end: approved, denied, timeout or cancelled; "" closes a card whose browser stopped qualifying.
type approvalEndMsg struct {
	T       string `json:"t"`
	Request string `json:"request"`
	End     string `json:"end"`
	By      string `json:"by"`
}

// No names: it goes to every page, signed in or not.
type approversMsg struct {
	T             string `json:"t"`
	Trusted       bool   `json:"trusted"`
	Administrator bool   `json:"administrator"`
}

var errNoApprover = errors.New("no one signed in can approve this")

var approvalWhat = map[string]string{"settingsSave": "Saving these changes", "clearHistory": "Clearing History"}

// askWhat: the words the approver and the Log see. A Free Protect names every destination it frees, as the page does.
func (a *App) askWhat(act *inMsg) string {
	if act.T == "protectClear" {
		reqs, _ := act.Protect.reqs()
		dsts := make([]int, len(reqs))
		for i, q := range reqs {
			dsts[i] = q.Dst
		}
		return freeWhat(a.dstLabels(dsts...))
	}
	return approvalWhat[act.T]
}

// freeWhat: "Freeing another user's Protect on 4 DSM2"; past freeShown destinations, a count of the others.
func freeWhat(labels []string) string {
	if len(labels) == 1 {
		return "Freeing another user's Protect on " + labels[0]
	}
	shown := labels[:min(len(labels), freeShown)]
	text := fmt.Sprintf("Freeing other users' Protects on %d destinations: %s", len(labels), strings.Join(shown, ", "))
	if more := len(labels) - len(shown); more > 0 {
		text += fmt.Sprintf(" and %d more", more)
	}
	return text
}

const freeShown = 5

// dstLabels: "4 DSM2" for each destination, the number as the page shows it and the router's name.
func (a *App) dstLabels(dsts ...int) []string {
	off := a.r().cfg.NumberOffset
	first := a.firstUnit()
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(dsts))
	for _, d := range dsts {
		l := strconv.Itoa(d + off)
		if n := first.router.dstNames[d]; n != "" {
			l += " " + n
		}
		out = append(out, l)
	}
	return out
}

// For a save, the highest Role among the fields that need an Approval from this Role; "" when none does.
func askNeed(act *inMsg, rl rule, role string) string {
	if act.T == "protectClear" {
		// Only another Sode user's Protect needs one; the handler checks the owner when it runs.
		return config.RoleAdministrator
	}
	if act.T != "settingsSave" {
		return rl.role
	}
	need := ""
	for key := range act.Fields {
		f, ok := lookupField(key)
		if !ok || fieldRefusal(role, f.section) != reasonApproval {
			continue
		}
		if e := sectionRoles[f.section].edit; need == "" || !config.RoleAtLeast(need, e) {
			need = e
		}
	}
	return need
}

// Fields outside role's view are left out: the save refuses them, and the approver must not see their values.
func (a *App) askChanges(fields map[string]fieldEdit, role string) ([]SavedField, []string) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	live := a.r().cfg
	next := live.Clone()
	x, _ := a.swapCtx(live, fields)
	var keys []string
	for key, e := range fields {
		if f, ok := lookupField(key); ok && fieldRefusal(role, f.section) != reasonRole && setField(&next, key, e.Value, x) == nil {
			keys = append(keys, key)
		}
	}
	next.Normalize()
	saved := a.savedFields(live, next, keys, x)
	if slices.Contains(keys, swapKey) {
		saved = append(saved, a.swapPanelsField(x, fields, next, false))
	}
	return saved, interruptionTexts(a.interruptions(live, next))
}

func (a *App) serveApprovalAsk(c *client, m *inMsg) {
	if err := a.approvalAsk(c, m.For, m.ID); err != nil {
		a.reply(c, opResultFor("approvalAsk", &m.ID, err))
	}
}

// approvalAsk answers an accepted request itself, so the result comes before approvalWaiting.
func (a *App) approvalAsk(c *client, act *inMsg, id int) error {
	if act == nil {
		return errors.New("nothing to approve")
	}
	rl, ok := ruleOf(act.T)
	if !ok || !rl.approvable {
		return errors.New("only a Settings save, Clear History or freeing a Protect can go through Request approval")
	}
	if act.T == "protectClear" {
		// The card must name every destination the frozen message frees.
		if _, err := act.Protect.reqs(); err != nil {
			return err
		}
	}
	var changes []SavedField
	var stops []string
	if act.T == "settingsSave" {
		changes, stops = a.askChanges(act.Fields, a.role(c))
	}
	what := a.askWhat(act)
	a.mu.Lock()
	role := a.roleMu(c)
	r := &approvalRequest{from: c, userID: c.userID(), who: c.who(), ip: c.ip, role: role, need: askNeed(act, rl, role),
		what: what, msg: *act, changes: changes, stops: stops, cards: map[*client]string{}}
	switch {
	case role == "":
		a.mu.Unlock()
		return errPageLocked
	case a.asks[c] != nil:
		a.mu.Unlock()
		return errors.New("this browser waits for an Approval already: cancel it first")
	case r.need == "" || config.RoleAtLeast(role, r.need):
		a.mu.Unlock()
		return errors.New("this needs no Approval")
	case !a.anyApproverMu(r):
		a.mu.Unlock()
		return errNoApprover
	}
	a.askSeq++
	r.id = strconv.Itoa(a.askSeq)
	r.msg.For, r.msg.Approval = nil, nil
	r.until = time.Now().Add(a.approvalWait)
	r.timer = time.AfterFunc(a.approvalWait, func() { a.endAsk(r, "timeout", "") })
	a.asks[c] = r
	a.deliverJSONMu(c, opResultFor("approvalAsk", &id, nil))
	a.deliverJSONMu(c, approvalWaitingMsg{T: "approvalWaiting", Request: r.id, What: r.what, Left: r.left()})
	a.cardsSyncMu()
	a.mu.Unlock()
	a.command(c, "approvalRequest", "sent", r.what+", needs "+config.RoleLabel(r.need), nil)
	return nil
}

func (r *approvalRequest) left() int64 { return max(time.Until(r.until).Milliseconds(), 1) }

// Caller holds mu.
func (a *App) qualifiesMu(q *client, r *approvalRequest) bool {
	cfg := a.r().cfg
	switch {
	case q == r.from, q.api != nil, !q.u.cards:
		return false
	case r.from.u.sess != nil && q.u.sess == r.from.u.sess: // the same sign-in in another tab
		return false
	case !a.userOKMu(q, cfg) || !q.u.user.Load().HasPin():
		return false
	}
	// The approver's own Role: the Guest role never lifts it (config.Acts).
	u, _ := cfg.FindUserByID(q.u.user.Load().ID)
	role, _ := cfg.ActsAs(u)
	return config.RoleAtLeast(role, r.need)
}

func (a *App) anyApproverMu(r *approvalRequest) bool {
	for q := range a.clients {
		if a.qualifiesMu(q, r) {
			return true
		}
	}
	return false
}

// false when r ended already. The caller writes the Log row, outside mu.
func (a *App) endAskMu(r *approvalRequest, end, by string) bool {
	if a.asks[r.from] != r {
		return false
	}
	delete(a.asks, r.from)
	r.timer.Stop()
	a.deliverJSONMu(r.from, approvalEndMsg{T: "approvalEnded", Request: r.id, End: end, By: by})
	for q := range r.cards {
		a.deliverJSONMu(q, approvalEndMsg{T: "approvalCardClosed", Request: r.id, End: end, By: by})
	}
	return true
}

func (a *App) endAsk(r *approvalRequest, end, by string) {
	a.mu.Lock()
	ended := a.endAskMu(r, end, by)
	a.cardsSyncMu()
	a.mu.Unlock()
	if ended {
		a.logAskEnd(r, end, by)
	}
}

func (a *App) logAskEnd(r *approvalRequest, end, by string) {
	note := r.what
	if by != "" {
		note += ", by " + by
	}
	e := event(kindCommand, "approvalRequest", r.who, end)
	e.Note = note
	a.logEvent(e)
}

// Caller holds mu.
func (a *App) cardsSyncMu() {
	var gone []*approvalRequest
	for c, r := range a.asks {
		if _, ok := a.clients[c]; !ok || c.userID() != r.userID {
			gone = append(gone, r)
		}
	}
	for _, r := range gone {
		if a.endAskMu(r, "cancelled", "") {
			go a.logAskEnd(r, "cancelled", "")
		}
	}
	for _, r := range a.asks {
		for q := range r.cards {
			if _, ok := a.clients[q]; !ok {
				delete(r.cards, q)
			}
		}
		for q := range a.clients {
			who, had := r.cards[q]
			want := a.qualifiesMu(q, r)
			if had && (!want || who != q.userID()) {
				delete(r.cards, q)
				a.deliverJSONMu(q, approvalEndMsg{T: "approvalCardClosed", Request: r.id})
				had = false
			}
			if want && !had {
				r.cards[q] = q.userID()
				a.deliverJSONMu(q, approvalCardMsg{T: "approvalCard", Request: r.id, What: r.what, From: r.who, Browser: r.ip,
					Changes: r.changes, Interruptions: r.stops, Left: r.left()})
			}
		}
	}
	for c := range a.clients {
		if c.api != nil {
			continue
		}
		m := approversMsg{T: "approvers",
			Trusted:       a.anyApproverMu(&approvalRequest{from: c, need: config.RoleTrusted}),
			Administrator: a.anyApproverMu(&approvalRequest{from: c, need: config.RoleAdministrator})}
		key := strconv.FormatBool(m.Trusted) + strconv.FormatBool(m.Administrator)
		if key != c.u.approvers {
			c.u.approvers = key
			a.deliverJSONMu(c, m)
		}
	}
}

func (a *App) serveApprovalCards(c *client, m *inMsg) {
	a.mu.Lock()
	c.u.cards = m.On
	a.cardsSyncMu()
	a.mu.Unlock()
}

func (a *App) serveApprovalCancel(c *client) {
	a.mu.Lock()
	r := a.asks[c]
	ended := r != nil && a.endAskMu(r, "cancelled", "")
	a.cardsSyncMu()
	a.mu.Unlock()
	if ended {
		a.logAskEnd(r, "cancelled", "")
	}
}

var errAskEnded = errors.New("this request has ended")

// The approver is this browser's signed-in user, never a typed name. Deny needs no PIN.
func (a *App) serveApprovalAnswer(c *client, m *inMsg) {
	reply := func(err error) {
		res := opResultFor("approvalAnswer", nil, err)
		a.reply(c, struct {
			opResult
			Request string `json:"request"`
		}{res, m.Request})
	}
	find := func() *approvalRequest {
		for _, r := range a.asks {
			if who, ok := r.cards[c]; ok && r.id == m.Request && who == c.userID() && a.qualifiesMu(c, r) {
				return r
			}
		}
		return nil
	}
	a.mu.Lock()
	r := find()
	cfg := a.r().cfg
	var hit config.User
	if u := c.u.user.Load(); u != nil {
		hit, _ = cfg.FindUserByID(u.ID)
	}
	if r == nil || hit.ID == "" {
		a.mu.Unlock()
		reply(errAskEnded)
		return
	}
	if !m.Approve {
		ended := a.endAskMu(r, "denied", hit.Name)
		a.cardsSyncMu()
		a.mu.Unlock()
		if ended {
			a.logAskEnd(r, "denied", hit.Name)
		}
		reply(nil)
		return
	}
	a.mu.Unlock()
	if err := a.provePin(c, nil, &hit, m.Pin); err != nil {
		reply(err)
		return
	}
	a.mu.Lock()
	live, ok := a.livePinMu(&hit)
	if !ok || find() != r || !a.endAskMu(r, "approved", live.Name) {
		a.mu.Unlock()
		reply(errAskEnded)
		return
	}
	a.cardsSyncMu()
	a.mu.Unlock()
	reply(nil)
	a.logAskEnd(r, "approved", live.Name)
	a.runApproved(r, live)
}

// Skips admit on the requester's connection, which can be under the Lock; asRole stands in for it.
func (a *App) runApproved(r *approvalRequest, ap config.User) {
	rl, _ := ruleOf(r.msg.T)
	m := r.msg
	m.approver, m.asRole = &ap, r.role
	rl = rl.forMsg(&m)
	if !config.RoleAtLeast(approverRole(a.r().cfg, &ap), rl.role) {
		a.refuse(r.from, m.T, rl, &m, cannotApprove(&ap, rl.role))
		return
	}
	done, ok := a.writes.start(m.T)
	if !ok {
		a.refuse(r.from, m.T, rl, &m, errShuttingDown)
		return
	}
	m.done = done
	rl.handle(a, r.from, &m)
}

// Caller holds mu.
func (a *App) deliverJSONMu(c *client, v any) {
	b, _ := json.Marshal(v)
	a.deliver(c, b)
}

// ruleOf: policy names these handlers, so they reach the table through a func set in init.
var ruleOf func(t string) (rule, bool)

func init() {
	ruleOf = func(t string) (rule, bool) {
		r, ok := policy[t]
		return r, ok
	}
}
