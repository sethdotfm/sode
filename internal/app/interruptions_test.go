package app

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Seam: the review's `settingsReview` and the one Save's `settingsSave` over the WebSocket. The
// floor has each kind of thing a save can end.

// floor: keyless streams on tally and names, and a Public panel with a tablet key. Ana
// (persistent) has a session and a Session panel key. Bo (not persistent) has a tab sign-in and
// a Session panel key. Cy has a personal key. Di (no PIN) has a Session panel key. Ada, the
// Administrator, is rechecked on her own connection.
type floor struct {
	t     *testing.T
	a     *App
	admin *wsPeer
}

func newFloor(t *testing.T) *floor {
	t.Helper()
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.API.GuestEnabled = true
		c.GuestAPIScopes = []string{config.ScopeNames, config.ScopeTally, config.ScopeStatus}
		cy := mkUser(t, "Cy", "4321")
		cy.Role = config.RoleTrusted
		c.Users = append(c.Users, cy, config.User{Name: "Di"})
		for i := range c.Users {
			if c.Users[i].Name == "Ana" {
				c.Users[i].Persistent = true
			}
		}
	})
	f := &floor{t: t, a: a}
	srv := strmServer(t, a)
	for _, topic := range []string{"tally", "names"} {
		s := strmDial(t, srv, "", topic)
		s.next() // hello
		s.next() // snapshot
	}
	cyKey := userKey(t, a, "Cy")
	lobby := makePanel(t, a, cyKey, publicPanel+`}`)
	if code := apiSend(a, "POST", "/api/v1/panels/"+lobby+"/instances", publicKeyOf(t, a, lobby), `{"device":"Lobby tablet"}`).Code; code != 200 {
		t.Fatalf("tablet keepalive: %d", code)
	}

	ana, anext := signedIn(t, a, "10.0.0.1", "1234")
	if a.issueToken(ana, false) == "" {
		t.Fatal("Ana has no session")
	}
	sessionKeyOf(t, a, ana, anext)
	bo, bnext := signedIn(t, a, "10.0.0.2", "567890")
	sessionKeyOf(t, a, bo, bnext)
	di, dnext := signInNoPin(t, a, "10.0.0.4", "Di")
	sessionKeyOf(t, a, di, dnext)

	f.admin = dialPeer(t, a)
	f.admin.signInRechecked("Ada", "2468")
	return f
}

// live counts what runs on the floor now.
func (f *floor) live() map[string]int {
	a := f.a
	cfg := a.r().cfg
	out := map[string]int{}
	a.mu.Lock()
	for st := range a.streams {
		if st.ended == "" && st.keyID == "" {
			out["keyless streams"]++
		}
	}
	a.mu.Unlock()
	a.public.mu.Lock()
	out["tablet keys"] = len(a.public.recs)
	a.public.mu.Unlock()
	a.tokens.mu.Lock()
	for _, r := range a.tokens.toks {
		if u, ok := valid(r, cfg); ok {
			out["sessions "+u.Name]++
		}
	}
	a.tokens.mu.Unlock()
	a.keys.mu.Lock()
	for _, r := range a.keys.recs {
		if r.Setup || r.Owner.Kind != actorUser {
			continue
		}
		u, _ := cfg.FindUserByID(r.Owner.User)
		if r.Sess != "" {
			out["session keys "+u.Name]++
		} else {
			out["keys "+u.Name]++
		}
	}
	a.keys.mu.Unlock()
	maps.DeleteFunc(out, func(_ string, n int) bool { return n == 0 })
	return out
}

// wantEnded: between before and now exactly the counts in ended went down, to the value given.
func (f *floor) wantEnded(before map[string]int, ended map[string]int) {
	f.t.Helper()
	now := f.live()
	want := maps.Clone(before)
	maps.Copy(want, ended)
	for k, v := range want {
		if v == 0 {
			delete(want, k)
		}
	}
	if !maps.Equal(now, want) {
		f.t.Fatalf("after the save\n have %v\n want %v", now, want)
	}
}

func (f *floor) fields(set map[string]any) map[string]fieldEdit {
	live := f.a.r().cfg
	out := map[string]fieldEdit{}
	for k, v := range set {
		out[k] = fieldEdit{Value: jsonOf(v), Version: fieldVersion(live, k)}
	}
	return out
}

func interruptionsOf(t *testing.T, r map[string]any) []map[string]any {
	t.Helper()
	l, _ := r["interruptions"].([]any)
	out := []map[string]any{}
	for _, x := range l {
		out = append(out, x.(map[string]any))
	}
	return out
}

// review asks for the Interruptions of set, then saves set; it returns both lists.
func (f *floor) reviewAndSave(set map[string]any) (review, saved []map[string]any) {
	f.t.Helper()
	r := f.admin.ask("settingsReview", 1, map[string]any{"fields": f.fields(set)})
	if r["ok"] != true {
		f.t.Fatalf("review: %v", r)
	}
	review = interruptionsOf(f.t, r)
	s := f.admin.ask("settingsSave", 2, map[string]any{"fields": f.fields(set)})
	for k := range set {
		wantSaved(f.t, s, k)
	}
	return review, interruptionsOf(f.t, s)
}

func count(l []map[string]any, what string) int {
	n := 0
	for _, x := range l {
		v, _ := x[what].(float64)
		n += int(v)
	}
	return n
}

func texts(l []map[string]any) string {
	var out []string
	for _, x := range l {
		out = append(out, x["text"].(string))
	}
	return strings.Join(out, " | ")
}

func wantCounts(t *testing.T, what string, l []map[string]any, want map[string]int) {
	t.Helper()
	for _, k := range []string{"panels", "streams", "sessions", "keys", "browsers"} {
		if got := count(l, k); got != want[k] {
			t.Fatalf("%s: %s %d, want %d (%s)", what, k, got, want[k], texts(l))
		}
	}
}

func TestInterruptionsKeylessAPIOffEndsEveryKeylessStream(t *testing.T) {
	f := newFloor(t)
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{"api": map[string]any{"guestEnabled": false}})
	want := map[string]int{"streams": 2}
	wantCounts(t, "review", rv, want)
	wantCounts(t, "save", sv, want)
	f.wantEnded(before, map[string]int{"keyless streams": 0})
}

func TestInterruptionsKeylessScopeRemovedEndsOnlyItsStreams(t *testing.T) {
	f := newFloor(t)
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{"guestApiScopes": []string{config.ScopeNames, config.ScopeStatus}})
	wantCounts(t, "review", rv, map[string]int{"streams": 1})
	wantCounts(t, "save", sv, map[string]int{"streams": 1})
	if !strings.Contains(texts(sv), "1 narrows") {
		t.Fatalf("the names stream is not said to narrow: %s", texts(sv))
	}
	f.wantEnded(before, map[string]int{"keyless streams": 1})
}

func TestInterruptionsPublicPanelsOffEndsEveryPublicKey(t *testing.T) {
	f := newFloor(t)
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{"publicPanels": false})
	want := map[string]int{"panels": 1}
	wantCounts(t, "review", rv, want)
	wantCounts(t, "save", sv, want)
	if s := texts(sv); !strings.Contains(s, "Lobby") || !strings.Contains(s, "1 tablet") || !strings.Contains(s, "comes back by itself") {
		t.Fatalf("Public panel line: %s", s)
	}
	f.wantEnded(before, map[string]int{"tablet keys": 0})
}

func TestInterruptionsPersistentUntickedEndsThatUsersSessions(t *testing.T) {
	f := newFloor(t)
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{"user/" + idOf(f.a, "Ana") + "/persistent": false})
	want := map[string]int{"sessions": 1}
	wantCounts(t, "review", rv, want)
	wantCounts(t, "save", sv, want)
	if !strings.Contains(texts(sv), "Ana") {
		t.Fatalf("the line does not name Ana: %s", texts(sv))
	}
	// The Session panel key rides the session, so it ends with it.
	f.wantEnded(before, map[string]int{"sessions Ana": 0, "session keys Ana": 0})
}

func TestInterruptionsPersistentTickedEndsNothing(t *testing.T) {
	f := newFloor(t)
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{"user/" + idOf(f.a, "Bo") + "/persistent": true})
	if len(rv) != 0 || len(sv) != 0 {
		t.Fatalf("ticking Never sign out lists %s / %s", texts(rv), texts(sv))
	}
	// Bo's tab sign-in becomes his persistent session, and his Session panel key stays on it.
	f.wantEnded(before, nil)
}

func TestInterruptionsPinResetEndsThatUsersSessionsAndKeys(t *testing.T) {
	f := newFloor(t)
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{"user/" + idOf(f.a, "Cy") + "/pin": "8642"})
	// Cy's personal key, and the Public panel's maker key from makePanel.
	want := map[string]int{"keys": before["keys Cy"]}
	wantCounts(t, "review", rv, want)
	wantCounts(t, "save", sv, want)
	f.wantEnded(before, map[string]int{"keys Cy": 0})

	before = f.live()
	_, sv = f.reviewAndSave(map[string]any{"user/" + idOf(f.a, "Ana") + "/pin": "9753"})
	wantCounts(t, "save Ana", sv, map[string]int{"sessions": 1, "keys": 1})
	f.wantEnded(before, map[string]int{"sessions Ana": 0, "session keys Ana": 0})
}

func TestInterruptionsPanelLockOnEndsSessionKeysOfUsersWithoutAPin(t *testing.T) {
	f := newFloor(t)
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{"locks": map[string]any{"panel": true}})
	want := map[string]int{"panels": 1}
	wantCounts(t, "review", rv, want)
	wantCounts(t, "save", sv, want)
	if !strings.Contains(texts(sv), "Di") {
		t.Fatalf("the users without a PIN it shuts out are not named: %s", texts(sv))
	}
	f.wantEnded(before, map[string]int{"session keys Di": 0})
}

// Main and Log locks, Guest role, auto-lock and Wait after wrong PINs end nothing. They are still
// listed: browsers that drop to a lock screen, a Guest role demotion and an auto-lock reset.
func TestInterruptionsLocksGuestRoleAndAutoLockEndNothing(t *testing.T) {
	f := newFloor(t)
	cfg := f.a.r().cfg
	cfg.AutoLockMinutes = 15
	cfg.GuestRole = config.RoleTrusted
	if err := f.a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	before := f.live()
	rv, sv := f.reviewAndSave(map[string]any{
		"locks":           map[string]any{"main": true, "log": true},
		"guestRole":       config.RoleStandard,
		"autoLockMinutes": 0,
		"pinWaits":        !cfg.PinWaits,
	})
	// Di's browser is on the main page; the Main page lock puts it on a lock screen.
	want := map[string]int{"browsers": 1}
	wantCounts(t, "review", rv, want)
	wantCounts(t, "save", sv, want)
	s := texts(sv)
	for _, w := range []string{"Di", "Trusted user", "Standard user", "turns auto-lock off (was 15 min)"} {
		if !strings.Contains(s, w) {
			t.Fatalf("no %q in %s", w, s)
		}
	}
	for _, x := range sv {
		if x["ends"] == true {
			t.Fatalf("a line says it ends something: %v", x)
		}
	}
	f.wantEnded(before, nil)
}

// The record of the save keeps the Interruptions.
func TestInterruptionsGoInTheSettingsSavesRow(t *testing.T) {
	f := newFloor(t)
	defer startLog(t, f.a)()
	_, sv := f.reviewAndSave(map[string]any{"publicPanels": false})
	rows := waitSaves(t, f.a, 1)
	got := rows[0].Save.Interruptions
	var want []string
	for _, x := range sv {
		want = append(want, x["text"].(string))
	}
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Fatalf("row %v, save %v", got, want)
	}
}

// A Role that cannot save an Access field gets no Interruptions for it: they name users, Public
// panels and sessions.
func TestInterruptionsReviewShowsNothingARoleCannotSave(t *testing.T) {
	f := newFloor(t)
	bo := dialPeer(t, f.a)
	bo.signIn("Bo", "567890")
	r := bo.ask("settingsReview", 1, map[string]any{"fields": f.fields(map[string]any{
		"publicPanels": false, "api": map[string]any{"guestEnabled": false},
	})})
	if l := interruptionsOf(t, r); len(l) != 0 {
		t.Fatalf("a Standard user's review lists %s", texts(l))
	}
}

// Readers below Administrator see a row's Interruptions only as a count.
func TestInterruptionsInTheLogAreForAdministrators(t *testing.T) {
	f := newFloor(t)
	defer startLog(t, f.a)()
	f.reviewAndSave(map[string]any{"publicPanels": false})
	waitSaves(t, f.a, 1)
	read := func(name, pin string) Sent {
		p := dialPeer(t, f.a)
		p.signIn(name, pin)
		p.send(map[string]any{"t": "log", "before": -1, "n": 100})
		var page sentPageMsg
		if err := json.Unmarshal(jsonOf(p.until("logPage")), &page); err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Entries {
			if e.Kind == kindSettings {
				return e
			}
		}
		t.Fatalf("no Settings saves row for %s", name)
		return Sent{}
	}
	if row := read("Ada", "2468"); !strings.Contains(strings.Join(row.Save.Interruptions, " "), "Lobby") {
		t.Fatalf("Administrator's row: %+v", row.Save)
	}
	row := read("Bo", "567890")
	if s := strings.Join(row.Save.Interruptions, " "); s != "" || row.Save.HiddenInterruptions != 1 {
		t.Fatalf("Standard user's row: %q, %d", s, row.Save.HiddenInterruptions)
	}
}
