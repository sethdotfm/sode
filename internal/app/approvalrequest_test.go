package app

import (
	"strings"
	"testing"
	"time"
)

func askApp(t *testing.T, wait time.Duration) *App {
	t.Helper()
	a := userApp(t, false)
	a.approvalWait = wait
	return a
}

func approver(t *testing.T, a *App, name, pin string) *wsPeer {
	t.Helper()
	p := dialPeer(t, a)
	p.signIn(name, pin)
	p.cardsOn()
	return p
}

// Messages run in order, so any later answer proves that the server has it.
func (p *wsPeer) cardsOn() {
	p.t.Helper()
	p.send(map[string]any{"t": "approvalCards", "on": true})
	p.ask("settingsReview", 99, map[string]any{"fields": map[string]any{}})
}

func (p *wsPeer) lock() {
	p.t.Helper()
	p.send(map[string]any{"t": "userLock"})
	for m := p.until("lockState"); m["locked"] != true; m = p.until("lockState") {
	}
}

func (p *wsPeer) within(typ string, d time.Duration) map[string]any {
	p.t.Helper()
	end := time.After(d)
	for {
		select {
		case m, ok := <-p.reads:
			if !ok {
				return nil
			}
			if m["t"] == typ {
				return m
			}
		case <-end:
			return nil
		}
	}
}

func (p *wsPeer) askSave(saveID int, set map[string]any) map[string]any {
	p.t.Helper()
	vers, _ := p.hello["versions"].(map[string]any)
	fields := map[string]any{}
	for k, v := range set {
		fields[k] = map[string]any{"value": v, "version": vers[k]}
	}
	return p.ask("approvalAsk", 50, map[string]any{"for": map[string]any{"t": "settingsSave", "id": saveID, "fields": fields}})
}

func (p *wsPeer) answer(req any, approve bool, pin string) map[string]any {
	p.t.Helper()
	p.send(map[string]any{"t": "approvalAnswer", "request": req, "approve": approve, "pin": pin})
	return p.until("approvalAnswerResult")
}

func wantEnd(t *testing.T, m map[string]any, end, by string) {
	t.Helper()
	if m == nil || m["end"] != end || m["by"] != by && by != "*" {
		t.Fatalf("want end %q by %q, got %v", end, by, m)
	}
}

func asked(t *testing.T, r map[string]any) {
	t.Helper()
	if r["ok"] != true {
		t.Fatalf("approvalAsk refused: %v", r)
	}
}

func TestRequestApprovalApproved(t *testing.T) {
	a := askApp(t, time.Minute)
	defer startLog(t, a)()
	ada := approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
	w := ana.until("approvalWaiting")
	if left, _ := w["left"].(float64); left <= 0 || left > 60_000 {
		t.Fatalf("waiting left = %v", w["left"])
	}

	card := ada.until("approvalCard")
	if card["from"] != "Ana" || card["browser"] == "" || card["what"] != "Saving these changes" {
		t.Fatalf("card: %v", card)
	}
	ch, _ := card["changes"].([]any)
	if len(ch) != 1 || !strings.Contains(ch[0].(map[string]any)["key"].(string), "confirmLockoutMs") {
		t.Fatalf("card changes: %v", card["changes"])
	}
	if r := ada.answer(card["request"], true, "2468"); r["ok"] != true {
		t.Fatalf("approve: %v", r)
	}
	wantEnd(t, ana.until("approvalEnded"), "approved", "Ada")
	r := ana.until("settingsSaveResult")
	if r["id"] != float64(7) {
		t.Fatalf("result id %v, want the frozen save's 7", r["id"])
	}
	wantSaved(t, r, "confirmLockoutMs")
	rows := waitSaves(t, a, 1)
	if rows[0].By != "Ana" || rows[0].Save.Approver != "Ada" {
		t.Fatalf("Settings saves row by %q, approver %q", rows[0].By, rows[0].Save.Approver)
	}
	waitRequestRows(t, a, "sent", "approved")
}

func waitRequestRows(t *testing.T, a *App, want ...string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var got []string
	for time.Now().Before(deadline) {
		got = got[:0]
		for _, e := range logged(t, a) {
			if e.Why == "approvalRequest" {
				got = append(got, e.Result)
			}
		}
		if strings.Join(got, ",") == strings.Join(want, ",") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Request approval rows %v, want %v", got, want)
}

func TestRequestApprovalDeniedFirstAnswerWins(t *testing.T) {
	a := askApp(t, time.Minute)
	defer startLog(t, a)()
	ada1 := approver(t, a, "Ada", "2468")
	ada2 := approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
	c1, c2 := ada1.until("approvalCard"), ada2.until("approvalCard")
	if r := ada1.answer(c1["request"], false, ""); r["ok"] != true {
		t.Fatalf("deny: %v", r)
	}
	wantEnd(t, ana.until("approvalEnded"), "denied", "Ada")
	wantEnd(t, ada2.until("approvalCardClosed"), "denied", "Ada")
	if r := ada2.answer(c2["request"], true, "2468"); r["ok"] == true {
		t.Fatalf("a second answer was taken: %v", r)
	}
	if a.r().cfg.ConfirmLockoutMs == 900 {
		t.Fatal("a denied request saved")
	}
	waitRequestRows(t, a, "sent", "denied")
}

func TestRequestApprovalWrongPinKeepsIt(t *testing.T) {
	a := askApp(t, time.Minute)
	ada := approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
	card := ada.until("approvalCard")
	if r := ada.answer(card["request"], true, "1111"); r["ok"] == true || !strings.Contains(r["err"].(string), "wrong PIN") {
		t.Fatalf("wrong PIN: %v", r)
	}
	if r := ada.answer(card["request"], true, "2468"); r["ok"] != true {
		t.Fatalf("approve: %v", r)
	}
	wantSaved(t, ana.until("settingsSaveResult"), "confirmLockoutMs")
}

func TestRequestApprovalTimeout(t *testing.T) {
	a := askApp(t, 150*time.Millisecond)
	defer startLog(t, a)()
	ada := approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
	ada.until("approvalCard")
	wantEnd(t, ana.until("approvalEnded"), "timeout", "")
	wantEnd(t, ada.until("approvalCardClosed"), "timeout", "")
	waitRequestRows(t, a, "sent", "timeout")
}

func TestRequestApprovalWaitsWhenApproversLeave(t *testing.T) {
	a := askApp(t, 300*time.Millisecond)
	ada := approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
	ada.until("approvalCard")
	ada.ws.CloseNow()
	if m := ana.within("approvalEnded", 150*time.Millisecond); m != nil {
		t.Fatalf("ended when the approver left: %v", m)
	}
	wantEnd(t, ana.until("approvalEnded"), "timeout", "")
}

func TestRequestApprovalCancelled(t *testing.T) {
	for _, how := range []string{"cancel", "close", "signOut", "stepIn"} {
		t.Run(how, func(t *testing.T) {
			a := askApp(t, time.Minute)
			defer startLog(t, a)()
			ada := approver(t, a, "Ada", "2468")
			ana := dialPeer(t, a)
			ana.signIn("Ana", "1234")
			asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
			ada.until("approvalCard")
			switch how {
			case "cancel":
				ana.send(map[string]any{"t": "approvalCancel"})
			case "close":
				ana.ws.CloseNow()
			case "signOut":
				ana.send(map[string]any{"t": "userSignOut"})
			case "stepIn":
				ana.send(map[string]any{"t": "stepIn", "user": "Bo", "pin": "567890"})
			}
			wantEnd(t, ada.until("approvalCardClosed"), "cancelled", "")
			if how != "close" {
				wantEnd(t, ana.until("approvalEnded"), "cancelled", "")
			}
			waitRequestRows(t, a, "sent", "cancelled")
		})
	}
}

func TestRequestApprovalSurvivesRequesterLock(t *testing.T) {
	a := askApp(t, time.Minute)
	ada := approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
	card := ada.until("approvalCard")
	ana.lock()
	if m := ada.within("approvalCardClosed", 100*time.Millisecond); m != nil {
		t.Fatalf("the requester's Lock closed the card: %v", m)
	}
	if r := ada.answer(card["request"], true, "2468"); r["ok"] != true {
		t.Fatalf("approve: %v", r)
	}
	wantSaved(t, ana.until("settingsSaveResult"), "confirmLockoutMs")
	if a.r().cfg.ConfirmLockoutMs != 900 {
		t.Fatal("not saved")
	}
}

func TestRequestApprovalCardsReachOnlyQualified(t *testing.T) {
	a := askApp(t, time.Minute)
	bo := approver(t, a, "Bo", "567890") // Trusted is below the Administrator field
	setFlags(t, a, "Bo", true, false)
	silent := dialPeer(t, a) // a panel page: no cards
	silent.signIn("Ada", "2468")
	locked := approver(t, a, "Ada", "2468")
	locked.lock()
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	ana.cardsOn()
	late := dialPeer(t, a)
	late.signIn("Ada", "2468")
	approver(t, a, "Ada", "2468") // someone must qualify when the request is sent

	asked(t, ana.askSave(7, map[string]any{"router": "127.0.0.1:9"}))
	time.Sleep(50 * time.Millisecond)
	late.send(map[string]any{"t": "approvalCards", "on": true})
	card := late.until("approvalCard")
	if left, _ := card["left"].(float64); left <= 0 || left >= 60_000 {
		t.Fatalf("late card left = %v, want under the full wait", card["left"])
	}
	for name, p := range map[string]*wsPeer{"Trusted": bo, "no cards": silent, "locked": locked, "requester": ana} {
		if m := p.within("approvalCard", 50*time.Millisecond); m != nil {
			t.Fatalf("%s got a card: %v", name, m)
		}
	}
	locked.send(map[string]any{"t": "userSignIn", "user": "Ada", "pin": "2468", "id": 1})
	locked.until("approvalCard")
}

func TestRequestApprovalNeedsSomeoneSignedIn(t *testing.T) {
	a := askApp(t, time.Minute)
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	r := ana.askSave(7, map[string]any{"confirmLockoutMs": 900})
	if r["ok"] == true || !strings.Contains(r["err"].(string), "no one signed in can approve this") {
		t.Fatalf("with nobody to approve: %v", r)
	}
	ada := approver(t, a, "Ada", "2468")
	for m := ana.until("approvers"); m["administrator"] != true; m = ana.until("approvers") {
	}
	ada.ws.CloseNow()
	for m := ana.until("approvers"); m["administrator"] != false; m = ana.until("approvers") {
	}
}

func TestRequestApprovalOnePerBrowser(t *testing.T) {
	a := askApp(t, time.Minute)
	approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"confirmLockoutMs": 900}))
	if r := ana.askSave(8, map[string]any{"brandText": "X"}); r["ok"] == true {
		t.Fatalf("a second request was taken: %v", r)
	}
	ana.send(map[string]any{"t": "approvalCancel"})
	ana.until("approvalEnded")
	if r := ana.askSave(9, map[string]any{"noSuchField": 1}); r["ok"] == true {
		t.Fatalf("a request for nothing that needs an Approval was taken: %v", r)
	}
	if r := ana.ask("approvalAsk", 51, map[string]any{"for": map[string]any{"t": "take"}}); r["ok"] == true {
		t.Fatalf("a take was taken: %v", r)
	}
}

func TestRequestApprovalCardLeavesOutHiddenFields(t *testing.T) {
	a := askApp(t, time.Minute)
	setFlags(t, a, "Bo", true, false)
	bo := approver(t, a, "Bo", "567890")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.askSave(7, map[string]any{"brandText": "X", "autoLockMinutes": 5}))
	ch, _ := bo.until("approvalCard")["changes"].([]any)
	if len(ch) != 1 || ch[0].(map[string]any)["key"] != "brandText" {
		t.Fatalf("card changes: %v", ch)
	}
}

func TestRequestApprovalClearHistory(t *testing.T) {
	a := askApp(t, time.Minute)
	ada := approver(t, a, "Ada", "2468")
	ana := dialPeer(t, a)
	ana.signIn("Ana", "1234")
	asked(t, ana.ask("approvalAsk", 50, map[string]any{"for": map[string]any{"t": "clearHistory"}}))
	card := ada.until("approvalCard")
	if card["what"] != "Clearing History" {
		t.Fatalf("card: %v", card)
	}
	ada.answer(card["request"], true, "2468")
	ana.until("historyCleared")
}
