package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

const sentPage = 500

// onTx runs on the router writer goroutine (waits for ACKs): it only hands the event on; a
// full queue loses it, counted.
func (a *App) onTx(e swp08.TxEvent) {
	select {
	case a.txq <- e:
	default:
		if a.txLost.Add(1) == 1 {
			log.Printf("sent log: queue full, dropping entries")
		}
	}
}

// Only pushes, so safe under mu; a full queue loses it, counted.
func (a *App) logEvent(e Sent) {
	e.Time = time.Now()
	select {
	case a.evq <- evItem{Sent: e}:
	default:
		a.txLost.Add(1)
	}
}

// evItem is a queued event row. done, when set, is closed once logSent has written the row
// or failed to (Kill waits on it).
type evItem struct {
	Sent
	done chan struct{}
}

func (a *App) command(c *client, why, result, note string, err error) {
	e := event(kindCommand, why, c.who(), result)
	e.Note = note
	if err != nil {
		e.Err = err.Error()
	}
	a.logEvent(e)
}

// logSent is the only writer, so seqs, live feed and file agree on order. On ctx end it
// writes what is queued and a stop row.
func (a *App) logSent(ctx context.Context) {
	failing := false // logged once per run of failed writes
	write := func(e Sent) {
		if err := a.sent.Append(&e); err != nil {
			// Not saved, so not shown live; its seq goes to the next entry. Counted with
			// queue drops.
			a.txLost.Add(1)
			if !failing {
				log.Printf("sent log: not saved: %v", err)
			}
			failing = true
			return
		}
		failing = false
		a.mu.Lock()
		a.pendSent = append(a.pendSent, e)
		a.mu.Unlock()
	}
	// Counted before the append so a failing disk doesn't lose the graph's data.
	tx := func(ev swp08.TxEvent) {
		e := a.describe(ev)
		a.up.Note(e)
		write(e)
	}
	writeItem := func(it evItem) {
		write(it.Sent)
		if it.done != nil {
			close(it.done)
		}
	}
	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case ev := <-a.txq:
					tx(ev)
					continue
				case it := <-a.evq:
					writeItem(it)
					continue
				default:
				}
				break
			}
			stop := event(kindServer, "", "", "stop")
			stop.Time = time.Now()
			write(stop)
			return
		case ev := <-a.txq:
			tx(ev)
		case it := <-a.evq:
			writeItem(it)
		}
	}
}

func (a *App) describe(ev swp08.TxEvent) Sent {
	r := a.r()
	q := swp08.DescribeTx(ev.Msg)
	e := Sent{Time: ev.At, Kind: q.Kind, Wire: q.Level, Dst: q.Dst, Src: q.Src,
		By: ev.Why.By, Why: string(ev.Why.Reason), Tries: ev.Tries, Result: ev.Result, Err: ev.Err,
		Hex: fmt.Sprintf("% X", ev.Msg), AckMs: float64(ev.Ack.Microseconds()) / 1000}
	if len(ev.Msg) > 0 {
		e.Cmd = int(ev.Msg[0])
	}
	switch {
	case q.Kind == "salvoStage" || q.Kind == "salvoStageExt":
		e.Note = fmt.Sprintf("salvo %d", q.Group)
	case q.Kind == "salvoGo" && q.Clear:
		e.Note = fmt.Sprintf("salvo %d: clear", q.Group)
	case q.Kind == "salvoGo":
		e.Note = fmt.Sprintf("salvo %d: set", q.Group)
	}
	if ev.Why.Note != "" {
		if e.Note != "" {
			e.Note += "; "
		}
		e.Note += ev.Why.Note
	}
	if ev.Why.Reason == swp08.ReasonKeepalive {
		// A link check; whatever it asks about is beside the point.
		e.Kind = "keepalive"
		q.Level, q.Dst = -1, -1
		e.Wire, e.Dst = -1, -1
	}
	if l, ok := r.byWire[q.Level]; ok && q.Level >= 0 {
		e.Level = l.UI
	}
	a.mu.Lock()
	if q.Dst >= 0 {
		e.DstName = a.router.dstNames[q.Dst]
	}
	if q.Src >= 0 {
		e.SrcName = a.router.srcNames[q.Src]
	}
	a.mu.Unlock()
	return e
}

// Caller holds mu.
func (a *App) broadcastSent(es []Sent) {
	b, err := json.Marshal(sentAddMsg{T: "sentAdd", Entries: es, Total: a.sent.Total(), Lost: a.txLost.Load()})
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

// before -1 = newest. The read loop subscribed c already, in the policy step.
func (a *App) sentHistory(c *client, before int64, n int) {
	if n <= 0 || n > sentPage {
		n = sentPage
	}
	page, more, err := a.sent.Page(before, n)
	if err != nil {
		log.Printf("sent log: %v", err)
	}
	a.reply(c, sentPageMsg{T: "sentPage", Before: before, Entries: page, More: more, Total: a.sent.Total(), Lost: a.txLost.Load()})
}
