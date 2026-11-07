package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Sent struct {
	Seq     int64     `json:"seq"`
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"` // swp08.Request kinds, plus keepalive and the event kinds (kindBrowser…)
	Cmd     int       `json:"cmd"`
	Level   int       `json:"level"` // UI level; 0 = none or not one of ours
	Wire    int       `json:"wire"`  // wire level; -1 = none
	Dst     int       `json:"dst"`   // -1 = none
	DstName string    `json:"dstName,omitempty"`
	Src     int       `json:"src"` // -1 = none
	SrcName string    `json:"srcName,omitempty"`
	By      string    `json:"by,omitempty"`  // the browser's address, for take traffic
	Why     string    `json:"why,omitempty"` // take, check, confirm, resync, names, keepalive; for events, what happened
	Tries   int       `json:"tries"`
	Result  string    `json:"result"` // ack, nak, noack, write, dropped, unsent; for events, see event()
	Err     string    `json:"err,omitempty"`
	Hex     string    `json:"hex"`
	Note    string    `json:"note,omitempty"`  // events: the detail (a count, an address)
	AckMs   float64   `json:"ackMs,omitempty"` // how long the router took to ACK
}

// Event kinds: link events share the log (seqs, paging, live view); cmd, wire level, dst
// and src are -1 and hex is empty.
const (
	kindBrowser = "browser" // a browser's WebSocket connected or dropped
	kindCommand = "command" // something a browser asked for that acts
	kindLink    = "link"    // the router link came up, went down or was changed
	kindServer  = "server"  // Sode started or stopped
)

func event(kind, why, by, result string) Sent {
	return Sent{Kind: kind, Why: why, By: by, Result: result, Cmd: -1, Wire: -1, Dst: -1, Src: -1}
}

// sentMem: entries kept in memory when there is no file.
const sentMem = 20000

// segment: line i holds seq first+i.
type segment struct {
	f     *os.File
	offs  []int64
	size  int64
	first int64
}

func (s *segment) end() int64 { return s.first + int64(len(s.offs)) }

// SentLog: JSONL in path and path.1, together under a size cap; when path passes half the
// cap it replaces .1. With no path, the newest sentMem entries live in memory.
type SentLog struct {
	mu       sync.Mutex
	path     string
	half     int64
	cur, old *segment // old is nil until the first rotation (or a .1 on disk)
	next     int64
	mem      []Sent
}

func rotatedPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + ".1" + ext
}

var errDamaged = errors.New("damaged")

func quarantine(path string, why error) error {
	bad := path + ".bad-" + time.Now().Format("20060102-150405")
	log.Printf("sent log: %v; moved to %s and starting afresh", why, bad)
	return os.Rename(path, bad)
}

func OpenSentLog(path string, capBytes int64) (*SentLog, error) {
	l := &SentLog{path: path, half: capBytes / 2}
	if path == "" {
		return l, nil
	}
	if l.half < 1 {
		return nil, errors.New("sent log: size cap too small")
	}
	old, oldLast, err := openSegment(rotatedPath(path), false)
	if errors.Is(err, errDamaged) {
		err = quarantine(rotatedPath(path), err)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	cur, curLast, err := openSegment(path, true)
	if errors.Is(err, errDamaged) {
		if err = quarantine(path, err); err == nil {
			cur, curLast, err = openSegment(path, true)
		}
	}
	if err != nil {
		if old != nil {
			old.f.Close()
		}
		return nil, err
	}
	l.next = 0
	if old != nil && len(old.offs) > 0 {
		l.old = old
		l.next = oldLast + 1
	} else if old != nil {
		old.f.Close()
	}
	if len(cur.offs) > 0 {
		l.next = curLast + 1
	} else {
		cur.first = l.next
	}
	l.cur = cur
	log.Printf("sent log: %s, next seq %d", path, l.next)
	return l, nil
}

// openSegment: only the current file is created if missing and has a torn last line cut
// off.
func openSegment(path string, current bool) (*segment, int64, error) {
	flag := os.O_RDWR
	if current {
		flag |= os.O_CREATE
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return nil, 0, err
	}
	s := &segment{f: f}
	r := bufio.NewReader(f)
	var first, last []byte
	var off int64
	for {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			if len(line) > 0 && current {
				log.Printf("sent log: dropping %d-byte partial last line", len(line))
				if err := f.Truncate(off); err != nil {
					f.Close()
					return nil, 0, err
				}
			}
			break
		}
		if err != nil {
			f.Close()
			return nil, 0, err
		}
		if first == nil {
			first = line
		}
		last = line
		s.offs = append(s.offs, off)
		off += int64(len(line))
	}
	s.size = off
	if first == nil {
		return s, 0, nil
	}
	var a, b Sent
	if err := json.Unmarshal(first, &a); err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("%s %w: first line: %v", path, errDamaged, err)
	}
	if err := json.Unmarshal(last, &b); err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("%s %w: last line: %v", path, errDamaged, err)
	}
	s.first = a.Seq
	if b.Seq != a.Seq+int64(len(s.offs))-1 {
		f.Close()
		return nil, 0, fmt.Errorf("%s %w: seqs %d…%d over %d lines", path, errDamaged, a.Seq, b.Seq, len(s.offs))
	}
	return s, b.Seq, nil
}

// Append: on a failed write the seq stays unused (no gaps) and e must not be shown as
// logged.
func (l *SentLog) Append(e *Sent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.Seq = l.next
	if l.cur == nil {
		l.next++
		l.mem = append(l.mem, *e)
		// Trimmed at twice the size so the copy happens once per sentMem appends.
		if len(l.mem) > 2*sentMem {
			l.mem = append(l.mem[:0:0], l.mem[len(l.mem)-sentMem:]...)
		}
		return nil
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if l.cur.size > 0 && l.cur.size+int64(len(b)) > l.half {
		if err := l.rotate(); err != nil {
			log.Printf("sent log: rotating: %v", err)
		}
	}
	if _, err := l.cur.f.WriteAt(b, l.cur.size); err != nil {
		l.cur.f.Truncate(l.cur.size) // best effort: no partial line left behind
		return err
	}
	l.next++
	l.cur.offs = append(l.cur.offs, l.cur.size)
	l.cur.size += int64(len(b))
	return nil
}

func (l *SentLog) rotate() error {
	if err := os.Rename(l.path, rotatedPath(l.path)); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if l.old != nil {
		l.old.f.Close()
	}
	l.old, l.cur = l.cur, &segment{f: f, first: l.next}
	return nil
}

func (l *SentLog) Total() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cur == nil {
		return int64(len(l.mem))
	}
	if l.old != nil {
		return l.next - l.old.first
	}
	return l.next - l.cur.first
}

// Page: n entries older than before (< 0 = newest), newest first. Lines are decoded after
// the lock is released so Append isn't held up; undecodable lines are logged and skipped.
func (l *SentLog) Page(before int64, n int) ([]Sent, bool, error) {
	l.mu.Lock()
	if before < 0 || before > l.next {
		before = l.next
	}
	out := []Sent{}
	if l.cur == nil {
		defer l.mu.Unlock()
		first := l.next - int64(len(l.mem))
		seq := before - 1
		for ; seq >= first && len(out) < n; seq-- {
			out = append(out, l.mem[seq-first])
		}
		return out, seq >= first, nil
	}
	type line struct {
		seq int64
		b   []byte
	}
	var lines []line
	var err error
	oldest := l.cur.first
	if l.old != nil {
		oldest = l.old.first
	}
	seq := before - 1
	for seq >= oldest && len(lines) < n {
		s := l.cur
		if seq < s.first {
			s = l.old
		}
		if s == nil {
			break
		}
		if seq >= s.end() { // between the files: on to the older one's last
			seq = s.end() - 1
			continue
		}
		hi := seq - s.first
		lo := max(0, hi-int64(n-len(lines))+1)
		end := s.size
		if hi+1 < int64(len(s.offs)) {
			end = s.offs[hi+1]
		}
		buf := make([]byte, end-s.offs[lo])
		if _, err = s.f.ReadAt(buf, s.offs[lo]); err != nil {
			break
		}
		for i := hi; i >= lo; i-- {
			to := int64(len(buf))
			if i+1 <= hi {
				to = s.offs[i+1] - s.offs[lo]
			}
			lines = append(lines, line{s.first + i, buf[s.offs[i]-s.offs[lo] : to]})
		}
		seq = s.first + lo - 1
	}
	more := err == nil && seq >= oldest
	l.mu.Unlock()

	for _, ln := range lines {
		var e Sent
		if err := json.Unmarshal(ln.b, &e); err != nil {
			log.Printf("sent log entry %d: %v", ln.seq, err)
			continue
		}
		out = append(out, e)
	}
	return out, more, err
}
