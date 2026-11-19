package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Step in records (GLOSSARY.md): History, the Communication Log, the Settings saves record,
// Request approval and Interruptions. Seam: the App driven like the read loop, and over the
// WebSocket for Approval cards.

// takeAs makes one take on c through a fake router and returns its History entry.
func takeAs(t *testing.T, a *App, c *client) Entry {
	t.Helper()
	a.route(c, routeReq{ID: 1, Src: 7, Dst: 5, Levels: []int{1}})
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Ext {
		t.Fatalf("no take recorded: %+v", es)
	}
	return es[0]
}

// History names the stepped-in user as the actor and keeps the original apart, so nothing that
// matches the actor matches the original.
func TestStepInHistoryNamesBrowserOf(t *testing.T) {
	a, c, read, _ := stepApp(t)
	shortTimers(a)
	newFakeRouter(t, a)
	mustStepIn(t, a, c, read, "Bo", "567890")
	e := takeAs(t, a, c)
	if e.Client != "Bo" || e.BrowserOf != "Ana" || e.Actor != nil {
		t.Fatalf("entry client %q browserOf %q actor %+v", e.Client, e.BrowserOf, e.Actor)
	}
	if strings.Contains(e.Client, "Ana") {
		t.Fatalf("the actor names the original: %q", e.Client)
	}
	if line := e.logLine(1); !strings.Contains(line, "Bo (browser of Ana)") {
		t.Fatalf("log line %q", line)
	}

	a.StepOut(c)
	if e := takeAs(t, a, c); e.Client != "Ana" || e.BrowserOf != "" {
		t.Fatalf("after Step out: client %q browserOf %q", e.Client, e.BrowserOf)
	}
}

// With nobody signed in, the browser is its address.
func TestStepInHistoryBrowserOfAddress(t *testing.T) {
	a := userApp(t, false)
	shortTimers(a)
	newFakeRouter(t, a)
	c := &client{ip: "10.0.0.9"}
	read := peerOf(a, c)
	a.Hi(c, "main", "")
	mustStepIn(t, a, c, read, "Bo", "567890")
	if e := takeAs(t, a, c); e.Client != "Bo" || e.BrowserOf != "10.0.0.9" {
		t.Fatalf("client %q browserOf %q", e.Client, e.BrowserOf)
	}
}

// stepRows waits for n stepIn and stepOut rows in the Communication Log.
func stepRows(t *testing.T, a *App, n int) []Sent {
	t.Helper()
	var out []Sent
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		out = out[:0]
		for _, e := range logged(t, a) {
			if e.Kind == kindCommand && (e.Why == "stepIn" || e.Why == "stepOut") {
				out = append(out, e)
			}
		}
		if len(out) >= n {
			return out
		}
	}
	t.Fatalf("got %d step rows, want %d: %+v", len(out), n, out)
	return nil
}

// The Log has a row for each failed try, Step in and Step out, with how the step-in ended.
func TestStepInLogRows(t *testing.T) {
	a, c, read, _ := stepApp(t)
	defer startLog(t, a)()
	if r := askStepIn(t, a, c, read, "Bo", "000000"); r["ok"] == true {
		t.Fatal("a wrong PIN stepped in")
	}
	mustStepIn(t, a, c, read, "Bo", "567890")
	a.StepOut(c)
	mustStepIn(t, a, c, read, "Bo", "567890")
	a.UserLock(c)
	rows := stepRows(t, a, 5)
	type row struct{ why, by, result, note string }
	want := []row{
		{"stepIn", "Ana", "refused", "as Bo"},
		{"stepIn", "Bo", "ok", "browser of Ana"},
		{"stepOut", "Bo", "ok", "Step out, browser of Ana"},
		{"stepIn", "Bo", "ok", "browser of Ana"},
		{"stepOut", "Bo", "ok", "a Lock, browser of Ana"},
	}
	for i, w := range want {
		e := rows[i]
		if (row{e.Why, e.By, e.Result, e.Note}) != w {
			t.Fatalf("row %d = %s %s %s %q, want %+v", i, e.Why, e.By, e.Result, e.Note, w)
		}
	}
	if rows[0].Err == "" {
		t.Fatal("the failed try has no reason")
	}
}

// saveLive sends the one Save with the live versions, user fields included.
func saveLive(p *wsPeer, a *App, id int, set map[string]any, extra map[string]any) map[string]any {
	p.t.Helper()
	live := a.r().cfg
	fields := map[string]any{}
	for k, v := range set {
		fields[k] = map[string]any{"value": v, "version": fieldVersion(live, k)}
	}
	m := map[string]any{"fields": fields}
	for k, v := range extra {
		m[k] = v
	}
	return p.ask("settingsSave", id, m)
}

func (p *wsPeer) stepIn(name, pin string) {
	p.t.Helper()
	if r := p.ask("stepIn", 3, map[string]any{"user": name, "pin": pin}); r["ok"] != true {
		p.t.Fatalf("step in as %s: %v", name, r)
	}
}

// The stepped-in user is who saved, also for a save that ends their own step-in.
func TestStepInSavesRowNamesTheSteppedInUser(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	p.stepIn("Ada", "2468") // the step-in PIN counts as the PIN recheck
	wantSaved(t, saveLive(p, a, 1, map[string]any{"brandText": "Studio B"}, nil), "brandText")
	if e := waitSaves(t, a, 1)[0]; e.By != "Ada" {
		t.Fatalf("who saved = %q, want Ada", e.By)
	}

	ada := idOf(a, "Ada")
	wantSaved(t, saveLive(p, a, 2, map[string]any{"user/" + ada + "/name": "Adele"}, nil), "user/"+ada+"/name")
	if e := waitSaves(t, a, 2)[0]; e.By != "Ada" {
		t.Fatalf("who saved the rename = %q, want Ada", e.By)
	}
	rows := stepRows(t, a, 2)
	if e := rows[1]; e.Why != "stepOut" || !strings.HasPrefix(e.Note, "a save changed the user") {
		t.Fatalf("Step out row %+v", e)
	}
}

// An in-person Approval during a step-in names the stepped-in user and the approver, and the
// browser stays stepped in.
func TestStepInApprovalInPersonReturnsToTheStepIn(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	p.stepIn("Bo", "567890")
	r := saveLive(p, a, 1, map[string]any{"confirmLockoutMs": 900}, map[string]any{"approval": map[string]any{"user": "Ada", "pin": "2468"}})
	wantSaved(t, r, "confirmLockoutMs")
	e := waitSaves(t, a, 1)[0]
	if e.By != "Bo" || e.Save.Approver != "Ada" {
		t.Fatalf("row by %q approver %q", e.By, e.Save.Approver)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for c := range a.clients {
		if c.who() != "Bo" || c.u.step == nil {
			t.Fatalf("after the Approval: %q, stepped in %v", c.who(), c.u.step != nil)
		}
	}
}

// Cards reach a stepped-in browser by its stepped-in Role and close on Step out.
func TestStepInApprovalCardsFollowTheSteppedInRole(t *testing.T) {
	a := askApp(t, time.Minute)
	approver(t, a, "Ada", "2468") // someone must qualify when the request is sent
	desk := dialPeer(t, a)
	desk.signIn("Ana", "1234")
	desk.cardsOn()
	bo := dialPeer(t, a)
	bo.signIn("Bo", "567890")
	asked(t, bo.askSave(7, map[string]any{"router": "127.0.0.1:9"}))
	if m := desk.within("approvalCard", 50*time.Millisecond); m != nil {
		t.Fatalf("a Standard user got a card: %v", m)
	}
	desk.send(map[string]any{"t": "stepIn", "user": "Ada", "pin": "2468"}) // the card comes before the result
	card := desk.until("approvalCard")
	desk.send(map[string]any{"t": "stepOut"})
	if m := desk.until("approvalCardClosed"); m["request"] != card["request"] {
		t.Fatalf("closed %v, want %v", m, card["request"])
	}
	if m := bo.within("approvalEnded", 50*time.Millisecond); m != nil {
		t.Fatalf("Step out of an approver ended the request: %v", m)
	}
}

// A page close while stepped in writes the Step out row with how it ended.
func TestStepInPageCloseLogsStepOut(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	p.stepIn("Bo", "567890")
	p.ws.CloseNow()
	rows := stepRows(t, a, 2)
	if e := rows[1]; e.By != "Bo" || e.Note != "the page closed or the connection dropped, browser of Ana" {
		t.Fatalf("Step out row %+v", e)
	}
}

// A save that renames, re-PINs, removes or lowers a stepped-in user names their step-ins in the
// Interruptions and ends them. Other changes to that user end nothing.
func TestStepInInterruptionsNameTheStepInsASaveEnds(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(bo string) map[string]any
		ends bool
	}{
		{"rename", func(bo string) map[string]any { return map[string]any{"user/" + bo + "/name": "Bob"} }, true},
		{"PIN", func(bo string) map[string]any { return map[string]any{"user/" + bo + "/pin": "135790"} }, true},
		{"remove", func(bo string) map[string]any { return map[string]any{"user/" + bo + "/remove": true} }, true},
		{"lower", func(bo string) map[string]any { return map[string]any{"user/" + bo + "/role": config.RoleStandard} }, true},
		{"persistent", func(bo string) map[string]any { return map[string]any{"user/" + bo + "/persistent": true} }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := userApp(t, false)
			setFlags(t, a, "Bo", true, false)
			admin := dialPeer(t, a)
			admin.signInRechecked("Ada", "2468")
			desk := dialPeer(t, a)
			desk.signIn("Ana", "1234")
			desk.stepIn("Bo", "567890")
			set := tc.set(idOf(a, "Bo"))
			r := saveLive(admin, a, 1, set, nil)
			for k := range set {
				wantSaved(t, r, k)
			}
			l := interruptionsOf(t, r)
			line := ""
			for _, x := range l {
				if strings.HasPrefix(x["text"].(string), "Step-ins of Bo") {
					line = x["text"].(string)
					if count([]map[string]any{x}, "stepIns") != 1 || x["ends"] != true {
						t.Fatalf("line %v", x)
					}
				}
			}
			if !tc.ends {
				if line != "" {
					t.Fatalf("a save that ends no step-in: %s", texts(l))
				}
				return
			}
			if line != "Step-ins of Bo: this save ends 1 step-in." {
				t.Fatalf("Interruptions: %s", texts(l))
			}
			if m := desk.until("stepEnded"); m["why"] != "save" {
				t.Fatalf("stepEnded %v", m)
			}
		})
	}
}
