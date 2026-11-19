package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func init() {
	const read = config.ScopePanels
	registerAPI(
		apiRoute{"POST", apiBase + "/panels/{id}/instances", read, (*App).apiPanelKeepalive},
		apiRoute{"GET", apiBase + "/panels/{id}/instances/stream", read, (*App).apiPanelInstanceStream},
		apiRoute{"POST", apiBase + "/panels/{id}/watch", read, (*App).apiPanelWatch},
		apiRoute{"DELETE", apiBase + "/panels/{id}/watch", read, (*App).apiPanelUnwatch},
	)
}

// apiPanelInstanceStream: GET ?device=Lobby holds a connection open so Sode sees a closed page at
// once instead of after the keepalive miss. It writes a line every monBeat; the page reads and
// ignores it. The keepalive stays as the fallback for a page whose stream cannot stay open.
func (a *App) apiPanelInstanceStream(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	device, ok := deviceOf(w, r.URL.Query().Get("device"), p, cl)
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	done, out, ev, err := a.mon.openStream(p.ID, device, publicKeyID(cl), streamWhoOf(cl), cancel)
	if err != nil {
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	defer done()
	if ev != nil {
		a.monNotify(*ev)
	}
	rc := http.NewResponseController(w)
	// Each line sets a deadline that has passed by the next beat; the closing chunk needs a live one.
	defer func() { _ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second)) }()
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	line := func() bool {
		_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "{\"online\":true,\"graceSeconds\":%d}\n", int(monGrace.Seconds())); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !line() {
		return
	}
	t := time.NewTicker(monBeat)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !a.streamKeyLive(cl) || !line() {
				return
			}
		case l := <-out:
			// An Assign sends a long-lived key: never down a stream whose own key has ended since it opened.
			if !a.streamKeyLive(cl) {
				l.got <- false
				return
			}
			_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := w.Write(append(l.b, '\n'))
			ok := err == nil && rc.Flush() == nil
			l.got <- ok
			if ok {
				// The page leaves for the target panel: a deliberate move is no lost alarm here.
				a.mon.forgetDevice(p.ID, device)
			}
			return
		}
	}
}

// streamKeyLive: the key a held stream opened with still authenticates. endStreams reaches only the
// WebSocket streams, so a held instance stream checks for itself (expiry, revoke, leaving Public, its
// session or client gone).
func (a *App) streamKeyLive(cl *apiCtx) bool {
	if cl.Key == nil {
		return true
	}
	id := cl.Key.ID
	if !a.keys.alive(id) && !a.public.alive(id) {
		return false
	}
	cfg := a.r().cfg
	if sess := a.keys.sessionOf(id); sess != "" && !a.tokens.live(sess, cl.Key.Owner.User, cfg) {
		return false
	}
	return a.resolveKey(cfg, *cl.Key, cl.IP) != nil
}

func publicKeyID(cl *apiCtx) string {
	if cl.Key != nil && cl.Key.public {
		return cl.Key.ID
	}
	return ""
}

func streamWhoOf(cl *apiCtx) streamWho {
	switch cl.Actor.Kind {
	case actorClient:
		return streamWho{addr: cl.IP, signIn: "client"}
	case actorUser:
		return streamWho{addr: cl.IP, signIn: "user", user: cl.Actor.User}
	}
	return streamWho{addr: cl.IP, signIn: "none"}
}

// deviceOf names the instance, or answers 400 and false. With askInstanceName on, the page always
// names itself. With it off, an unnamed instance goes by its address, so monitoring, watches and
// notices still tell it apart.
func deviceOf(w http.ResponseWriter, s string, p Panel, cl *apiCtx) (string, bool) {
	if d := cleanDevice(s); d != "" {
		return d, true
	}
	if asksName(p) {
		apiFail(w, http.StatusBadRequest, "bad_request", "a device name is needed so the instances of one panel can be told apart")
		return "", false
	}
	return cleanDevice(cl.IP), true
}

// apiPanelKeepalive: {"device": "Lobby"}. Anyone who can use the panel counts, so a tablet signed
// in as a shared user is an instance too.
func (a *App) apiPanelKeepalive(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	var in struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"device":"<a name for this instance>"}`)
		return
	}
	device, ok := deviceOf(w, in.Device, p, cl)
	if !ok {
		return
	}
	ev, err := a.mon.beat(p.ID, device, publicKeyID(cl))
	if err != nil {
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	if ev != nil {
		a.monNotify(*ev)
	}
	apiJSON(w, map[string]any{"online": true, "keepaliveSeconds": int(monKeepalive.Seconds()), "missSeconds": int(monMiss.Seconds()), "graceSeconds": int(monGrace.Seconds())})
}

func (a *App) watchFor(w http.ResponseWriter, r *http.Request, cl *apiCtx) (Panel, bool) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "watching belongs to a user: use a user's key")
		return Panel{}, false
	}
	p, ok := a.panelFor(w, r, cl, true)
	return p, ok
}

func (a *App) apiPanelWatch(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	a.setWatch(w, r, cl, true)
}

func (a *App) apiPanelUnwatch(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	a.setWatch(w, r, cl, false)
}

func (a *App) setWatch(w http.ResponseWriter, r *http.Request, cl *apiCtx, on bool) {
	p, ok := a.watchFor(w, r, cl)
	if !ok {
		return
	}
	inst := ""
	if q, ok := r.URL.Query()["instance"]; ok {
		if inst = cleanDevice(q[0]); inst == "" {
			apiFail(w, http.StatusBadRequest, "bad_request", "instance is a device name: ?instance=<instance name>")
			return
		}
	}
	if err := a.mon.setWatchInst(cl.Actor.User, p.ID, inst, on); err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the watch list could not be saved: "+err.Error())
		return
	}
	a.mu.Lock()
	a.pushOfflineAllMu()
	a.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
