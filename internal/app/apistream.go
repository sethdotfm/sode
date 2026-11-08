package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
)

// The stream: GET /api/v1/stream upgrades to a WebSocket that only sends. The caller names
// its topics in ?topics=a,b at connect; every frame is {"t","topic"?,"data"?}. Keep docs in
// step: openapi.yaml.
//
//	hello          {v, topics, numberOffset}, always first
//	snapshot       one per names, tally and status topic, topic set, data as the REST read
//	names          full names, filtered
//	tally          {routes:[{dst,level,src}]} changed routes only
//	status         {router, routerUp, error}, plus address and err for a caller with the log scope
//	entry          a new History entry (topic history)
//	entryUpdate    a changed entry, such as a Late Result (topics history and results, sent once)
//	historyCleared (topic history)
//	ping           heartbeat, ignore
//
// Visibility (apiView) filters every frame, so a hidden caller never sees an out-of-rights
// route or entry. A tally change whose Source is hidden is left out, as the REST read
// leaves it out, so a client never sees -1 for a route that has a hidden source. Frames go out in the order the server saw the changes.

// A config commit re-resolves each open stream (refreshStreams): a rights change filters the
// next frame, and losing the API, Guest, a topic scope or the log scope ends the stream.
// Liveness is the text ping frame alone. Each key and each address may hold 8 streams; more
// get 429.

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/stream", "", (*App).apiStreamServe})
}

const apiStreamVersion = 1

// Open streams allowed at once. Vars so tests can isolate one cap.
var (
	apiStreamMaxPerKey  = 8
	apiStreamMaxPerAddr = 8
)

// streamTopics maps a topic to the scope that allows it. results (entry updates only) rides
// the history scope.
var streamTopics = map[string]string{
	"names": config.ScopeNames, "tally": config.ScopeTally, "status": config.ScopeStatus,
	"history": config.ScopeHistory, "results": config.ScopeHistory,
}

// streamOrder fixes the order of snapshots.
var streamOrder = []string{"names", "tally", "status", "history", "results"}

type streamMsg struct {
	T     string `json:"t"`
	Topic string `json:"topic,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// apiStream is one attached caller. Guarded by App.mu like a browser client's send channel.
type apiStream struct {
	view   apiView
	topics map[string]bool
	send   chan []byte
	detail bool               // the caller has the log scope: status frames carry address and err
	keyID  string             // "" for a keyless Guest call
	ip     string             // the caller's address, for the per-address cap
	owner  keyOwner           // who the key acts as, re-read on every config commit
	cancel context.CancelFunc // ends the connection; safe from any goroutine
	ended  string             // why a key lifecycle path cut it, set under App.mu
}

func parseStreamTopics(q string) ([]string, error) {
	var out []string
	for _, t := range strings.Split(q, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := streamTopics[t]; !ok {
			return nil, fmt.Errorf("unknown topic %q; topics are %s", t, strings.Join(streamOrder, ", "))
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("choose topics with ?topics=, from %s", strings.Join(streamOrder, ", "))
	}
	return out, nil
}

func (a *App) apiStreamServe(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	topics, err := parseStreamTopics(r.URL.Query().Get("topics"))
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	for _, t := range topics {
		if sc := streamTopics[t]; !cl.has(sc) {
			msg := fmt.Sprintf("topic %q needs the %q scope, which this key lacks", t, sc)
			a.apiEvent(cl.Actor, "refused", "refused", "stream "+strings.Join(topics, ","), fmt.Errorf("%s", msg))
			apiFail(w, http.StatusForbidden, "scope", msg)
			return
		}
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	st := &apiStream{view: cl.view(), detail: cl.has(config.ScopeLog), topics: map[string]bool{}, send: make(chan []byte, 256), cancel: cancel, ip: cl.IP}
	if cl.Key != nil {
		st.keyID = cl.Key.ID
		st.owner = cl.Key.Owner
	}
	for _, t := range topics {
		st.topics[t] = true
	}

	// Register (after the caps) and queue the snapshots under one lock so no change arrives
	// ahead of them. Registering before the upgrade keeps the caps exact under a burst.
	a.mu.Lock()
	if a.streams == nil {
		a.streams = map[*apiStream]struct{}{}
	}
	perKey, perAddr := 0, 0
	for o := range a.streams {
		if st.keyID != "" && o.keyID == st.keyID {
			perKey++
		}
		if o.ip == st.ip {
			perAddr++
		}
	}
	if perKey >= apiStreamMaxPerKey || perAddr >= apiStreamMaxPerAddr {
		a.mu.Unlock()
		msg := fmt.Sprintf("too many open streams: %d for a key and %d for an address at most", apiStreamMaxPerKey, apiStreamMaxPerAddr)
		a.apiEvent(cl.Actor, "refused", "refused", "stream "+strings.Join(topics, ","), fmt.Errorf("%s", msg))
		apiFail(w, http.StatusTooManyRequests, "too_many_streams", msg)
		return
	}
	a.streams[st] = struct{}{}
	a.streamQueue(st, streamMsg{T: "hello", Data: map[string]any{
		"v": apiStreamVersion, "topics": topics, "numberOffset": a.r().cfg.NumberOffset}})
	for _, t := range streamOrder {
		if !st.topics[t] {
			continue
		}
		switch t {
		case "names":
			a.streamQueue(st, streamMsg{T: "snapshot", Topic: t, Data: a.namesSnapshot(st.view)})
		case "tally":
			a.streamQueue(st, streamMsg{T: "snapshot", Topic: t, Data: a.tallySnapshot(st.view)})
		case "status":
			a.streamQueue(st, streamMsg{T: "snapshot", Topic: t, Data: linkStatus(a.status, st.detail)})
		}
	}
	a.mu.Unlock()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		a.dropStream(st)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 10) // the stream takes no input

	// A key ended between auth and registering is not seen by endStreams.
	if st.keyID != "" && !a.keys.alive(st.keyID) {
		a.endStreams([]apiKey{*cl.Key}, "key ended")
	}

	connected := time.Now()
	a.apiEvent(cl.Actor, "streamConnect", "ok", "topics "+strings.Join(topics, ", "), nil)
	var gone error
	defer func() {
		a.dropStream(st)
		a.mu.Lock()
		ended := st.ended
		a.mu.Unlock()
		var cerr error
		switch websocket.CloseStatus(gone) {
		case websocket.StatusNormalClosure, websocket.StatusGoingAway:
		default:
			if gone != nil && !strings.Contains(gone.Error(), context.Canceled.Error()) {
				cerr = gone
			}
		}
		note := "connected " + time.Since(connected).Round(time.Second).String()
		if ended != "" {
			note += "; closed: " + ended
		}
		a.apiEvent(cl.Actor, "streamDisconnect", "ok", note, cerr)
	}()

	go func() {
		defer cancel()
		beat := time.NewTicker(wsHeartbeat)
		defer beat.Stop()
		write := func(b []byte) error {
			wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
			defer wcancel()
			return conn.Write(wctx, websocket.MessageText, b)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case b, ok := <-st.send:
				if !ok {
					conn.Close(websocket.StatusPolicyViolation, "too slow")
					return
				}
				if write(b) != nil {
					return
				}
			case <-beat.C:
				if write(pingMsg) != nil {
					return
				}
			}
		}
	}()

	for {
		if _, _, err := conn.Read(ctx); err != nil {
			gone = err
			return
		}
	}
}

// dropStream unregisters st and closes its queue once. Takes mu.
func (a *App) dropStream(st *apiStream) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.streams[st]; ok {
		delete(a.streams, st)
		close(st.send)
	}
}

// endStreams closes the open streams of keys that just ended, so a stream cannot outlive its
// key. Each closes through its own disconnect row, which carries why. Takes mu.
func (a *App) endStreams(keys []apiKey, why string) {
	if len(keys) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for st := range a.streams {
		if st.keyID == "" || st.ended != "" {
			continue
		}
		for _, k := range keys {
			if k.ID == st.keyID {
				st.ended = why
				st.cancel()
				break
			}
		}
	}
}

// refreshStreams applies a config commit to the open streams as the next REST call would see
// it: a changed view takes effect at once, and a stream ends when its caller lost the API,
// Guest, a scope it uses or its log detail. Caller holds mu.
func (a *App) refreshStreams(cfg config.Config) {
	for st := range a.streams {
		if st.ended != "" {
			continue
		}
		why := ""
		cl := &apiCtx{Scopes: cfg.GuestAPIScopes, Rights: cfg.GuestRights}
		switch {
		case !cfg.API.Enabled:
			why = "the API is off"
		case st.keyID == "" && !cfg.API.GuestEnabled:
			why = "the Guest API is off"
		case st.keyID != "":
			if cl = a.resolveKey(cfg, apiKey{Owner: st.owner}, ""); cl == nil {
				continue // pruneKeys ends it
			}
		}
		if why == "" {
			lost := st.detail && !cl.has(config.ScopeLog)
			for t := range st.topics {
				lost = lost || !cl.has(streamTopics[t])
			}
			if lost {
				why = "its scopes changed"
			}
		}
		if why != "" {
			st.ended = why
			st.cancel()
			continue
		}
		st.view = cl.view()
	}
}

// streamQueue drops the stream if it can't keep up. Caller holds mu.
func (a *App) streamQueue(st *apiStream, m streamMsg) {
	b, err := json.Marshal(m)
	if err != nil {
		log.Printf("stream: marshal: %v", err)
		return
	}
	if _, ok := a.streams[st]; !ok {
		return
	}
	select {
	case st.send <- b:
	default:
		log.Printf("api stream: too slow, dropping")
		if st.ended == "" {
			st.ended = "too slow"
		}
		delete(a.streams, st)
		close(st.send)
	}
}

// streamFan turns what broadcast sends to browsers into frames for each stream that chose
// the topic. Caller holds mu.
func (a *App) streamFan(v any) {
	if len(a.streams) == 0 {
		return
	}
	for st := range a.streams {
		switch m := v.(type) {
		case deltaMsg:
			if m.T != "tally" || !st.topics["tally"] {
				continue
			}
			rows := []apiRouteRow{}
			for _, c := range m.U {
				if !st.view.Destination(c.Dst) {
					continue
				}
				if !st.view.Route(c.Dst, c.Level, c.Src) {
					continue
				}
				rows = append(rows, apiRouteRow{c.Dst, c.Level, c.Src})
			}
			if len(rows) > 0 {
				a.streamQueue(st, streamMsg{T: "tally", Data: map[string]any{"routes": rows}})
			}
		case namesMsg:
			if st.topics["names"] {
				a.streamQueue(st, streamMsg{T: "names", Data: a.namesSnapshot(st.view)})
			}
		case statusMsg:
			if st.topics["status"] {
				a.streamQueue(st, streamMsg{T: "status", Data: linkStatus(m, st.detail)})
			}
		case recentMsg:
			if m.E == nil {
				continue
			}
			wantNew, wantUpd := st.topics["history"], st.topics["history"] || st.topics["results"]
			if (!m.Update && !wantNew) || (m.Update && !wantUpd) {
				continue
			}
			e, ok := st.view.Entry(*m.E)
			if !ok {
				continue
			}
			t := "entry"
			if m.Update {
				t = "entryUpdate"
			}
			a.streamQueue(st, streamMsg{T: t, Data: e})
		case simpleMsg:
			if m.T == "historyCleared" && st.topics["history"] {
				a.streamQueue(st, streamMsg{T: "historyCleared"})
			}
		}
	}
}
