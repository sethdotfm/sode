package app

import (
	"strings"
	"testing"
)

func TestActorLabel(t *testing.T) {
	cases := []struct {
		a    Actor
		want string
	}{
		{Actor{Kind: actorUser, Name: "Seth", Key: "Panel 21"}, "Seth (Panel 21)"},
		{Actor{Kind: actorClient, Name: "PanelsBot", Key: "Panel 43"}, "PanelsBot (Panel 43)"},
		{Actor{Kind: actorClient, Name: "PanelsBot"}, "PanelsBot"},
	}
	for _, c := range cases {
		if got := c.a.String(); got != c.want {
			t.Errorf("%+v: %q, want %q", c.a, got, c.want)
		}
	}
}

func TestAPIEventRow(t *testing.T) {
	a := newTestApp(t)
	defer startLog(t, a)()
	act := Actor{Kind: actorClient, Name: "PanelsBot", Key: "Panel 43", Via: "wall-ui 2.1"}
	a.apiEvent(act, "take", "refused", "outside rights", nil)
	rows := waitLogged(t, a, 1)
	e := rows[len(rows)-1]
	if e.Kind != kindAPI || e.By != "PanelsBot (Panel 43)" || e.Why != "take" || e.Result != "refused" {
		t.Fatalf("row %+v", e)
	}
	if e.Cmd != -1 || e.Dst != -1 || e.Src != -1 || e.Wire != -1 || e.Hex != "" {
		t.Errorf("not shaped like an event: %+v", e)
	}
	if !strings.Contains(e.Note, "outside rights") || !strings.Contains(e.Note, "unverified") || !strings.Contains(e.Note, "wall-ui 2.1") {
		t.Errorf("note %q", e.Note)
	}
}

func TestAPIEventNeverWritesAKey(t *testing.T) {
	a := newTestApp(t)
	defer startLog(t, a)()
	k := newKeySecret()
	a.apiEvent(Actor{Kind: actorClient, Name: "Bot", Key: "k"}, "auth", "refused", "bad key "+k, errString("header Bearer "+k))
	rows := waitLogged(t, a, 1)
	e := rows[len(rows)-1]
	if strings.Contains(e.Note, k) || strings.Contains(e.Err, k) || strings.Contains(e.By, k) {
		t.Fatalf("key in row: %+v", e)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestEntryActorNeverRendersAsAddress(t *testing.T) {
	act := Actor{Kind: actorClient, Name: "PanelsBot", Key: "Panel 43"}
	e := &Entry{}
	e.setActor(act)
	if e.Actor == nil || e.Actor.Name != "PanelsBot" || e.Client != "PanelsBot (Panel 43)" {
		t.Fatalf("entry %+v", e)
	}
}
