package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func TestInboxStoreReloads(t *testing.T) {
	path := t.TempDir() + "/config.yaml"
	s := loadInbox(path)
	cards, full, err := s.add([]inboxCard{{From: "u-a", To: "u-b", ToName: "Bo", State: cardWaiting, Count: 1,
		Items: cardItems{Panels: []Panel{{ID: "p-00000001", Name: "Desk", Art: []PanelArt{{ID: "a-00000001", Type: "image/png"}}}}}}},
		map[string][]byte{"p-00000001/a-00000001": []byte("png")})
	if err != nil || full != "" || len(cards) != 1 {
		t.Fatalf("add: %v %q %+v", err, full, cards)
	}
	again := loadInbox(path)
	k, ok := again.get(cards[0].ID)
	if !ok || k.State != cardWaiting || len(k.Items.Panels) != 1 || k.Items.Panels[0].Name != "Desk" {
		t.Fatalf("reloaded %+v %v", k, ok)
	}
	if b, ok := again.artData(k.ID, "p-00000001", "a-00000001"); !ok || string(b) != "png" {
		t.Fatalf("art %q %v", b, ok)
	}
}

func TestInboxMessagesLog(t *testing.T) {
	a, ana, bo := inboxApp(t)
	p := mePreset("Show", 1, 2)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{p}})
	inboxRows(a, "")
	send := func(c *client, m inMsg) inboxResult {
		t.Helper()
		drainSend(c)
		m.done = func() {}
		policy[m.T].handle(a, c, &m)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case b := <-c.send:
				var r inboxResult
				if json.Unmarshal(b, &r) == nil && r.T == "inboxResult" {
					return r
				}
			case <-time.After(10 * time.Millisecond):
			}
		}
		t.Fatalf("no inboxResult for %s", m.T)
		return inboxResult{}
	}
	body := &inboxReq{To: []string{"Bo"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}}
	r := send(ana, inMsg{T: "inboxSend", ID: 7, Inbox: body})
	if !r.OK || r.ID != 7 || len(r.Cards) != 1 {
		t.Fatalf("send: %+v", r)
	}
	first := r.Cards[0]
	if rows := inboxRows(a, "inboxSend"); len(rows) != 1 || rows[0] != "Ana: to Bo: 1 preset" {
		t.Fatalf("send rows %q", rows)
	}
	r = send(bo, inMsg{T: "inboxDecline", Inbox: &inboxReq{Card: first}})
	if !r.OK {
		t.Fatalf("decline: %+v", r)
	}
	if rows := inboxRows(a, "inboxDecline"); len(rows) != 1 || rows[0] != "Bo: from Ana: 1 item" {
		t.Fatalf("decline rows %q", rows)
	}
	if len(profileOf(t, a, "Bo").Presets) != 0 {
		t.Fatal("a declined card wrote a preset")
	}
	second := send(ana, inMsg{T: "inboxSend", Inbox: body}).Cards[0]
	r = send(bo, inMsg{T: "inboxReview", Inbox: &inboxReq{Card: second}})
	if !r.OK || r.Plan == nil || len(r.Plan.Categories[catPresets].Items) != 1 {
		t.Fatalf("review: %+v", r)
	}
	r = send(bo, inMsg{T: "inboxAccept", Inbox: &inboxReq{Card: second}})
	if !r.OK || len(profileOf(t, a, "Bo").Presets) != 1 {
		t.Fatalf("accept: %+v", r)
	}
	if rows := inboxRows(a, "inboxAccept"); len(rows) != 1 || rows[0] != "Bo: from Ana: accepted 1 of 1" {
		t.Fatalf("accept rows %q", rows)
	}
	third := send(ana, inMsg{T: "inboxSend", Inbox: body}).Cards[0]
	if r := send(bo, inMsg{T: "inboxWithdraw", Inbox: &inboxReq{Card: third}}); r.OK {
		t.Fatal("the recipient withdrew a card")
	}
	if r := send(ana, inMsg{T: "inboxWithdraw", Inbox: &inboxReq{Card: third}}); !r.OK {
		t.Fatalf("withdraw: %+v", r)
	}
	if rows := inboxRows(a, "inboxWithdraw"); len(rows) != 2 || rows[1] != "Ana: to Bo: 1 item" {
		t.Fatalf("withdraw rows %q", rows)
	}
	if r := send(bo, inMsg{T: "inboxAccept", Inbox: &inboxReq{Card: third}}); r.OK {
		t.Fatal("a withdrawn card was accepted")
	}
	if r := send(ana, inMsg{T: "inboxWithdraw", Inbox: &inboxReq{Card: second}}); r.OK {
		t.Fatal("an answered card was withdrawn")
	}
	r = send(ana, inMsg{T: "inboxList"})
	if !r.OK {
		t.Fatalf("list: %+v", r)
	}
	a.mu.Lock()
	a.pushInboxMu(ana, nil)
	a.mu.Unlock()
	m, _ := lastInbox(t, ana)
	states := []string{}
	for _, v := range m.Sent {
		states = append(states, v.State)
	}
	if strings.Join(states, " ") != "declined accepted withdrawn" {
		t.Fatalf("Sent list states %v", states)
	}
}

func TestInboxExpires(t *testing.T) {
	a, ana, bo := inboxApp(t)
	p := mePreset("Show", 1, 2)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{p}})
	now := time.Now()
	a.now = func() time.Time { return now }
	a.inboxTTL = time.Hour
	k := sendOne(t, a, ana, inboxReq{To: []string{"Bo"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}})
	inboxRows(a, "")
	now = now.Add(59 * time.Minute)
	a.inboxSweep()
	if got, _ := a.inbox.get(k.ID); got.State != cardWaiting {
		t.Fatalf("expired early: %s", got.State)
	}
	now = now.Add(time.Minute)
	a.inboxSweep()
	if rows := inboxRows(a, "inboxExpired"); len(rows) != 1 || rows[0] != "Sode: from Ana to Bo: 1 item" {
		t.Fatalf("expiry rows %q", rows)
	}
	if m, _ := lastInbox(t, bo); len(m.Cards) != 0 {
		t.Fatalf("Bo still holds %+v", m.Cards)
	}
	if m, _ := lastInbox(t, ana); m.Sent[0].State != cardExpired {
		t.Fatalf("Ana's Sent list %+v", m.Sent)
	}
	if _, _, _, err := a.AcceptCard(bo, k.ID, importReq{}); err == nil {
		t.Fatal("an expired card was accepted")
	}
	now = now.Add(inboxKeep)
	a.inboxSweep()
	if _, ok := a.inbox.get(k.ID); ok {
		t.Fatal("an answered card outlived inboxKeep")
	}
}

func TestInboxFull(t *testing.T) {
	a, ana, bo := inboxApp(t)
	p := mePreset("Show", 1, 2)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{p}})
	req := inboxReq{To: []string{"Bo"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}}
	var last inboxCard
	for range inboxWaitingMax {
		last = sendOne(t, a, ana, req)
	}
	if _, _, err := a.SendCopy(ana, req); err == nil || err.Error() != "Bo's Inbox is full" {
		t.Fatalf("101st card: %v", err)
	}
	// One full recipient stops the whole send, so nobody gets part of it.
	req.To = []string{"Ada", "Bo"}
	if _, _, err := a.SendCopy(ana, req); err == nil {
		t.Fatal("a send to a full Inbox worked")
	}
	if n := len(a.inbox.cards(func(k inboxCard) bool { return k.ToName == "Ada" })); n != 0 {
		t.Fatalf("Ada got %d cards", n)
	}
	if _, err := a.answerCard(bo, last.ID, cardDeclined); err != nil {
		t.Fatal(err)
	}
	req.To = []string{"Bo"}
	sendOne(t, a, ana, req)
}

// A Guest neither sends nor receives; a user without a PIN does both.
func TestInboxGuestAndNoPin(t *testing.T) {
	a, ana, _ := inboxApp(t)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users = append(cfg.Users, config.User{Name: "Cy", Role: config.RoleStandard})
	})
	p := mePreset("Show", 1, 2)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{p}})
	guest := watched(t, a, "10.0.0.9", "")
	browser := mePreset("Desk", 1, 2)
	if _, _, err := a.SendCopy(guest, inboxReq{To: []string{"Ana"}, Presets: []cardPresetReq{{Scope: scopeBrowser, Preset: &browser}}}); err != errGuestInbox {
		t.Fatalf("Guest send: %v", err)
	}
	cy := watched(t, a, "10.0.0.3", "")
	if err := a.UserUnlockAs(cy, "Cy", ""); err != nil {
		t.Fatal(err)
	}
	k := sendOne(t, a, ana, inboxReq{To: []string{"Cy"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}})
	if _, _, _, err := a.AcceptCard(cy, k.ID, importReq{}); err != nil {
		t.Fatalf("Cy accept: %v", err)
	}
	sendOne(t, a, cy, inboxReq{To: []string{"Ana"}, Presets: []cardPresetReq{{Scope: scopeBrowser, Preset: &browser}}})
	if _, _, err := a.SendCopy(ana, inboxReq{To: []string{"Guest"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}}); err == nil {
		t.Fatal("a send to no user worked")
	}
	if _, _, err := a.SendCopy(ana, inboxReq{To: []string{"Bo"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: "pr-none"}}}); err == nil {
		t.Fatal("a preset Ana does not have was sent")
	}
}

// A clash with the recipient's own preset defaults to Rename.
func TestInboxPartialAcceptAndRights(t *testing.T) {
	a, ana, bo := inboxApp(t)
	show, wide := mePreset("Show", 1, 2), mePreset("Wide", 7, 8)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{show, wide}})
	setProfile(t, a, "Bo", config.Profile{Presets: []config.Preset{mePreset("Show", 3, 3)}})
	lifeCommit(t, a, func(cfg *config.Config) {
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Bo" {
				cfg.Users[i].Rights = config.Rights{Sources: &[]config.Ref{{N: 1}}}
			}
		}
	})
	drainSend(bo)
	k := sendOne(t, a, ana, inboxReq{To: []string{"Bo"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: show.ID}, {Scope: scopeMe, ID: wide.ID}}})
	m, _ := lastInbox(t, bo)
	keys := map[string]string{}
	for _, it := range m.Cards[0].Items {
		keys[it.Name] = it.Key
	}
	plan, err := a.ReviewCard(bo, k.ID, importReq{})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range plan.Categories[catPresets].Items {
		switch it.Name {
		case "Show":
			if it.Clash != "Show" || it.Choice != useRename || it.As != "Show 2" || it.Note != "" {
				t.Errorf("Show: %+v", it)
			}
		case "Wide":
			if it.Note != noteOutsideRights {
				t.Errorf("Wide: %+v", it)
			}
		}
	}
	if _, _, _, err := a.AcceptCard(bo, k.ID, importReq{Pick: map[string][]string{catPresets: {keys["Show"]}}}); err != nil {
		t.Fatal(err)
	}
	if p := profileOf(t, a, "Bo"); len(p.Presets) != 2 || p.Presets[1].Name != "Show 2" {
		t.Fatalf("Bo's presets %+v", p.Presets)
	}
	if m, _ := lastInbox(t, ana); m.Sent[0].Accepted != 1 || m.Sent[0].Count != 2 {
		t.Fatalf("Sent %+v", m.Sent)
	}
}

// A removed recipient loses their cards, and the sender sees Expired. A removed sender's card stays.
func TestInboxUserRemoved(t *testing.T) {
	a, ana, bo := inboxApp(t)
	p := mePreset("Show", 1, 2)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{p}})
	setProfile(t, a, "Bo", config.Profile{Presets: []config.Preset{p}})
	toBo := sendOne(t, a, ana, inboxReq{To: []string{"Bo"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}})
	fromBo := sendOne(t, a, bo, inboxReq{To: []string{"Ana"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}})
	before := a.r().cfg.Users
	lifeCommit(t, a, func(cfg *config.Config) {
		var keep []config.User
		for _, u := range cfg.Users {
			if u.Name != "Bo" {
				keep = append(keep, u)
			}
		}
		cfg.Users = keep
	})
	a.panelsFollowUsers(before, a.r().cfg.Users)
	if got, _ := a.inbox.get(toBo.ID); got.State != cardExpired {
		t.Fatalf("card to Bo: %s", got.State)
	}
	m, _ := lastInbox(t, ana)
	if len(m.Sent) != 1 || m.Sent[0].State != cardExpired || m.Sent[0].To != "Bo" {
		t.Fatalf("Ana's Sent %+v", m.Sent)
	}
	if len(m.Cards) != 1 || m.Cards[0].ID != fromBo.ID || m.Cards[0].From != "" {
		t.Fatalf("Ana's Inbox %+v", m.Cards)
	}
	if _, _, _, err := a.AcceptCard(ana, fromBo.ID, importReq{}); err != nil {
		t.Fatalf("accept from a removed user: %v", err)
	}
}

func TestInboxAcceptOfOnlyRefusedItemsKeepsCard(t *testing.T) {
	a, ana, bo := inboxApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.PublicPanels = true })
	src, err := a.panels.create(Panel{Name: "Desk", Owner: userID(t, a, "Ana"), Acts: panelActsPublic, Rows: 1, Cols: 1,
		Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: []map[string]any{}}}})
	if err != nil {
		t.Fatal(err)
	}
	k := sendOne(t, a, ana, inboxReq{To: []string{"Bo"}, Panels: []string{src.ID}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.PublicPanels = false })
	if _, _, _, err := a.AcceptCard(bo, k.ID, importReq{}); err == nil {
		t.Fatal("an Accept that wrote nothing succeeded")
	}
	if got, _ := a.inbox.get(k.ID); got.State != cardWaiting {
		t.Fatalf("card state %q", got.State)
	}
}

func TestInboxSendsPanel(t *testing.T) {
	a, ana, bo := inboxApp(t)
	uid, anaID, boID := a.r().cfg.UID, userID(t, a, "Ana"), userID(t, a, "Bo")
	lifeCommit(t, a, func(cfg *config.Config) {
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Bo" {
				cfg.Users[i].Rights = config.Rights{Sources: &[]config.Ref{{N: 2}}}
			}
		}
	})
	src, err := a.panels.create(Panel{Name: "Desk", Owner: anaID, Acts: panelActsAPI, Slug: "desk", Rows: 1, Cols: 1,
		Share:    PanelShare{Mode: panelShareNamed, Users: []string{"Ada"}},
		Settings: map[string]any{panelModeKey: "instant"},
		Pages:    []PanelPage{{ID: "pg-1", Enabled: true, Cells: []map[string]any{{"kind": "source", "n": 2, "router": uid, "name": "CAM 2"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	rights := config.Rights{}
	if err := a.putPanelClient(sodeActor, src.ID, &rights, nil); err != nil {
		t.Fatal(err)
	}
	src.Client = panelClientName(src.ID)
	if _, err := a.panels.replace(src); err != nil {
		t.Fatal(err)
	}
	art, _, err := a.panels.addArt(src.ID, []byte("\x89PNG\r\n\x1a\nart"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	src, _ = a.panels.get(src.ID)
	if _, _, err := a.panels.pushLayout(src.ID, layoutOf(src), anaID, "push", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.panels.sets.create(PanelSet{Name: "Show", Scope: panelShareMe, Owner: anaID, Entries: []PanelSetEntry{{Panel: src.ID, Name: "Desk", panelLayout: layoutOf(src)}}}); err != nil {
		t.Fatal(err)
	}
	k := sendOne(t, a, ana, inboxReq{To: []string{"Bo"}, Panels: []string{src.ID}})
	// The card is frozen: deleting the original's artwork does not reach it.
	if _, err := a.panels.delArt(src.ID, art.ID); err != nil {
		t.Fatal(err)
	}
	plan, failed, _, err := a.AcceptCard(bo, k.ID, importReq{})
	if err != nil || failed != nil {
		t.Fatalf("accept: %v %v", err, failed)
	}
	if it := plan.Categories[catPanels].Items[0]; it.Note != "" {
		t.Errorf("a panel inside Bo's Rights noted %q", it.Note)
	}
	var got Panel
	for _, p := range a.panels.list() {
		if p.Owner == boID {
			got = p
		}
	}
	switch {
	case got.ID == "" || got.ID == src.ID:
		t.Fatalf("no copy: %+v", a.panels.list())
	case got.Name != "Desk" || got.Share.Mode != panelShareMe || len(got.Share.Users) != 0 || got.Slug != "":
		t.Errorf("copy: %+v", got)
	case got.Settings[panelModeKey] != "instant" || len(got.Pages) != 1 || got.Pages[0].Cells[0]["name"] != "CAM 2":
		t.Errorf("copy lost its Pages or settings: %+v", got)
	case len(got.Art) != 1 || got.Art[0].ID != art.ID:
		t.Errorf("copy art %+v", got.Art)
	}
	if b, _, ok := a.panels.artData(got.ID, art.ID); !ok || string(b) != "\x89PNG\r\n\x1a\nart" {
		t.Errorf("art data %q", b)
	}
	if cps, _ := a.panels.checkpoints(got.ID); len(cps) != 0 {
		t.Errorf("checkpoints %+v", cps)
	}
	for _, set := range a.panels.sets.visible(boID) {
		for _, e := range set.Entries {
			if e.Panel == got.ID {
				t.Errorf("the copy is in Checkpoint set %s", set.Name)
			}
		}
	}
	cl, ok := clientOfPanel(a.r().cfg, got)
	if !ok || cl.Name == src.Client || cl.Rights.Sources == nil || len(*cl.Rights.Sources) != 1 || (*cl.Rights.Sources)[0].N != 2 {
		t.Fatalf("copy's client %+v %v", cl, ok)
	}
	for _, key := range a.keys.list() {
		if key.Owner.Name == cl.Name {
			t.Errorf("the copy's client has key %+v", key)
		}
	}
	if _, ok := a.inbox.artData(k.ID, src.ID, art.ID); ok {
		t.Error("the accepted card kept its artwork")
	}
}
