package app

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// uptimeKeep: older lines are dropped at start; older ranges show no data.
const uptimeKeep = 400 * 24 * time.Hour

// slowAckMs: ACK time above which a message is slow (yellow in the graph); under the
// client's 1 s ACK timeout.
const slowAckMs = 300

const (
	upUp   = "up"
	upDown = "down"
	upOff  = "off"
)

type UptimeTrans struct {
	T int64  `json:"t"`
	S string `json:"s"`
}

type UptimeSlot struct {
	I    int `json:"i"`
	Bad  int `json:"bad"`  // messages the router didn't ACK, or only after a retry
	Slow int `json:"slow"` // ACKs slower than slowAckMs
}

// UptimeRange: Start is the state at the range's start ("" = nothing recorded).
type UptimeRange struct {
	Start  string
	Trans  []UptimeTrans
	Counts []UptimeSlot
}

type minute struct{ M, Bad, Slow int64 }

type uptimeLine struct {
	T    int64  `json:"t,omitempty"`
	S    string `json:"s,omitempty"`
	M    int64  `json:"m,omitempty"`
	Bad  int64  `json:"bad,omitempty"`
	Slow int64  `json:"slow,omitempty"`
}

// Uptime: router link history for /log: state changes and per-minute bad-message counts.
// JSONL at path plus path.alive, a stamp rewritten each minute so time Sode wasn't running
// shows as down. No path = memory only. Nothing waits on disk except Run and Close, so it
// is safe under App.mu.
type Uptime struct {
	mu    sync.Mutex
	path  string
	now   func() time.Time
	trans []UptimeTrans
	mins  []minute // completed minutes with problems, oldest first
	cur   minute   // the minute being counted
	pend  []byte   // lines not yet in the file
}

func OpenUptime(path string) (*Uptime, error) {
	return openUptime(path, time.Now)
}

func openUptime(path string, now func() time.Time) (*Uptime, error) {
	u := &Uptime{path: path, now: now}
	t := now()
	if path != "" {
		f, err := os.Open(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
			bad := 0
			for sc.Scan() {
				var l uptimeLine
				if json.Unmarshal(sc.Bytes(), &l) != nil || (l.S == "" && l.M == 0) {
					bad++
					continue
				}
				if l.S != "" {
					u.trans = append(u.trans, UptimeTrans{l.T, l.S})
				} else {
					u.mins = append(u.mins, minute{l.M, l.Bad, l.Slow})
				}
			}
			f.Close()
			if bad > 0 {
				log.Printf("uptime: skipped %d damaged lines in %s", bad, path)
			}
		}
	}
	sort.SliceStable(u.trans, func(i, j int) bool { return u.trans[i].T < u.trans[j].T })
	sort.SliceStable(u.mins, func(i, j int) bool { return u.mins[i].M < u.mins[j].M })

	if n := len(u.trans); n > 0 && u.trans[n-1].S != upOff {
		at := u.trans[n-1].T
		if b, err := os.ReadFile(alivePath(path)); path != "" && err == nil {
			if s, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(b))); err == nil && s.UnixMilli() > at {
				at = s.UnixMilli()
			}
		}
		u.trans = append(u.trans, UptimeTrans{min(at, t.UnixMilli()), upOff})
	}
	u.prune(t)
	u.trans = append(u.trans, UptimeTrans{t.UnixMilli(), upDown})
	u.cur = minute{M: t.Unix() / 60}
	if path != "" {
		if err := u.rewrite(); err != nil {
			return nil, err
		}
		u.writeAlive()
	}
	return u, nil
}

func alivePath(path string) string { return path + ".alive" }

// prune keeps the last transition before the cutoff: the state the oldest kept range
// starts in.
func (u *Uptime) prune(now time.Time) {
	cut := now.Add(-uptimeKeep).UnixMilli()
	i := sort.Search(len(u.trans), func(i int) bool { return u.trans[i].T > cut })
	if i > 1 {
		u.trans = u.trans[i-1:]
	}
	j := sort.Search(len(u.mins), func(i int) bool { return u.mins[i].M*60_000 >= cut })
	u.mins = u.mins[j:]
}

func (u *Uptime) rewrite() error {
	var b []byte
	for _, t := range u.trans {
		l, _ := json.Marshal(uptimeLine{T: t.T, S: t.S})
		b = append(append(b, l...), '\n')
	}
	for _, m := range u.mins {
		l, _ := json.Marshal(uptimeLine{M: m.M, Bad: m.Bad, Slow: m.Slow})
		b = append(append(b, l...), '\n')
	}
	return replaceFile(u.path, b)
}

func replaceFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (u *Uptime) writeAlive() {
	if err := replaceFile(alivePath(u.path), []byte(u.now().UTC().Format(time.RFC3339Nano))); err != nil {
		log.Printf("uptime: %v", err)
	}
}

func (u *Uptime) State(up bool) {
	s := upDown
	if up {
		s = upUp
	}
	u.setState(s, u.now())
}

func (u *Uptime) setState(s string, at time.Time) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if n := len(u.trans); n > 0 && u.trans[n-1].S == s {
		return
	}
	u.trans = append(u.trans, UptimeTrans{at.UnixMilli(), s})
	if u.path != "" {
		l, _ := json.Marshal(uptimeLine{T: at.UnixMilli(), S: s})
		u.pend = append(append(u.pend, l...), '\n')
	}
}

func (u *Uptime) Note(e Sent) {
	bad := e.Result != "ack" || e.Tries > 1
	slow := e.AckMs > slowAckMs
	if !bad && !slow {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.roll(false)
	if bad {
		u.cur.Bad++
	}
	if slow {
		u.cur.Slow++
	}
}

// Caller holds mu.
func (u *Uptime) roll(force bool) {
	m := u.now().Unix() / 60
	if u.cur.M == m && !force {
		return
	}
	if u.cur.Bad > 0 || u.cur.Slow > 0 {
		u.mins = append(u.mins, u.cur)
		if u.path != "" {
			l, _ := json.Marshal(uptimeLine{M: u.cur.M, Bad: u.cur.Bad, Slow: u.cur.Slow})
			u.pend = append(append(u.pend, l...), '\n')
		}
	}
	u.cur = minute{M: m}
}

func (u *Uptime) flush(final ...bool) {
	u.mu.Lock()
	u.roll(len(final) > 0 && final[0])
	b := u.pend
	u.pend = nil
	u.mu.Unlock()
	if len(b) == 0 || u.path == "" {
		return
	}
	f, err := os.OpenFile(u.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err == nil {
		_, err = f.Write(b)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		log.Printf("uptime: not saved: %v", err)
	}
}

func (u *Uptime) Stop() {
	u.setState(upOff, u.now())
	u.flush(true)
}

func (u *Uptime) Run(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	alive := u.now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			u.flush()
			if u.path != "" && u.now().Sub(alive) >= time.Minute {
				u.writeAlive()
				alive = u.now()
			}
		}
	}
}

func (u *Uptime) Range(from, to, slot int64) UptimeRange {
	if slot < 60_000 {
		slot = 60_000
	}
	if oldest := u.now().Add(-uptimeKeep).UnixMilli(); from < oldest {
		from = oldest
	}
	var r UptimeRange
	if to <= from {
		return r
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.roll(false)
	i := sort.Search(len(u.trans), func(i int) bool { return u.trans[i].T > from })
	if i > 0 {
		r.Start = u.trans[i-1].S
	}
	for ; i < len(u.trans) && u.trans[i].T <= to; i++ {
		r.Trans = append(r.Trans, u.trans[i])
	}
	add := map[int]*UptimeSlot{}
	put := func(m minute) {
		at := m.M * 60_000
		if (m.Bad == 0 && m.Slow == 0) || at < from || at >= to {
			return
		}
		k := int((at - from) / slot)
		s := add[k]
		if s == nil {
			s = &UptimeSlot{I: k}
			add[k] = s
		}
		s.Bad += int(m.Bad)
		s.Slow += int(m.Slow)
	}
	j := sort.Search(len(u.mins), func(i int) bool { return u.mins[i].M*60_000 >= from })
	for ; j < len(u.mins); j++ {
		put(u.mins[j])
	}
	put(u.cur)
	for _, s := range add {
		r.Counts = append(r.Counts, *s)
	}
	sort.Slice(r.Counts, func(i, j int) bool { return r.Counts[i].I < r.Counts[j].I })
	return r
}
