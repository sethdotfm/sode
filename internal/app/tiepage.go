package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// The main page's route across routers (docs/tielines-design.md, "The Tie line picker"). The page sends the message
// tieTake with the routes of POST /takes. Sode runs that request for the page's own caller and gives back its status
// and body as tieTakeResult. Thus the page gets the plan (409 tie_line_needed or no_path) of an API client. It also
// gets the same refusals, the same stale check and the same take. The page never computes paths.
//
// The take is a take of the page, not of an API caller. apiCtx.page names the connection, so the in-process caller
// (client.forPage) has its who, its Rights and its step-in. Its entries carry no API actor, and its Communication Log rows
// are command rows (takeEvent), as for a plain take from the page.
//
// The message tiePlan asks for the plan only: the same 409 body, with no take and no Communication Log row. The open
// picker asks again each time the page hears a change that can change the plan: a tally, a Protect, a router status
// or the tie lines (web/tiepicker.js). Thus the server pushes each change, and an open picker holds nothing on the
// server.

// takeEvent writes the Communication Log row of an in-process take: a command row of the page for a page's tieTake,
// else an api row. A refusal before the plan is a refused take, as a page take's refusal is.
func (a *App) takeEvent(cl *apiCtx, why, result, note string, err error) {
	if cl.page == nil {
		a.apiEvent(cl.Actor, why, result, note, err)
		return
	}
	if why == "refused" {
		why = "take"
	}
	a.command(cl.page, why, result, note, err)
}

type tieTakeMsg struct {
	T      string          `json:"t"` // tieTakeResult or tiePlanResult
	ID     int             `json:"id"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// answerWriter keeps the answer of an in-process API call.
type answerWriter struct {
	h      http.Header
	status int
	body   bytes.Buffer
}

func (w *answerWriter) Header() http.Header { return w.h }

func (w *answerWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *answerWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.body.Write(b)
}

func (a *App) serveTieTake(c *client, m *inMsg) { a.servePageTakes(c, m, false) }

func (a *App) serveTiePlan(c *client, m *inMsg) { a.servePageTakes(c, m, true) }

func (a *App) servePageTakes(c *client, m *inMsg, planOnly bool) {
	act := a.caller(c)
	cl := &apiCtx{Actor: act.Actor, Rights: act.rights, IP: c.ip, page: c, planOnly: planOnly}
	body := append(append([]byte(`{"routes":`), m.Routes...), '}')
	if len(m.Routes) == 0 {
		body = []byte(`{"routes":[]}`)
	}
	t := "tieTakeResult"
	if planOnly {
		t = "tiePlanResult"
	}
	go func() {
		if !planOnly {
			defer m.finish()
		}
		w := &answerWriter{h: http.Header{}}
		r, err := http.NewRequestWithContext(context.Background(), http.MethodPost, apiBase+"/takes", bytes.NewReader(body))
		if err != nil {
			a.pageTakesRefused(c, m, t, err)
			return
		}
		a.apiTakeRoutes(w, r, cl)
		a.reply(c, tieTakeMsg{T: t, ID: m.ID, Status: w.status, Body: w.body.Bytes()})
	}()
}

// tieTakeRefused answers a tieTake that never ran, in the shape of an API problem.
func (a *App) tieTakeRefused(c *client, m *inMsg, err error) {
	a.pageTakesRefused(c, m, "tieTakeResult", err)
}

func (a *App) pageTakesRefused(c *client, m *inMsg, t string, err error) {
	b, _ := json.Marshal(apiErrBody{Error: apiErr{Code: "refused", Message: err.Error()}})
	a.reply(c, tieTakeMsg{T: t, ID: m.ID, Status: http.StatusForbidden, Body: b})
}
