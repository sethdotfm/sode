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

// Sent is one message written to the router (or one that never got
// there), as logged and as shown on the /log page.
type Sent struct {
	Seq     int64     `json:"seq"`
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"` // swp08.Request kinds, plus keepalive
	Cmd     int       `json:"cmd"`
	Level   int       `json:"level"` // UI level; 0 = none or not one of ours
	Wire    int       `json:"wire"`  // wire level; -1 = none
	Dst     int       `json:"dst"`   // -1 = none
	DstName string    `json:"dstName,omitempty"`
	Src     int       `json:"src"` // -1 = none
	SrcName string    `json:"srcName,omitempty"`
	By      string    `json:"by,omitempty"`  // the browser's address, for take traffic
	Why     string    `json:"why,omitempty"` // take, check, confirm, resync, names, keepalive
	Tries   int       `json:"tries"`
	Result  string    `json:"result"` // ack, nak, noack, write, dropped, unsent
	Err     string    `json:"err,omitempty"`
	Hex     string    `json:"hex"`
}

// sentMem is how many entries the log keeps at least with no file.
const sentMem = 20000

// segment is one file of the sent log: line i holds seq first+i.
type segment struct {
	f     *os.File
	offs  []int64
	size  int64
	first int64
}

func (s *segment) end() int64 { return s.first + int64(len(s.offs)) }

// SentLog is the sent log: JSONL in two files, path and its .1, which
// together stay under a size cap. When path would pass half the cap it
// replaces the .1 and a new path starts. With no path it keeps the newest
// sentMem entries in memory.
type SentLog struct {
	mu       sync.Mutex
	path     string
	half     int64
	cur, old *segment // old is nil until the first rotation (or a .1 on disk)
	next     int64
	mem      []Sent
}

// rotatedPath is path's older file: sent.jsonl → sent.1.jsonl.
func rotatedPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + ".1" + ext
}

// errDamaged marks a file whose lines can't be trusted: it's set aside
// rather than stopping Sode from starting.
var errDamaged = errors.New("damaged")

// quarantine renames a damaged file out of the way, keeping it to look at.
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

// openSegment indexes one file and returns it with the seq of its last
// line. Only the current file is created if missing and has a torn last
// line cut off.
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

// Append assigns e.Seq and stores it. When the write fails, the seq stays
// unused (lines' seqs run without gaps) and e must not be shown as logged.
func (l *SentLog) Append(e *Sent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.Seq = l.next
	if l.cur == nil {
		l.next++
		l.mem = append(l.mem, *e)
		// Trimmed at twice the size, so the copy happens once per sentMem
		// appends rather than on every one.
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

// rotate makes the current file the .1 and starts a new one. Caller holds mu.
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

// Page returns up to n entries older than seq before, newest first, and
// whether there are older ones still. before < 0 means from the newest.
// A page's lines are read with one ReadAt per file and decoded after the
// lock is released, so a page doesn't hold up Append. A line that won't
// decode is logged and left out.
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
