package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func (p *wsPeer) saveApproved(id int, set map[string]any, name, pin string) map[string]any {
	p.t.Helper()
	vers, _ := p.hello["versions"].(map[string]any)
	fields := map[string]any{}
	for k, v := range set {
		fields[k] = map[string]any{"value": v, "version": vers[k]}
	}
	return p.ask("settingsSave", id, map[string]any{"fields": fields, "approval": map[string]any{"user": name, "pin": pin}})
}

func approvalRefused(t *testing.T, what string, r map[string]any, says string) {
	t.Helper()
	e, _ := r["err"].(string)
	if r["ok"] == true || r["approval"] != true || !strings.Contains(e, says) {
		t.Fatalf("%s: want an Approval refusal that says %q, got %v", what, says, r)
	}
}

func TestApprovalCoversOneSave(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	wantRefused(t, p.saveFields(1, map[string]any{"confirmLockoutMs": 900}), "confirmLockoutMs", "needs Approval")

	r := p.saveApproved(2, map[string]any{"confirmLockoutMs": 900}, "Ada", "2468")
	wantSaved(t, r, "confirmLockoutMs")
	if got := a.r().cfg.ConfirmLockoutMs; got != 900 {
		t.Fatalf("confirmLockoutMs = %d", got)
	}
	rows := waitSaves(t, a, 1)
	if rows[0].By != "Ana" || rows[0].Save.Approver != "Ada" {
		t.Fatalf("Settings saves row by %q, approver %q; want Ana and Ada", rows[0].By, rows[0].Save.Approver)
	}

	wantRefused(t, p.saveFields(3, map[string]any{"confirmLockoutMs": 700}), "confirmLockoutMs", "needs Approval")
}

func TestApprovalNeedsApproverRole(t *testing.T) {
	a := userApp(t, false)
	setFlags(t, a, "Bo", true, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	approvalRefused(t, "Trusted approver", p.saveApproved(1, map[string]any{"confirmLockoutMs": 900}, "Bo", "567890"), "Bo")
	if a.r().cfg.ConfirmLockoutMs == 900 {
		t.Fatal("a refused Approval saved the field")
	}
	wantSaved(t, p.saveApproved(2, map[string]any{"brandText": "Studio B"}, "Bo", "567890"), "brandText")
}

// A Hidden section needs an Administrator signed in on that browser.
func TestApprovalLeavesHiddenSectionsRefused(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	r := p.saveApproved(1, map[string]any{"autoLockMinutes": 5, "brandText": "X"}, "Ada", "2468")
	wantRefused(t, r, "autoLockMinutes", "Role")
	wantSaved(t, r, "brandText")
}

func TestApprovalPinCountsTowardApproverWait(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	approvalRefused(t, "wrong PIN", p.saveApproved(1, map[string]any{"brandText": "X"}, "Ada", "1111"), "wrong PIN")
	a.mu.Lock()
	n := a.userPins[idOf(a, "Ada")].n
	a.mu.Unlock()
	if n != 1 {
		t.Fatalf("Ada's wrong PINs = %d, want 1", n)
	}
	if a.r().cfg.BrandText == "X" {
		t.Fatal("a wrong Approval PIN saved the field")
	}
}

func TestApprovalNeedsApproverWithPin(t *testing.T) {
	a := userApp(t, false)
	a.cfgMu.Lock()
	_, nr, err := a.update(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Bo" {
				cfg.Users[i].Pin, cfg.Users[i].Len = "", 0
			}
		}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.mu.Unlock()
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	approvalRefused(t, "approver without a PIN", p.saveApproved(1, map[string]any{"brandText": "X"}, "Bo", ""), "no PIN")
}

func TestApprovalRefusedUnderLock(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	p.send(map[string]any{"t": "userLock"})
	p.until("lockState")
	r := p.saveApproved(1, map[string]any{"brandText": "X"}, "Ada", "2468")
	if r["ok"] == true || a.r().cfg.BrandText == "X" {
		t.Fatalf("an Approval under the Lock saved: %v", r)
	}
	a.mu.Lock()
	pool := a.userPins[idOf(a, "Ada")]
	a.mu.Unlock()
	if pool != nil && pool.n != 0 {
		t.Fatal("an Approval under the Lock checked the PIN")
	}
}

func TestAdministratorApprovalCountsAsRecheck(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	wantSaved(t, p.saveApproved(1, map[string]any{aKey(a, "address"): "127.0.0.1:9"}, "Ada", "2468"), aKey(a, "address"))

	// Clear History needs an Administrator and a recheck; the Approval gives both.
	p.send(map[string]any{"t": "clearHistory"})
	if r := p.until("clearHistoryResult"); r["ok"] == true {
		t.Fatalf("a Standard user cleared History: %v", r)
	}
	p.send(map[string]any{"t": "clearHistory", "approval": map[string]any{"user": "Ada", "pin": "2468"}})
	p.until("historyCleared")
}

// A take uses the browser's own sign-in.
func TestApprovalNotForTakes(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	p.send(map[string]any{"t": "take", "items": []any{map[string]any{"id": 7, "dst": 1, "src": 1, "levels": []int{1}}},
		"approval": map[string]any{"user": "Ada", "pin": "2468"}})
	r := p.until("takeResult")
	if e, _ := r["err"].(string); !strings.Contains(e, "Approval") {
		t.Fatalf("a take with an Approval: %v", r)
	}
}
