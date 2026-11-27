package app

import (
	"strings"
	"testing"
)

// The first browser to claim setup runs it, and a second is refused. Take control moves setup
// to the second. The first gets a message, its Router step address goes back, and its Finish
// is refused.
func TestSetupTakeControlMovesSetup(t *testing.T) {
	a := setupApp(t)
	before := a.r().cfg.Routers[0].Address
	p := dialPeer(t, a)
	q := dialPeer(t, a)

	if r := p.ask("setupClaim", 1, nil); r["ok"] != true {
		t.Fatalf("first claim: %v", r)
	}
	if r := p.ask("setupRouter", 2, map[string]any{"router": "127.0.0.1:2"}); r["ok"] != true {
		t.Fatalf("owner's router step: %v", r)
	}
	if r := q.ask("setupClaim", 3, nil); r["ok"] == true {
		t.Fatalf("a second browser got setup while the first holds it: %v", r)
	}
	if r := q.ask("setupRouter", 4, map[string]any{"router": "127.0.0.1:3"}); r["ok"] == true {
		t.Fatalf("a waiting browser moved the router: %v", r)
	}

	if r := q.ask("setupClaim", 5, map[string]any{"take": true}); r["ok"] != true {
		t.Fatalf("Take control: %v", r)
	}
	p.until("setupTaken")
	if got := a.r().cfg.Routers[0].Address; got != before {
		t.Fatalf("the first browser's router address stayed: %q, want %q", got, before)
	}
	if r := p.ask("setupFinish", 6, map[string]any{"setup": wizardAnswers()}); r["ok"] == true || !strings.Contains(r["err"].(string), "another browser") {
		t.Fatalf("the first browser's Finish after Take control: %v", r)
	}
	if r := q.ask("setupFinish", 7, map[string]any{"setup": wizardAnswers()}); r["ok"] != true {
		t.Fatalf("the new owner's Finish: %v", r)
	}
}

// When the browser that runs setup closes, the waiting browsers are told setup is free.
func TestSetupOwnerGoneFreesSetup(t *testing.T) {
	a := setupApp(t)
	p := dialPeer(t, a)
	q := dialPeer(t, a)
	if r := p.ask("setupClaim", 1, nil); r["ok"] != true {
		t.Fatalf("first claim: %v", r)
	}
	p.ws.CloseNow()
	q.until("setupFree")
	if r := q.ask("setupClaim", 2, nil); r["ok"] != true {
		t.Fatalf("claim after the owner left: %v", r)
	}
}
