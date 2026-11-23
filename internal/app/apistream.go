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

// GET /api/v1/stream upgrades to a WebSocket that only sends. The frames and topics are in docs/api.md and
// openapi.yaml; keep them in step. Visibility (apiView) filters every frame, and a tally change with a hidden Source
// is left out,
// as in the REST read. Frames go out in the order the server saw the changes.

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/stream", "", (*App).apiStreamServe})
}

const apiStreamVersion = 1

// Vars so a test can isolate one cap.
var (
	apiStreamMaxPerKey  = 8
	apiStreamMaxPerAddr = 8

	apiStreamMaxPerPanelKey  = 64
	apiStreamMaxPanelPerAddr = 4 * apiStreamMaxPerPanelKey
)

// The scope that allows each topic. results (entry updates only) rides the history scope.
var streamTopics = map[string]string{
	"names": config.ScopeNames, "tally": config.ScopeTally, "status": config.ScopeStatus,
	"history": config.ScopeHistory, "results": config.ScopeHistory,
}

var streamOrder = []string{"names", "tally", "status", "history", "results"}

type streamMsg struct {
	T     string `json:"t"`
	Topic string `json:"topic,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// Guarded by App.mu.
type apiStream struct {
	unit   *routerUnit // the router of the stream; names, tally and status frames come only from it
	view   apiView
	topics map[string]bool
	send   chan []byte
	detail bool   // the caller has the log scope: status frames carry address and err
	panel  bool   // the key is a panel's own: its instances share it, so the caps differ
	keyID  string // "" for a keyless Guest call
	ip     string
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
	st := &apiStream{unit: a.apiUnit(r), view: cl.view(), detail: cl.has(config.ScopeLog), topics: map[string]bool{}, send: make(chan []byte, 256), cancel: cancel, ip: cl.IP}
	if cl.Key != nil {
		st.keyID = cl.Key.ID
		st.owner = cl.Key.Owner
	}
	for _, t := range topics {
		st.topics[t] = true
	}

	// One lock covers the register (after the caps) and the snapshots, so no change arrives before them.
	// The register comes before the upgrade, which keeps the caps exact in a burst.
	a.mu.Lock()
	if a.streams == nil {
		a.streams = map[*apiStream]struct{}{}
	}
	st.panel = cl.Panel != ""
	perKey, perAddr := 0, 0
	for o := range a.streams {
		if st.keyID != "" && o.keyID == st.keyID {
			perKey++
		}
		if o.ip == st.ip && o.panel == st.panel {
			perAddr++
		}
	}
	// The instances of a panel share one key and often one address. So their streams have higher caps, per key and
	// per address, apart from other streams.
	keyMax, addrMax := apiStreamMaxPerKey, apiStreamMaxPerAddr
	if st.panel {
		addrMax = apiStreamMaxPanelPerAddr
		if publicKeyID(cl) == "" { // a Public page holds a key of its own
			keyMax = apiStreamMaxPerPanelKey
		}
	}
	if perKey >= keyMax || perAddr >= addrMax {
		a.mu.Unlock()
		msg := fmt.Sprintf("too many open streams: %d for a key and %d for an address at most", keyMax, addrMax)
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
			a.streamQueue(st, streamMsg{T: "snapshot", Topic: t, Data: a.namesSnapshot(st.unit, st.view)})
		case "tally":
			a.streamQueue(st, streamMsg{T: "snapshot", Topic: t, Data: a.tallySnapshot(st.unit, st.view)})
		case "status":
			a.streamQueue(st, streamMsg{T: "snapshot", Topic: t, Data: linkStatus(st.unit.status, st.detail)})
		}
	}
	a.mu.Unlock()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		a.dropStream(st)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 10) // the client sends nothing on the stream

	// endStreams does not see a key that ended between auth and the register.
	if st.keyID != "" && !a.keys.alive(st.keyID) && !a.public.alive(st.keyID) {
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

	// So a stalled client closes its stream and does not back up the writer. Equal to wsHeartbeat, but independent of
	// it.
	const streamWriteTimeout = 5 * time.Second
	go func() {
		defer cancel()
		beat := time.NewTicker(wsHeartbeat)
		defer beat.Stop()
		write := func(b []byte) error {
			wctx, wcancel := context.WithTimeout(ctx, streamWriteTimeout)
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

// Acquires mu.
func (a *App) dropStream(st *apiStream) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.streams[st]; ok {
		delete(a.streams, st)
		close(st.send)
	}
}

// A stream must not outlive its key. Each closes through its own disconnect row, which carries why. Acquires mu.
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

// A changed view applies immediately. A stream ends when its caller lost the keyless API, a scope it uses or its log
// detail.
// Caller holds mu.
func (a *App) refreshStreams(cfg config.Config) {
	for st := range a.streams {
		if st.ended != "" {
			continue
		}
		why := ""
		cl := &apiCtx{Scopes: cfg.GuestAPIScopes, Rights: cfg.GuestRights}
		switch {
		case st.keyID == "" && !cfg.API.GuestEnabled:
			why = "the keyless API is off"
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

// streamQueue drops the stream if it cannot keep up. Caller holds mu.
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

// hears: a message from this router unit goes to the stream. nil is from no one router.
func (st *apiStream) hears(from *routerUnit) bool { return from == nil || from == st.unit }

// Caller holds mu.
func (a *App) streamFan(v any) {
	if len(a.streams) == 0 {
		return
	}
	for st := range a.streams {
		switch m := v.(type) {
		case deltaMsg:
			if m.T != "tally" || !st.topics["tally"] || !st.hears(m.from) {
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
				rows = append(rows, apiRouteRow(c))
			}
			if len(rows) > 0 {
				a.streamQueue(st, streamMsg{T: "tally", Topic: "tally", Data: map[string]any{"routes": rows}})
			}
		case namesMsg:
			if st.topics["names"] && st.hears(m.from) {
				a.streamQueue(st, streamMsg{T: "names", Topic: "names", Data: a.namesSnapshot(st.unit, st.view)})
			}
		case statusMsg:
			if st.topics["status"] && st.hears(m.from) {
				a.streamQueue(st, streamMsg{T: "status", Topic: "status", Data: linkStatus(m, st.detail)})
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
			t, topic := "entry", "history"
			if m.Update {
				t = "entryUpdate"
				if !st.topics["history"] {
					topic = "results"
				}
			}
			a.streamQueue(st, streamMsg{T: t, Topic: topic, Data: e})
		case simpleMsg:
			if m.T == "historyCleared" && st.topics["history"] {
				a.streamQueue(st, streamMsg{T: "historyCleared", Topic: "history"})
			}
		}
	}
}
