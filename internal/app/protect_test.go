package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// fakeProtect models the Ultrix's Protects (docs/agents/router-verified.md). One message covers one level, and
// 0x8D or 0x8F answers it. Only the device that set a level clears it. A connect to a Protected level gets no answer.
type fakeProtect struct {
	holds     map[[2]int]protHold // dst, wire level
	deafSet   map[int]bool        // wire levels whose 0x8C gets no answer
	deafClear map[int]bool        // wire levels whose 0x8E gets no answer
}

// Caller holds f.mu.
func (f *fakeRouter) protectMsg(m []byte) {
	if f.prot.holds == nil {
		f.prot.holds = map[[2]int]protHold{}
	}
	mx, l, dst, dev := int(m[1]), int(m[2]), int(m[3])<<8|int(m[4]), int(m[5])<<8|int(m[6])
	k := [2]int{dst, l}
	h, held := f.prot.holds[k]
	if m[0] == swp08.CmdExtProtectConnect {
		if f.prot.deafSet[l] || (held && h.dev != dev) {
			return
		}
		f.prot.holds[k] = protHold{dev, swp08.KindProtect}
		f.out <- swp08.ExtProtectChange(swp08.CmdExtProtectConnected, mx, l, 1, dst, dev)
		return
	}
	if f.prot.deafClear[l] || !held || h.dev != dev {
		return
	}
	delete(f.prot.holds, k)
	f.out <- swp08.ExtProtectChange(swp08.CmdExtProtectDisconnected, mx, l, 0, dst, 0)
}

func (f *fakeRouter) holdsOn(dst int) map[int]protHold {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]protHold{}
	for k, h := range f.prot.holds {
		if k[0] == dst {
			out[k[1]] = h
		}
	}
	return out
}

// protectApp: userApp (Ana and Bo Standard, Ada Administrator), 3 levels, device 126, the router up. Ana is a
// Trusted user, so she has the Protect right by default.
func protectApp(t *testing.T) (*App, *fakeRouter) {
	t.Helper()
	a := userApp(t, false)
	f := newFakeRouter(t, a)
	shortTimers(a)
	f.prot = fakeProtect{holds: map[[2]int]protHold{}, deafSet: map[int]bool{}, deafClear: map[int]bool{}}
	cfg := a.r().cfg.Clone()
	cfg.Levels = 3
	cfg.ProtectDevice = 126
	cfg.DefaultLevels = "1-3"
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Ana" {
			cfg.Users[i].Role = config.RoleTrusted
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.status.Router = string(swp08.StateUp)
	for d := 0; d < 8; d++ {
		a.router.dstNames[d] = "MON " + string(rune('0'+d))
	}
	a.mu.Unlock()
	return a, f
}

func signedClient(t *testing.T, a *App, name, pin string) *client {
	t.Helper()
	c := &client{ip: "10.0.0." + name}
	if err := a.UserUnlockAs(c, name, pin); err != nil {
		t.Fatal(err)
	}
	return c
}

func protEntry(t *testing.T, o protectOutcome) *Entry {
	t.Helper()
	if o.Err != nil || o.Entry == nil {
		t.Fatalf("refused: %v", o.Err)
	}
	return o.Entry
}

func protRefused(t *testing.T, o protectOutcome, code, text string) protectErr {
	t.Helper()
	var pe protectErr
	if o.Entry != nil || !errorsAs(o.Err, &pe) || pe.code != code || !strings.Contains(pe.msg, text) {
		t.Fatalf("got %+v %v, want %s %q", o.Entry, o.Err, code, text)
	}
	return pe
}

func errorsAs(err error, pe *protectErr) bool {
	p, ok := err.(protectErr)
	if ok {
		*pe = p
	}
	return ok
}

func readProtects(t *testing.T, a *App) []protectRec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.Dir(a.cfgPath), "protects.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f protectFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f.Protects
}

func TestProtectSetsEveryLevelAndRefusesTakes(t *testing.T) {
	a, f := protectApp(t)
	ana := signedClient(t, a, "Ana", "1234")
	a.mu.Lock()
	a.router.tally[4] = map[int]int{1: 7, 2: 7, 3: -1}
	a.mu.Unlock()
	e := protEntry(t, a.protectSet(ana, 4, nil))
	if !e.OK || e.Protect != "set" || !slices.Equal(e.Levels, []int{1, 2, 3}) || e.Src != -1 || e.Client != "Ana" {
		t.Fatalf("entry %+v", e)
	}
	// Prev holds the routes on the destination when it ran, for History's Source detail.
	if !slices.Equal(e.Prev, [][2]int{{1, 7}, {2, 7}, {3, -1}}) {
		t.Fatalf("prev %v", e.Prev)
	}
	want := map[int]protHold{0: {126, 1}, 1: {126, 1}, 2: {126, 1}}
	if got := f.holdsOn(4); len(got) != 3 || got[0] != want[0] || got[2] != want[2] {
		t.Fatalf("router holds %v", got)
	}
	recs := readProtects(t, a)
	if len(recs) != 1 || recs[0].Dst != 4 || recs[0].By != "Ana" || recs[0].User != idOf(a, "Ana") || !slices.Equal(recs[0].Levels, []int{1, 2, 3}) {
		t.Fatalf("record %+v", recs)
	}
	// Sode refuses its own Protected destination before it sends anything; it never clears, takes and sets again.
	n := f.sends()
	te := routeOne(t, a, 7, 4, 1, 2, 3)
	if te.OK || !strings.Contains(te.Err, "protected") || f.sends() != n {
		t.Fatalf("take %+v, sends %d to %d", te, n, f.sends())
	}
	protRefused(t, a.protectSet(ana, 4, nil), "conflict", "already")
}

func TestProtectRollsBackAFailedLevel(t *testing.T) {
	a, f := protectApp(t)
	f.prot.deafSet[1] = true
	e := protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
	if e.OK || !slices.Equal(e.Failed, []int{2}) || len(e.Still) != 0 || !strings.Contains(e.Err, "freed the other levels") {
		t.Fatalf("entry %+v", e)
	}
	if got := f.holdsOn(4); len(got) != 0 {
		t.Fatalf("rollback left %v", got)
	}
	if _, ok := a.protects.get(a.r().cfg.UID, 4); ok {
		t.Fatal("a rolled back Protect stays in the record")
	}
}

func TestProtectFailedRollbackKeepsWhatIsStillProtected(t *testing.T) {
	a, f := protectApp(t)
	f.prot.deafSet[1], f.prot.deafClear[2] = true, true
	e := protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
	if e.OK || !slices.Equal(e.Still, []int{3}) || !strings.Contains(e.Err, "levels 3 are still Protected") {
		t.Fatalf("entry %+v", e)
	}
	rec, ok := a.protects.get(a.r().cfg.UID, 4)
	if !ok || !slices.Equal(rec.Levels, []int{3}) {
		t.Fatalf("record %+v %v", rec, ok)
	}
	// The record lets Ana clear it later.
	f.prot.deafClear[2] = false
	if c := protEntry(t, a.protectClear(signedClient(t, a, "Ana", "1234"), 4, nil, nil)); !c.OK || len(f.holdsOn(4)) != 0 {
		t.Fatalf("clear %+v, holds %v", c, f.holdsOn(4))
	}
}

func TestProtectRefusals(t *testing.T) {
	a, f := protectApp(t)
	bo := signedClient(t, a, "Bo", "567890")
	protRefused(t, a.protectSet(bo, 4, nil), "outside_rights", "Protect is outside your Rights")

	cfg := a.r().cfg.Clone()
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Bo" {
			cfg.Users[i].Rights = config.Rights{Protect: ptr(true), Dsts: nums(5)}
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	protRefused(t, a.protectSet(bo, 4, nil), "outside_rights", "destination 5 is outside your rights")

	ana := signedClient(t, a, "Ana", "1234")
	a.onRouterMsg(swp08.ExtProtectChange(swp08.CmdExtProtectTally, 0, 1, 2, 6, 9))
	protRefused(t, a.protectSet(ana, 6, nil), "protected", "Router lock by another system (device 9) on levels 2")
	a.onRouterMsg(swp08.ExtProtectChange(swp08.CmdExtProtectTally, 0, 0, 1, 7, 112))
	protRefused(t, a.protectSet(ana, 7, nil), "protected", "Protected by another system (device 112) on levels 1")
	if n := f.sends(); n != 0 {
		t.Fatalf("%d messages sent for refusals", n)
	}

	cfg = a.r().cfg.Clone()
	cfg.ProtectDevice = 0
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.status.Router = string(swp08.StateUp)
	a.mu.Unlock()
	protRefused(t, a.protectSet(ana, 4, nil), "conflict", "device number")
}

func TestClearAnotherUsersProtectNeedsAnAdministrator(t *testing.T) {
	a, f := protectApp(t)
	protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
	setRole(t, a, "Bo", config.RoleTrusted)
	bo := signedClient(t, a, "Bo", "567890")
	if pe := protRefused(t, a.protectClear(bo, 4, nil, nil), "administrator_only", "Ana set this Protect"); !pe.admin {
		t.Fatal("the refusal must ask for an Administrator")
	}
	// An Approval from an Administrator stands in.
	ada, _ := a.r().cfg.FindUserByName("Ada")
	e := protEntry(t, a.protectClear(bo, 4, nil, &ada))
	if !e.OK || e.Protect != "clear" || e.Client != "Bo" || len(f.holdsOn(4)) != 0 {
		t.Fatalf("entry %+v", e)
	}
	// An Administrator clears it as themself.
	protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 5, nil))
	if e := protEntry(t, a.protectClear(adminClient(t, a), 5, nil, nil)); !e.OK {
		t.Fatalf("entry %+v", e)
	}
	// No button on a Protect Sode did not set: the server refuses a clear of one.
	a.onRouterMsg(swp08.ExtProtectChange(swp08.CmdExtProtectTally, 0, 0, 1, 6, 112))
	protRefused(t, a.protectClear(adminClient(t, a), 6, nil, nil), "not_found", "no Protect")
}

func TestProtectUndoFromHistory(t *testing.T) {
	a, f := protectApp(t)
	ana := signedClient(t, a, "Ana", "1234")
	set := protEntry(t, a.protectSet(ana, 4, nil))
	protRefused(t, a.protectSet(ana, 4, &set.ID), "bad_request", "not a Protect to undo")
	undo := protEntry(t, a.protectClear(ana, 4, &set.ID, nil))
	if !undo.OK || undo.Undo == nil || *undo.Undo != set.ID || len(f.holdsOn(4)) != 0 {
		t.Fatalf("undo %+v", undo)
	}
	again := protEntry(t, a.protectSet(ana, 4, &undo.ID))
	if !again.OK || len(f.holdsOn(4)) != 3 {
		t.Fatalf("undo of the clear %+v", again)
	}
	f.mu.Lock()
	whys := slices.Clone(f.whys)
	f.mu.Unlock()
	if w := whys[len(whys)-1].why.Reason; w != "protectUndo" {
		t.Fatalf("an Undo's messages are logged as %q", w)
	}
}

// Free Sode's Protects frees every Protect with Sode's device number, record or not, and asks the PIN again.
func TestClearSodeProtectsNeedsRecheck(t *testing.T) {
	a, f := protectApp(t)
	protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
	// A Protect with Sode's number and no record, and one by a panel.
	for _, l := range []int{0, 1, 2} {
		f.protectMsgLocked(swp08.ExtProtectConnect(0, l, 5, 126))
		f.protectMsgLocked(swp08.ExtProtectConnect(0, l, 6, 112))
	}
	waitFor(t, "the Protects", func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return len(a.router.protect[5]) == 3 && len(a.router.protect[6]) == 3
	})
	if err := a.protects.put(protectRec{Router: a.r().cfg.UID, Dst: 4}); err != nil {
		t.Fatal(err)
	}

	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	if r := p.ask("protectClearAll", 7, nil); r["ok"] == true || r["recheck"] != true {
		t.Fatalf("without a recheck: %v", r)
	}
	if r := p.recheck("2468"); r["ok"] != true {
		t.Fatalf("recheck %v", r)
	}
	if r := p.ask("protectClearAll", 8, nil); r["ok"] != true || r["cleared"] != float64(2) {
		t.Fatalf("clear all: %v", r)
	}
	if len(f.holdsOn(4)) != 0 || len(f.holdsOn(5)) != 0 || len(f.holdsOn(6)) != 3 {
		t.Fatalf("holds 4 %v, 5 %v, 6 %v", f.holdsOn(4), f.holdsOn(5), f.holdsOn(6))
	}
}

func (f *fakeRouter) protectMsgLocked(m []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.protectMsg(m)
}

func TestAPIProtectNeedsScopeAndRight(t *testing.T) {
	a, f := protectApp(t)
	cfg := a.r().cfg.Clone()
	cfg.APIClients = []config.APIClient{
		{Name: "TakeBot", Scopes: []string{config.ScopeTake}, Rights: config.Rights{Protect: ptr(true)}},
		{Name: "NoRight", Scopes: []string{config.ScopeProtect}},
		{Name: "Holder", Scopes: []string{config.ScopeProtect, config.ScopeProtected}, Rights: config.Rights{Protect: ptr(true)}},
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.status.Router = string(swp08.StateUp)
	a.mu.Unlock()
	key := func(name string) string {
		return apiTestKey(t, a, keyOwner{Kind: actorClient, Name: name}, "k", time.Time{})
	}
	if rec := apiSend(a, "POST", "/api/v1/protected/4", key("TakeBot"), ""); rec.Code != 403 || apiErrCode(t, rec) != "scope" {
		t.Fatalf("take scope only: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "POST", "/api/v1/protected/4", key("NoRight"), ""); rec.Code != 403 || apiErrCode(t, rec) != "outside_rights" {
		t.Fatalf("no right: %d %s", rec.Code, rec.Body.String())
	}
	holder := key("Holder")
	rec := apiSend(a, "POST", "/api/v1/protected/4", holder, "")
	if body := apiBody(t, rec); rec.Code != 200 || body["ok"] != true || len(f.holdsOn(4)) != 3 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	rows := apiBody(t, apiDo(a, "GET", "/api/v1/protected", holder, nil))["protected"].([]any)
	var sode int
	for _, r := range rows {
		m := r.(map[string]any)
		if m["dst"] == float64(4) {
			if m["kind"] != "protect" || m["device"] != float64(126) || m["sodeUser"] != "Holder (k)" && m["sodeUser"] != "Holder" {
				t.Fatalf("row %v", m)
			}
			sode++
		}
	}
	if sode != 3 {
		t.Fatalf("rows %v", rows)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/protected/4", holder, ""); rec.Code != 200 || len(f.holdsOn(4)) != 0 {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "DELETE", "/api/v1/protected/4", holder, ""); rec.Code != 404 {
		t.Fatalf("clear of nothing: %d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 2 || es[0].Protect != "clear" || es[1].Protect != "set" || es[1].Actor == nil {
		t.Fatalf("History %+v", es)
	}
	if rec := apiPost(a, "/api/v1/history/"+strconv.Itoa(es[1].ID)+"/undo", holder, "", nil); rec.Code != 403 && rec.Code != 409 {
		t.Fatalf("API undo of a Protect: %d %s", rec.Code, rec.Body.String())
	}
}

// A Protect's kind and a Router lock reach the page; the record names Sode's own.
func TestProtectMessageCarriesKindAndSodeUser(t *testing.T) {
	a, _ := protectApp(t)
	c := &client{ip: "10.0.0.50"}
	next := registerAll(a, c)
	protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
	a.onRouterMsg(swp08.ExtProtectChange(swp08.CmdExtProtectTally, 0, 0, 2, 6, 9))
	a.flush()
	var by []any
	var lock bool
	for _, m := range next() {
		if m["t"] != "protected" {
			continue
		}
		if b, _ := m["by"].([]any); len(b) > 0 {
			by = b
		}
		for _, u := range m["u"].([]any) {
			row := u.([]any)
			if row[0] == float64(6) && row[2] == float64(9) && row[3] == float64(swp08.KindRouterLock) {
				lock = true
			}
		}
	}
	if len(by) != 1 || by[0].(map[string]any)["by"] != "Ana" || by[0].(map[string]any)["kind"] != byUser || by[0].(map[string]any)["dst"] != float64(4) || !lock {
		t.Fatalf("by %v, Router lock seen %v", by, lock)
	}
}

// The page tells "Protected by you" from another Sode user by the name and the kind. The message reaches locked pages,
// so it carries no user id. A user who is gone, a guest and an API client each have their own kind.
func TestProtectByKindTellsOwnersApart(t *testing.T) {
	a, _ := protectApp(t)
	r := a.r()
	ana, _ := r.cfg.FindUserByName("Ana")
	recs := []protectRec{
		{Router: r.cfg.UID, Dst: 1, Device: 126, Levels: []int{1}, By: "Ana old", User: ana.ID},
		{Router: r.cfg.UID, Dst: 2, Device: 126, Levels: []int{1}, By: "Desk", Client: "Desk"},
		{Router: r.cfg.UID, Dst: 3, Device: 126, Levels: []int{1}, By: "10.0.0.9"},
		{Router: r.cfg.UID, Dst: 5, Device: 126, Levels: []int{1}, By: "Cy", User: "u-gone"},
	}
	for _, rec := range recs {
		if err := a.protects.put(rec); err != nil {
			t.Fatal(err)
		}
	}
	got := map[int][2]string{}
	for _, b := range a.protectByList(r) {
		got[b.Dst] = [2]string{b.By, b.Kind}
	}
	want := map[int][2]string{1: {"Ana", byUser}, 2: {"Desk", byClient}, 3: {"10.0.0.9", byGuest}, 5: {"Cy", byGone}}
	for d, w := range want {
		if got[d] != w {
			t.Fatalf("dst %d: got %v, want %v", d, got[d], w)
		}
	}
}

// Request approval covers a clear of another user's Protect: the frozen message runs with the approver.
func TestRequestApprovalClearsAnotherUsersProtect(t *testing.T) {
	a, f := protectApp(t)
	a.approvalWait = time.Minute
	protEntry(t, a.protectSet(signedClient(t, a, "Ana", "1234"), 4, nil))
	setRole(t, a, "Bo", config.RoleTrusted)
	a.mu.Lock()
	a.status.Router = string(swp08.StateUp)
	a.mu.Unlock()
	ada := approver(t, a, "Ada", "2468")
	bo := dialPeer(t, a)
	bo.signIn("Bo", "567890")
	if r := bo.ask("protectClear", 3, map[string]any{"protect": map[string]any{"dst": 4}}); r["ok"] == true || r["admin"] != true {
		t.Fatalf("Bo's clear: %v", r)
	}
	asked(t, bo.ask("approvalAsk", 50, map[string]any{"for": map[string]any{"t": "protectClear", "id": 4, "protect": map[string]any{"dst": 4}}}))
	card := ada.until("approvalCard")
	if card["what"] != "Freeing another user's Protect" || card["from"] != "Bo" {
		t.Fatalf("card %v", card)
	}
	if r := ada.answer(card["request"], true, "2468"); r["ok"] != true {
		t.Fatalf("approve: %v", r)
	}
	if r := bo.until("protectClearResult"); r["ok"] != true || r["id"] != float64(4) {
		t.Fatalf("result %v", r)
	}
	if got := f.holdsOn(4); len(got) != 0 {
		t.Fatalf("holds %v", got)
	}
}
