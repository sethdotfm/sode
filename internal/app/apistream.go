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
	"github.com/sethdotfm/sode/internal/swp08"
)

// The stream: GET /api/v1/stream upgrades to a WebSocket that only sends. The caller names
// its topics in ?topics=a,b at connect; every frame is {"t","topic"?,"data"?}. Keep docs in
// step: openapi.yaml.
//
//	hello          {v, topics, numberOffset}, always first
//	snapshot       one per names, tally and status topic, topic set, data as the REST read
//	names          full names, filtered
//	tally          {routes:[{dst,level,src}]} changed routes only
//	status         {router, addr, err, routerUp}
//	entry          a new History entry (topic history)
//	entryUpdate    a changed entry, such as a Late Result (topics history and results, sent once)
//	historyCleared (topic history)
//	ping           heartbeat, ignore
//
// Visibility (apiView) filters every frame, so a hidden caller never sees an out-of-rights
// route or entry. A tally change whose Source is hidden reads as src -1, the same as the
// REST read omits it. Frames go out in the order the server saw the changes.

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/stream", "", (*App).apiStreamServe})
}

const apiStreamVersion = 1

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
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 10) // the stream takes no input

	st := &apiStream{view: cl.view(), topics: map[string]bool{}, send: make(chan []byte, 256)}
	for _, t := range topics {
		st.topics[t] = true
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Register and queue the snapshots under one lock so no change arrives ahead of them.
	a.mu.Lock()
	if a.streams == nil {
		a.streams = map[*apiStream]struct{}{}
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
			a.streamQueue(st, streamMsg{T: "snapshot", Topic: t, Data: streamStatus(a.status)})
		}
	}
	a.mu.Unlock()

	connected := time.Now()
	a.apiEvent(cl.Actor, "streamConnect", "ok", "topics "+strings.Join(topics, ", "), nil)
	var gone error
	defer func() {
		a.mu.Lock()
		if _, ok := a.streams[st]; ok {
			delete(a.streams, st)
			close(st.send)
		}
		a.mu.Unlock()
		var cerr error
		switch websocket.CloseStatus(gone) {
		case websocket.StatusNormalClosure, websocket.StatusGoingAway:
		default:
			if gone != nil && !strings.Contains(gone.Error(), context.Canceled.Error()) {
				cerr = gone
			}
		}
		a.apiEvent(cl.Actor, "streamDisconnect", "ok", "connected "+time.Since(connected).Round(time.Second).String(), cerr)
	}()

	go func() {
		defer cancel()
		ping := time.NewTicker(20 * time.Second)
		defer ping.Stop()
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
			case <-ping.C:
				pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Ping(pctx)
				pcancel()
				if err != nil {
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

func streamStatus(s statusMsg) map[string]any {
	return map[string]any{"router": s.Router, "addr": s.Addr, "err": s.Err, "routerUp": s.Router == string(swp08.StateUp)}
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
				src := c.Src
				if src >= 0 && !st.view.Source(src) {
					src = -1
				}
				rows = append(rows, apiRouteRow{c.Dst, c.Level, src})
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
				a.streamQueue(st, streamMsg{T: "status", Data: streamStatus(m)})
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
