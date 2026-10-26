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

// sentMem is how many entries the log keeps with no file.
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

func OpenSentLog(path string, capBytes int64) (*SentLog, error) {
	l := &SentLog{path: path, half: capBytes / 2}
	if path == "" {
		return l, nil
	}
	if l.half < 1 {
		return nil, errors.New("sent log: size cap too small")
	}
	old, oldLast, err := openSegment(rotatedPath(path), false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	cur, curLast, err := openSegment(path, true)
	if err != nil {
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
		return nil, 0, fmt.Errorf("sent log %s: %w", path, err)
	}
	if err := json.Unmarshal(last, &b); err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("sent log %s: %w", path, err)
	}
	s.first = a.Seq
	if b.Seq != a.Seq+int64(len(s.offs))-1 {
		f.Close()
		return nil, 0, fmt.Errorf("sent log %s: seqs %d…%d over %d lines", path, a.Seq, b.Seq, len(s.offs))
	}
	return s, b.Seq, nil
}

// Append assigns e.Seq and stores it.
func (l *SentLog) Append(e *Sent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.Seq = l.next
	if l.cur == nil {
		l.next++
		l.mem = append(l.mem, *e)
		if len(l.mem) > sentMem {
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
func (l *SentLog) Page(before int64, n int) ([]Sent, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if before < 0 || before > l.next {
		before = l.next
	}
	out := []Sent{}
	if l.cur == nil {
		first := l.next - int64(len(l.mem))
		seq := before - 1
		for ; seq >= first && len(out) < n; seq-- {
			out = append(out, l.mem[seq-first])
		}
		return out, seq >= first, nil
	}
	oldest := l.cur.first
	if l.old != nil {
		oldest = l.old.first
	}
	seq := before - 1
	for ; seq >= oldest && len(out) < n; seq-- {
		s := l.cur
		if seq < s.first {
			s = l.old
		}
		if s == nil || seq >= s.end() {
			continue
		}
		e, err := s.at(seq)
		if err != nil {
			return out, false, err
		}
		out = append(out, e)
	}
	return out, seq >= oldest, nil
}

func (s *segment) at(seq int64) (Sent, error) {
	i := seq - s.first
	end := s.size
	if i+1 < int64(len(s.offs)) {
		end = s.offs[i+1]
	}
	buf := make([]byte, end-s.offs[i])
	if _, err := s.f.ReadAt(buf, s.offs[i]); err != nil {
		return Sent{}, err
	}
	var e Sent
	if err := json.Unmarshal(buf, &e); err != nil {
		return Sent{}, fmt.Errorf("sent log entry %d: %w", seq, err)
	}
	return e, nil
}
