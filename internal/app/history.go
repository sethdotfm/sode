package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one take, as logged and as shown in the browser's history list.
// Ext entries are routes changed elsewhere: Client is empty and a burst of
// them across several destinations shares a Group like a confirm's takes.
type Entry struct {
	ID      int       `json:"id"`
	Time    time.Time `json:"time"`
	Client  string    `json:"client"`
	Src     int       `json:"src"`
	SrcName string    `json:"srcName"`
	Dst     int       `json:"dst"`
	DstName string    `json:"dstName"`
	Levels  []int     `json:"levels"`
	Prev    [][2]int  `json:"prev"` // [UI level, src] before the take; src -1 = nothing routed
	OK      bool      `json:"ok"`
	Err     string    `json:"err,omitempty"`
	Undo    *int      `json:"undo,omitempty"`    // ID of the entry this take undid
	Retry   *int      `json:"retry,omitempty"`   // ID of the failed entry this take retried
	Failed  []int     `json:"failed,omitempty"`  // levels the router never confirmed or that weren't sent
	Now     [][2]int  `json:"now,omitempty"`     // [UI level, src] the router reported for Failed levels afterwards (-1 = nothing routed); none = no answer
	Already []int     `json:"already,omitempty"` // levels that were already on Src (router confirmed, no change)
	Group   string    `json:"group,omitempty"`   // shared by the takes of one multi-take confirm
	GroupN  int       `json:"groupN,omitempty"`  // how many takes that confirm sent
	Ext     bool      `json:"ext,omitempty"`     // made elsewhere (a panel, Companion), seen on the router
	// ext only: group ID of the multi-take confirm this seems to follow; never one of its takes in particular
	FollowGroup string `json:"followGroup,omitempty"`
	Follow      *int   `json:"follow,omitempty"` // ext only: ID of the take this seems to follow (destination follow, automation)
	Found       bool   `json:"found,omitempty"`  // ext only: changed without a Connected; seen when Sode re-read the router
	Late        []int  `json:"late,omitempty"`   // levels that were in Failed until the router confirmed them after the take gave up
}

// History is an append-only JSONL log of takes. Only line offsets are kept
// in memory; pages are read back from disk on demand. With no path it
// keeps entries in memory instead.
type History struct {
	mu   sync.Mutex
	path string
	f    *os.File
	offs []int64 // start of each entry's line, indexed by ID
	size int64
	mem  []Entry
}

func OpenHistory(path string) (*History, error) {
	h := &History{}
	if path == "" {
		return h, nil
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(f)
	var off int64
	for {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			if len(line) > 0 { // torn final write: drop it
				log.Printf("history: dropping %d-byte partial last line", len(line))
				if err := f.Truncate(off); err != nil {
					return nil, err
				}
			}
			break
		}
		if err != nil {
			return nil, err
		}
		h.offs = append(h.offs, off)
		off += int64(len(line))
	}
	h.f, h.size, h.path = f, off, path
	log.Printf("history: %s, %d entries", path, len(h.offs))
	return h, nil
}

// Append assigns e.ID and stores it.
func (h *History) Append(e *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.f == nil {
		e.ID = len(h.mem)
		h.mem = append(h.mem, *e)
		return nil
	}
	e.ID = len(h.offs)
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := h.f.WriteAt(b, h.size); err != nil {
		return err
	}
	h.offs = append(h.offs, h.size)
	h.size += int64(len(b))
	return nil
}

// Update changes entry id with fn and stores it, keeping its ID and place.
// It reports the entry as stored, and false if there's no such entry. On
// disk the file is rewritten (temp file, then rename), since a line's length
// changes; that is rare (a late confirm), so it isn't made cheap.
func (h *History) Update(id int, fn func(*Entry)) (Entry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.f == nil {
		if id < 0 || id >= len(h.mem) {
			return Entry{}, false
		}
		fn(&h.mem[id])
		h.mem[id].ID = id
		return h.mem[id], true
	}
	if id < 0 || id >= len(h.offs) {
		return Entry{}, false
	}
	e, err := h.at(id)
	if err != nil {
		log.Printf("history: update %d: %v", id, err)
		return Entry{}, false
	}
	old, _ := json.Marshal(e)
	fn(&e)
	e.ID = id
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("history: update %d: %v", id, err)
		return e, false
	}
	if string(b) == string(old) {
		return e, true
	}
	b = append(b, '\n')
	if err := h.rewrite(id, b); err != nil {
		log.Printf("history: update %d: %v", id, err)
		return e, false
	}
	return e, true
}

// rewrite replaces entry id's line with line, via a temp file renamed over
// the log, and reopens it. Caller holds mu.
func (h *History) rewrite(id int, line []byte) error {
	start, end := h.offs[id], h.size
	if id+1 < len(h.offs) {
		end = h.offs[id+1]
	}
	tmp, err := os.CreateTemp(filepath.Dir(h.path), filepath.Base(h.path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	tmp.Chmod(0o644)            // CreateTemp makes it 0600
	_, err = io.Copy(tmp, io.NewSectionReader(h.f, 0, start))
	if err == nil {
		_, err = tmp.Write(line)
	}
	if err == nil {
		_, err = io.Copy(tmp, io.NewSectionReader(h.f, end, h.size-end))
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), h.path); err != nil {
		return err
	}
	nf, err := os.OpenFile(h.path, os.O_RDWR, 0o644)
	if err != nil {
		return err // the renamed file is complete; the next restart reads it
	}
	h.f.Close()
	h.f = nf
	delta := int64(len(line)) - (end - start)
	for i := id + 1; i < len(h.offs); i++ {
		h.offs[i] += delta
	}
	h.size += delta
	return nil
}

// Clear drops every entry; IDs start again from 0.
func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mem, h.offs, h.size = nil, nil, 0
	if h.f == nil {
		return nil
	}
	return h.f.Truncate(0)
}

func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.f == nil {
		return len(h.mem)
	}
	return len(h.offs)
}

// Page returns up to n entries older than ID before, newest first.
// before < 0 means from the newest. A page that ends inside a group runs
// on to the end of it, so a confirm's takes come back together.
func (h *History) Page(before, n int) ([]Entry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	total := len(h.offs)
	if h.f == nil {
		total = len(h.mem)
	}
	if before < 0 || before > total {
		before = total
	}
	out := []Entry{}
	for id := before - 1; id >= 0; id-- {
		e, err := h.at(id)
		if err != nil {
			return out, err
		}
		if len(out) >= n && (e.Group == "" || e.Group != out[len(out)-1].Group) {
			break
		}
		out = append(out, e)
	}
	return out, nil
}

// at reads one entry. Caller holds mu.
func (h *History) at(id int) (Entry, error) {
	if h.f == nil {
		return h.mem[id], nil
	}
	end := h.size
	if id+1 < len(h.offs) {
		end = h.offs[id+1]
	}
	buf := make([]byte, end-h.offs[id])
	if _, err := h.f.ReadAt(buf, h.offs[id]); err != nil {
		return Entry{}, err
	}
	var e Entry
	if err := json.Unmarshal(buf, &e); err != nil {
		return Entry{}, fmt.Errorf("history entry %d: %w", id, err)
	}
	return e, nil
}
