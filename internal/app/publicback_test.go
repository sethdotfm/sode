package app

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// A Public panel page comes back by itself: a public line on its keyless stream, then one ask.

func wantLine(t *testing.T, ch <-chan map[string]any, typ string) map[string]any {
	t.Helper()
	select {
	case m := <-ch:
		if m["t"] != typ {
			t.Fatalf("heard %v, want t %q", m, typ)
		}
		return m
	case <-time.After(5 * time.Second):
		t.Fatalf("no %s line", typ)
	}
	return nil
}

func TestPublicPanelsBackOnTellsItsKeylessPagesAndTheyGetAKey(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Lobby","acts":"public","slug":"lobby"}`)
	other := makePanel(t, a, cy, `{"name":"Other","acts":"public"}`)
	setPublicPanels(t, a, false)
	byID, _ := keylessHold(t, a, id, "?ended=1")
	bySlug, _ := keylessHold(t, a, "lobby", "?instance=Wall")
	elsewhere, _ := keylessHold(t, a, "nolobby", "")
	keylessRow(t, a, id, cy, "Wall")
	setPublicPanels(t, a, true)
	wantLine(t, byID, "public")
	wantLine(t, bySlug, "public")
	silent(t, elsewhere)
	publicKeyOf(t, a, id)
	publicKeyOf(t, a, other)
	// The line leaves the stream open, so the page still hears an Assign or the next line.
	setPublicPanels(t, a, false)
	setPublicPanels(t, a, true)
	wantLine(t, byID, "public")
}

func TestAPanelBackToPublicTellsItsKeylessPagesAndLeavingDoesNot(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Lobby"}`)
	page, _ := keylessHold(t, a, id, "?instance=Hall")
	keylessRow(t, a, id, cy, "Hall")
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, `{"name":"Lobby"}`); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	silent(t, page)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, publicPanel+`}`); rec.Code != 200 {
		t.Fatalf("to Public: %d %s", rec.Code, rec.Body.String())
	}
	wantLine(t, page, "public")
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, `{"name":"Lobby","acts":"session"}`); rec.Code != 200 {
		t.Fatalf("to Session: %d %s", rec.Code, rec.Body.String())
	}
	silent(t, page)
}

// The page marks the ask that follows a public line, so the Log has one row per page that came back. It names the
// address, never a user, and a refused ask writes nothing.
func TestAPublicPageThatComesBackWritesALogRowWithNoUser(t *testing.T) {
	a, _ := clientApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Lobby","acts":"public","slug":"lobby"}`)
	setPublicPanels(t, a, false)
	wantNotPublic(t, askBack(a, id))
	setPublicPanels(t, a, true)
	if rec := askPublic(a, apiPeer, id); rec.Code != 201 {
		t.Fatalf("plain ask: %d %s", rec.Code, rec.Body.String())
	}
	if rec := askBack(a, id); rec.Code != 201 {
		t.Fatalf("ask: %d %s", rec.Code, rec.Body.String())
	}
	row := lastRow(t, a, "publicBack")
	if strings.Contains(row.By, "Cy") || !strings.Contains(row.By, "192.0.2.7") || !strings.Contains(row.Note, "lobby") {
		t.Fatalf("row %+v", row)
	}
	if n := countRows(t, a, "publicBack"); n != 1 {
		t.Fatalf("%d publicBack rows", n)
	}
}

func askBack(a *App, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/panels/"+id+"/public?back=1", nil)
	r.RemoteAddr = apiPeer
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func countRows(t *testing.T, a *App, why string) int {
	t.Helper()
	n := 0
	for _, r := range logged(t, a) {
		if r.Why == why {
			n++
		}
	}
	return n
}
