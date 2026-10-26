package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/swp08"
)

// sentPage is how many sent log entries one request gets at most.
const sentPage = 500

// onTx runs on the router writer goroutine, which waits for ACKs, so it
// only hands the event on. A full queue loses it, counted.
func (a *App) onTx(e swp08.TxEvent) {
	select {
	case a.txq <- e:
	default:
		if a.txLost.Add(1) == 1 {
			log.Printf("sent log: queue full, dropping entries")
		}
	}
}

// logSent writes queued events to the sent log and queues them for /log
// pages, until ctx ends.
func (a *App) logSent(ctx context.Context) {
	failing := false // logged once per run of failed writes
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-a.txq:
			// Link checks that went fine are only counted: one every 10s
			// idle would crowd takes out of the size cap.
			if ev.Why.Reason == "keepalive" && ev.Result == swp08.TxAck && ev.Tries == 1 {
				a.keepalives.Add(1)
				a.mu.Lock()
				a.keepDirty = true
				a.mu.Unlock()
				continue
			}
			e := a.describe(ev)
			if err := a.sent.Append(&e); err != nil {
				// Not saved, so not shown live either: its seq goes to the
				// next entry. Counted with the ones the queue dropped.
				a.txLost.Add(1)
				if !failing {
					log.Printf("sent log: not saved: %v", err)
				}
				failing = true
				continue
			}
			failing = false
			a.mu.Lock()
			a.pendSent = append(a.pendSent, e)
			a.mu.Unlock()
		}
	}
}

// describe decodes an event, with names as they are now.
func (a *App) describe(ev swp08.TxEvent) Sent {
	r := a.r()
	q := swp08.DescribeTx(ev.Msg)
	e := Sent{Time: ev.At, Kind: q.Kind, Wire: q.Level, Dst: q.Dst, Src: q.Src,
		By: ev.Why.By, Why: ev.Why.Reason, Tries: ev.Tries, Result: ev.Result, Err: ev.Err,
		Hex: fmt.Sprintf("% X", ev.Msg)}
	if len(ev.Msg) > 0 {
		e.Cmd = int(ev.Msg[0])
	}
	if ev.Why.Reason == "keepalive" {
		e.Kind = "keepalive"
	}
	if l, ok := r.byWire[q.Level]; ok && q.Level >= 0 {
		e.Level = l.UI
	}
	a.mu.Lock()
	if q.Dst >= 0 {
		e.DstName = a.dstNames[q.Dst]
	}
	if q.Src >= 0 {
		e.SrcName = a.srcNames[q.Src]
	}
	a.mu.Unlock()
	return e
}

// broadcastSent sends new entries, oldest first, to the browsers showing
// the sent log; none just updates the keepalive count. Caller holds mu.
func (a *App) broadcastSent(es []Sent) {
	if es == nil {
		es = []Sent{}
	}
	b, err := json.Marshal(map[string]any{"t": "sentAdd", "entries": es, "lost": a.txLost.Load(), "keepalives": a.keepalives.Load()})
	if err != nil {
		log.Printf("marshal: %v", err)
		return
	}
	for c := range a.clients {
		if c.sent {
			a.deliver(c, b)
		}
	}
}

// sentHistory answers a /log page's request for entries older than seq
// before (-1: the newest) and from then on sends it new ones.
func (a *App) sentHistory(c *client, before int64, n int) {
	if n <= 0 || n > sentPage {
		n = sentPage
	}
	// Subscribe first: an entry logged between the read and the subscribe
	// would be missed, one in both is dropped by the page (by seq).
	a.mu.Lock()
	c.sent = true
	a.mu.Unlock()
	page, more, err := a.sent.Page(before, n)
	if err != nil {
		log.Printf("sent log: %v", err)
	}
	a.reply(c, map[string]any{"t": "sentPage", "before": before, "entries": page, "more": more, "lost": a.txLost.Load(), "keepalives": a.keepalives.Load()})
}
