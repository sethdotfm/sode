package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// inboxApp is userApp with router names read, so references check, and Ana and Bo signed in.
func inboxApp(t *testing.T) (a *App, ana, bo *client) {
	t.Helper()
	a = userApp(t, false)
	a.mu.Lock()
	for n := 1; n <= 8; n++ {
		a.first.router.srcNames[n] = "CAM " + string(rune('0'+n))
		a.first.router.dstNames[n] = "MON " + string(rune('0'+n))
	}
	a.mu.Unlock()
	ana = watched(t, a, "10.0.0.1", "1234")
	bo = watched(t, a, "10.0.0.2", "567890")
	return a, ana, bo
}

// lastInbox drains c's queue and returns the last inbox message and every card that was new.
func lastInbox(t *testing.T, c *client) (inboxMsg, []string) {
	t.Helper()
	var last inboxMsg
	var fresh []string
	got := false
	for {
		select {
		case b := <-c.send:
			var m inboxMsg
			if json.Unmarshal(b, &m) == nil && m.T == "inbox" {
				last, got = m, true
				fresh = append(fresh, m.New...)
			}
		default:
			if !got {
				t.Fatalf("no inbox message")
			}
			return last, fresh
		}
	}
}

func drainSend(c *client) {
	for len(c.send) > 0 {
		<-c.send
	}
}

// inboxRows drains the Log queue and returns the Notes of rows with why.
func inboxRows(a *App, why string) []string {
	var out []string
	for len(a.evq) > 0 {
		if e := <-a.evq; e.Why == why {
			out = append(out, e.By+": "+e.Note)
		}
	}
	return out
}

func mePreset(name string, src, dst int) config.Preset {
	return config.Preset{ID: config.NewPresetID(), Name: name, Items: []config.PresetItem{presetItem(src, nil, dst)}}
}

func setProfile(t *testing.T, a *App, name string, p config.Profile) {
	t.Helper()
	lifeCommit(t, a, func(cfg *config.Config) {
		for i := range cfg.Users {
			if cfg.Users[i].Name == name {
				cfg.Users[i].Profile = p
			}
		}
	})
}

func profileOf(t *testing.T, a *App, name string) config.Profile {
	t.Helper()
	u, ok := a.r().cfg.FindUserByName(name)
	if !ok {
		t.Fatalf("no user %s", name)
	}
	return u.Profile
}

func sendOne(t *testing.T, a *App, from *client, req inboxReq) inboxCard {
	t.Helper()
	cards, _, err := a.SendCopy(from, req)
	if err != nil || len(cards) != 1 {
		t.Fatalf("send: %v %+v", err, cards)
	}
	return cards[0]
}

// One send of a Me preset, a This browser preset and a Me filter set is one card. Accept makes each the
// recipient's own Me item, and the sender's Sent list says Accepted.
func TestInboxSendAndAccept(t *testing.T) {
	a, ana, bo := inboxApp(t)
	show := mePreset("Show", 1, 2)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{show}, FilterSets: []config.FilterSet{{ID: "set1", Name: "Cams", Src: "CAM", Sode: true, Ext: true}}})
	drainSend(ana)
	drainSend(bo)
	browser := mePreset("Desk", 3, 4)
	k := sendOne(t, a, ana, inboxReq{
		To:         []string{"bo"},
		Presets:    []cardPresetReq{{Scope: scopeMe, ID: show.ID}, {Scope: scopeBrowser, Preset: &browser}},
		FilterSets: []cardSetReq{{Scope: scopeMe, ID: "set1"}},
	})
	if k.Count != 3 {
		t.Fatalf("card count %d", k.Count)
	}
	if rows := inboxRows(a, "inboxSend"); len(rows) != 0 {
		t.Fatalf("SendCopy logs nothing itself: %v", rows)
	}

	m, fresh := lastInbox(t, bo)
	if len(m.Cards) != 1 || m.Cards[0].From != "Ana" || len(m.Cards[0].Items) != 3 || len(fresh) != 1 || fresh[0] != k.ID {
		t.Fatalf("Bo's inbox: %+v new %v", m, fresh)
	}
	if exp := m.Cards[0].Expires - m.Cards[0].At; exp != (30 * 24 * time.Hour).Milliseconds() {
		t.Fatalf("expires after %d ms", exp)
	}
	s, fresh := lastInbox(t, ana)
	if len(s.Sent) != 1 || s.Sent[0].State != cardWaiting || s.Sent[0].To != "Bo" || len(s.Cards) != 0 || len(fresh) != 0 {
		t.Fatalf("Ana's lists: %+v new %v", s, fresh)
	}

	// A later edit of the original does not reach the card.
	setProfile(t, a, "Ana", config.Profile{})
	plan, failed, _, err := a.AcceptCard(bo, k.ID, importReq{})
	if err != nil || failed != nil || plan.Critical {
		t.Fatalf("accept: %v %v %+v", err, failed, plan)
	}
	p := profileOf(t, a, "Bo")
	if len(p.Presets) != 2 || p.Presets[0].Name != "Show" || p.Presets[1].Name != "Desk" || len(p.FilterSets) != 1 || p.FilterSets[0].Name != "Cams" {
		t.Fatalf("Bo's profile: %+v", p)
	}
	s, _ = lastInbox(t, ana)
	if s.Sent[0].State != cardAccepted || s.Sent[0].Accepted != 3 || s.Sent[0].Count != 3 {
		t.Fatalf("Ana's Sent list: %+v", s.Sent)
	}
	if m, _ := lastInbox(t, bo); len(m.Cards) != 0 {
		t.Fatalf("Bo's Inbox after accept: %+v", m.Cards)
	}
	if _, _, _, err := a.AcceptCard(bo, k.ID, importReq{}); err == nil {
		t.Fatal("a second accept of the same card worked")
	}
}
