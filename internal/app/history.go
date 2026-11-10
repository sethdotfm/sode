package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type Entry struct {
	ID          int       `json:"id"`
	Time        time.Time `json:"time"`
	Client      string    `json:"client"`
	Actor       *Actor    `json:"actor,omitempty"` // set for an API actor: Client then holds its label, never an address
	Src         int       `json:"src"`
	SrcName     string    `json:"srcName"`
	Dst         int       `json:"dst"`
	DstName     string    `json:"dstName"`
	Levels      []int     `json:"levels"`
	Prev        [][2]int  `json:"prev"` // [UI level, src] before the take; src -1 = nothing routed
	OK          bool      `json:"ok"`
	Err         string    `json:"err,omitempty"`
	Undo        *int      `json:"undo,omitempty"`
	Retry       *int      `json:"retry,omitempty"`
	Failed      []int     `json:"failed,omitempty"`  // levels the router never confirmed or that weren't sent
	Now         [][2]int  `json:"now,omitempty"`     // [UI level, src] the router reported for Failed levels afterwards (-1 = nothing routed); none = no answer
	Already     []int     `json:"already,omitempty"` // levels that were already on Src (router confirmed, no change)
	Group       string    `json:"group,omitempty"`
	GroupN      int       `json:"groupN,omitempty"`
	Preset      string    `json:"preset,omitempty"` // name of the preset this take was fired from (browser-supplied, cleaned); older files call it "salvo"
	Ext         bool      `json:"ext,omitempty"`
	FollowGroup string    `json:"followGroup,omitempty"`
	Follow      *int      `json:"follow,omitempty"` // ext only: ID of the take this seems to follow (destination follow, automation)
	Found       bool      `json:"found,omitempty"`  // ext only: changed without a Connected; seen when Sode re-read the router
	Late        []int     `json:"late,omitempty"`   // levels that were in Failed until the router confirmed them after the take gave up
	Unsent      []int     `json:"unsent,omitempty"`
	SendErr     string    `json:"sendErr,omitempty"`
	Cut         bool      `json:"cut,omitempty"` // the router changed under the take, so its Failed levels just got no answer
	// Panel and Instance name the panel page that made the take (panelNames). With a user's key they are the
	// page's own word, which the log marks unverified.
	Panel    string `json:"panel,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// logLine: off turns 0-based numbers into the router's display numbers.
func (e *Entry) logLine(off int) string {
	undo := "-"
	if e.Undo != nil {
		undo = fmt.Sprint(*e.Undo)
	}
	retry := "-"
	if e.Retry != nil {
		retry = fmt.Sprint(*e.Retry)
	}
	result := "ok"
	if !e.OK {
		result = e.Err
	}
	group := "-"
	if e.Group != "" {
		group = fmt.Sprintf("%s/%d", e.Group, e.GroupN)
	}
	if e.Ext {
		follow := "-"
		if e.Follow != nil {
			follow = fmt.Sprint(*e.Follow)
		}
		followGroup := "-"
		if e.FollowGroup != "" {
			followGroup = e.FollowGroup
		}
		return fmt.Sprintf("EXT src=%d %q dst=%d %q levels=%v group=%s follow=%s followGroup=%s found=%t",
			e.Src+off, e.SrcName, e.Dst+off, e.DstName, e.Levels, group, follow, followGroup, e.Found)
	}
	return fmt.Sprintf("TAKE client=%s src=%d %q dst=%d %q levels=%v undo=%s retry=%s group=%s result=%q",
		e.Client, e.Src+off, e.SrcName, e.Dst+off, e.DstName, e.Levels, undo, retry, group, result)
}

// History: append-only JSONL; only line offsets are kept in memory. With no path, entries
// live in memory.
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

// Update rewrites the file (temp file, rename) since a line's length changes; rare (late
// confirm), so callers keep it off hot paths (App.lateWorker).
func (h *History) Update(id int, fn func(*Entry)) (Entry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.f == nil {
		if id < 0 || id >= len(h.mem) {
			return Entry{}, false
		}
		// Copy: entries already handed out share the slices.
		e := cloneEntry(h.mem[id])
		fn(&e)
		e.ID = id
		h.mem[id] = e
		return e, true
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

// Caller holds mu.
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
	if err == nil {
		err = os.Rename(tmp.Name(), h.path)
	}
	if err != nil {
		tmp.Close()
		return err
	}
	// tmp is the log now and already open read-write, so no reopen can fail and leave h.f
	// on the replaced file.
	h.f.Close()
	h.f = tmp
	delta := int64(len(line)) - (end - start)
	for i := id + 1; i < len(h.offs); i++ {
		h.offs[i] += delta
	}
	h.size += delta
	return nil
}

func cloneEntry(e Entry) Entry {
	e.Levels = slices.Clone(e.Levels)
	e.Prev = slices.Clone(e.Prev)
	e.Failed = slices.Clone(e.Failed)
	e.Now = slices.Clone(e.Now)
	e.Already = slices.Clone(e.Already)
	e.Late = slices.Clone(e.Late)
	e.Unsent = slices.Clone(e.Unsent)
	return e
}

func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mem, h.offs, h.size = nil, nil, 0
	if h.f == nil {
		return nil
	}
	return h.f.Truncate(0)
}

// A page ending inside a group runs to its end, so a confirm's takes come back together.
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

// Caller holds mu.
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

// Loads the preset name from "salvo" for old history files.
func (e *Entry) UnmarshalJSON(b []byte) error {
	type plain Entry
	aux := struct {
		*plain
		Salvo string `json:"salvo"`
	}{plain: (*plain)(e)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if e.Preset == "" {
		e.Preset = aux.Salvo
	}
	return nil
}
