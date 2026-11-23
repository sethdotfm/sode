package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/sethdotfm/sode/internal/config"
)

// The page messages of Protects and their replies (wire fields in docs/agents/server.md); protect.go runs them.

// protectReq is the page's Protect or Free Protect: one action on the destinations of dsts, with one History Group.
// A lone dst (with undo) is the same as dsts of one. Wire fields: docs/agents/server.md.
type protectReq struct {
	Dst  *int         `json:"dst"`
	Undo *int         `json:"undo"`
	Dsts []protectDst `json:"dsts"`
}

// protectDst is one destination of a protectReq. Undo names the History entry it undoes.
type protectDst struct {
	Dst  int  `json:"dst"`
	Undo *int `json:"undo"`
	// Router: the UID of the router of Dst, from the hello's routers list; blank is the first router.
	Router string `json:"router,omitempty"`
}

// reqs: the destinations to act on, in the page's order. Two rows for one destination would both pass the checks
// before anything is sent, so a duplicate refuses the message.
func (p *protectReq) reqs() ([]protectDst, error) {
	switch {
	case p == nil || (p.Dst == nil && p.Dsts == nil):
		return nil, refusal(400, "bad_request", "nothing to Protect")
	case p.Dst != nil && p.Dsts != nil, p.Undo != nil && p.Dsts != nil:
		return nil, refusal(400, "bad_request", "send dst or dsts, not both")
	case p.Dsts == nil:
		return []protectDst{{Dst: *p.Dst, Undo: p.Undo}}, nil
	case len(p.Dsts) == 0:
		return nil, refusal(400, "bad_request", "nothing to Protect")
	case len(p.Dsts) > protectManyMax:
		return nil, refusal(400, "bad_request", "more than %d destinations", protectManyMax)
	}
	type key struct {
		router string
		dst    int
	}
	seen := make(map[key]bool, len(p.Dsts))
	for _, q := range p.Dsts {
		if seen[key{q.Router, q.Dst}] {
			return nil, refusal(400, "bad_request", "destination %d is in the list two times", q.Dst)
		}
		seen[key{q.Router, q.Dst}] = true
	}
	return p.Dsts, nil
}

// protectManyMax: more destinations than any router has, so a bad page cannot hold protMu for long.
const protectManyMax = 4096

// protectResultMsg answers protectSet and protectClear. OK only when every row is. Err is a refusal of the whole
// message, which then has no rows.
type protectResultMsg struct {
	T       string            `json:"t"` // protectSetResult or protectClearResult
	ID      int               `json:"id"`
	OK      bool              `json:"ok"`
	Err     string            `json:"err,omitempty"`
	Results []protectDstReply `json:"results"`
}

// protectDstReply: one destination of a protectResultMsg, in the order of the request.
type protectDstReply struct {
	Dst   int    `json:"dst"`
	OK    bool   `json:"ok"`
	Err   string `json:"err,omitempty"`
	Entry *int   `json:"entry,omitempty"`
	// Still: the levels Sode still holds after a failed Protect or Free Protect.
	Still []int `json:"still,omitempty"`
	// Admin: the Protect is another Sode user's; the page asks an Approval. Recheck and Approval as in opResult.
	Admin    bool `json:"admin,omitempty"`
	Recheck  bool `json:"recheck,omitempty"`
	Approval bool `json:"approval,omitempty"`
}

type protectClearAllResultMsg struct {
	T        string `json:"t"` // protectClearAllResult
	ID       int    `json:"id"`
	OK       bool   `json:"ok"`
	Err      string `json:"err,omitempty"`
	Cleared  int    `json:"cleared,omitempty"` // the destinations it freed
	Recheck  bool   `json:"recheck,omitempty"`
	Approval bool   `json:"approval,omitempty"`
}

func protectReplyOf(dst int, o protectOutcome) protectDstReply {
	m := protectDstReply{Dst: dst, OK: o.Err == nil}
	if o.Entry != nil {
		e := o.Entry.ID
		m.Entry, m.Still = &e, o.Entry.Still
		if !o.Entry.OK {
			m.OK, m.Err = false, o.Entry.Err
		}
	}
	if o.Err != nil {
		m.Err = o.Err.Error()
		var pe protectErr
		m.Admin = errors.As(o.Err, &pe) && pe.admin
		m.Recheck, m.Approval = isRecheckErr(o.Err), isApprovalErr(o.Err)
	}
	return m
}

func protectResult(kind string, id int, reqs []protectDst, outs []protectOutcome) protectResultMsg {
	res := protectResultMsg{T: kind + "Result", ID: id, OK: true, Results: make([]protectDstReply, len(outs))}
	for i, o := range outs {
		res.Results[i] = protectReplyOf(reqs[i].Dst, o)
		res.OK = res.OK && res.Results[i].OK
	}
	return res
}

// protectBy: Sode's own Protects for the page, dst → who set it. It reaches locked pages, as History does, so it
// carries names and never a user's id. Kind lets the page tell its own signed-in user from another user of the same
// name, such as an API client.
type protectBy struct {
	Dst    int    `json:"dst"`
	By     string `json:"by"`
	Kind   string `json:"kind"` // byUser, byGone, byClient or byGuest; keep in step with web/protectrules.js
	Device int    `json:"device"`
	Levels []int  `json:"levels"`
}

const (
	byUser   = "user"   // a user who still exists, also through their own API client
	byGone   = "gone"   // a user who is gone: By is the name when set
	byClient = "client" // an API client with no owner
	byGuest  = "guest"  // a guest: By is where they were (an IP address)
)

type protectMsg struct {
	T  string       `json:"t"`  // "protected"
	U  []protChange `json:"u"`  // [dst, level, device, kind]; device -1 when freed
	By []protectBy  `json:"by"` // every Protect in Sode's record of this router
	// RouterUID names a router after the first.
	RouterUID string `json:"routerUid,omitempty"`
}

type protChange struct {
	Dst, Level, Dev, Kind int
}

func (c protChange) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, "[%d,%d,%d,%d]", c.Dst, c.Level, c.Dev, c.Kind), nil
}

// [[dst, level, device, kind], ...] for the hello.
func protectRows(m map[int]map[int]protHold) json.RawMessage {
	rows := make([]protChange, 0, len(m))
	for d, t := range m {
		for l, h := range t {
			rows = append(rows, protChange{d, l, h.dev, h.kind})
		}
	}
	b, _ := json.Marshal(rows)
	return b
}

// protectByList: the live name of a user who set one, else the name when set.
func (a *App) protectByList(r *runtime) []protectBy { return a.protectByListOf(r, r.cfg.UID) }

// protectByListOf is protectByList for the router with this settings UID.
func (a *App) protectByListOf(r *runtime, uid string) []protectBy {
	out := []protectBy{}
	for _, rec := range a.protects.list(uid) {
		by, kind := recWho(r.cfg, rec)
		out = append(out, protectBy{Dst: rec.Dst, By: by, Kind: kind, Device: rec.Device, Levels: rec.Levels})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dst < out[j].Dst })
	return out
}

// recWho: who set the Protect of rec, by the live name of a user, and the kind.
func recWho(cfg config.Config, rec protectRec) (by, kind string) {
	switch u, ok := cfg.FindUserByID(rec.User); {
	case ok && rec.Client == "":
		return u.Name, byUser
	case rec.Client != "":
		return rec.By, byClient
	case rec.User != "":
		return rec.By, byGone
	}
	return rec.By, byGuest
}

// recName is the name of recWho.
func recName(cfg config.Config, rec protectRec) string {
	by, _ := recWho(cfg, rec)
	return by
}

// It sends the router's pending hold changes too, so the page has the state with the entry and History's Undo
// follows at once. Caller holds mu.
// uid names the router (blank: the first).
func (a *App) broadcastProtectsMu(uid string) {
	u := a.unitOrFirst(uid)
	ch := u.router.pendProt
	if ch == nil {
		ch = []protChange{}
	}
	u.router.pendProt = nil
	a.broadcast(a.protectMsgOf(u, ch))
}

// protectMsgOf is the protected message of router u with the hold changes ch. Caller holds mu.
func (a *App) protectMsgOf(u *routerUnit, ch []protChange) protectMsg {
	r := a.r()
	if u == a.firstUnit() {
		return protectMsg{T: "protected", U: ch, By: a.protectByList(r)}
	}
	return protectMsg{T: "protected", U: ch, By: a.protectByListOf(r, r.routerOf(u.uid()).UID), RouterUID: u.uid()}
}

func (a *App) serveProtect(c *client, m *inMsg, set bool) {
	go func() {
		defer m.finish()
		reqs, err := m.Protect.reqs()
		if err != nil {
			a.reply(c, protectResultMsg{T: m.T + "Result", ID: m.ID, Err: err.Error(), Results: []protectDstReply{}})
			return
		}
		why := "protect"
		if !set {
			why = "protectClear"
		}
		outs := a.protectMany(c, reqs, set, m.approver)
		for i, o := range outs {
			if o.Err != nil {
				a.command(c, why, outcome(o.Err), approvedNote(m.approver, "destination "+a.dstLabelsOn(reqs[i].Router, reqs[i].Dst)[0]), o.Err)
			}
		}
		a.reply(c, protectResult(m.T, m.ID, reqs, outs))
	}()
}

// refuseProtect: a policy refusal (a lock, the Role, an Approval) answers every destination with it.
func refuseProtect(a *App, c *client, m *inMsg, err error) {
	if m.T == "protectClearAll" {
		a.reply(c, protectClearAllResultMsg{T: m.T + "Result", ID: m.ID, Err: err.Error(), Recheck: isRecheckErr(err), Approval: isApprovalErr(err)})
		return
	}
	reqs, bad := m.Protect.reqs()
	if bad != nil {
		a.reply(c, protectResultMsg{T: m.T + "Result", ID: m.ID, Err: bad.Error(), Results: []protectDstReply{}})
		return
	}
	outs := make([]protectOutcome, len(reqs))
	for i := range outs {
		outs[i].Err = err
	}
	a.reply(c, protectResult(m.T, m.ID, reqs, outs))
}

func (a *App) serveProtectClearAll(c *client, m *inMsg) {
	go func() {
		defer m.finish()
		n, still, err := a.clearSodeProtects(c)
		res := protectClearAllResultMsg{T: "protectClearAllResult", ID: m.ID, OK: err == nil && len(still) == 0, Cleared: n}
		note := fmt.Sprintf("%d freed", n)
		if err == nil && len(still) > 0 {
			err = fmt.Errorf("%d %s still Protected", len(still), plural(len(still), "destination is", "destinations are"))
		}
		if err != nil {
			res.Err = err.Error()
		}
		a.command(c, "protectClearAll", outcome(err), approvedNote(m.approver, note), err)
		a.reply(c, res)
	}()
}
