package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Entry is one take, as logged and as shown in the browser's history list.
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
	Already []int     `json:"already,omitempty"` // levels that were already on Src (router confirmed, no change)
	Group   string    `json:"group,omitempty"`   // shared by the takes of one multi-take confirm
	GroupN  int       `json:"groupN,omitempty"`  // how many takes that confirm sent
}

// History is an append-only JSONL log of takes. Only line offsets are kept
// in memory; pages are read back from disk on demand. With no path it
// keeps entries in memory instead.
type History struct {
	mu   sync.Mutex
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
	h.f, h.size = f, off
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
