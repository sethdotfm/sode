package app

import (
	"context"
	"time"
	"unicode"
)

const (
	idemTTL    = 10 * time.Minute // how long a repeated Idempotency-Key gets the first answer
	idemMax    = 2048             // answers kept at once; the oldest goes first
	idemKeyMax = 128
)

// idemSlot is one Idempotency-Key. The first request owns it and fills status and body;
// a repeat waits on done and replays them. fp ties the key to one request, so reusing a key
// for something else is refused instead of answered with the wrong take.
type idemSlot struct {
	fp     string
	at     time.Time
	done   chan struct{}
	status int
	body   []byte
}

func validIdemKey(k string) bool {
	if k == "" || len(k) > idemKeyMax {
		return false
	}
	for _, r := range k {
		if !unicode.IsPrint(r) || r == ' ' {
			return false
		}
	}
	return true
}

// takeStart reserves one in-flight take; false when the cap is reached.
func (s *apiState) takeStart(limit int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > 0 && s.inflight >= limit {
		return false
	}
	s.inflight++
	return true
}

func (s *apiState) takeEnd() {
	s.mu.Lock()
	s.inflight--
	s.mu.Unlock()
}

// idemBegin returns the slot and whether the caller owns it. A non-owner gets the finished
// slot to replay (or mismatch=true). A slot the owner abandoned is gone, so the wait loops.
func (s *apiState) idemBegin(ctx context.Context, subject, key, fp string, now time.Time) (slot *idemSlot, owner, mismatch bool) {
	id := subject + "\x00" + key
	for {
		s.mu.Lock()
		if s.idem == nil {
			s.idem = map[string]*idemSlot{}
		}
		sl := s.idem[id]
		if sl != nil && now.Sub(sl.at) >= idemTTL {
			delete(s.idem, id)
			sl = nil
		}
		if sl == nil {
			if len(s.idem) >= idemMax {
				s.idemPruneLocked(now)
			}
			sl = &idemSlot{fp: fp, at: now, done: make(chan struct{})}
			s.idem[id] = sl
			s.mu.Unlock()
			return sl, true, false
		}
		s.mu.Unlock()
		if sl.fp != fp {
			return nil, false, true
		}
		select {
		case <-sl.done:
		case <-ctx.Done():
			return nil, false, false
		}
		// A committed answer is replayed even if the map dropped the slot meanwhile; running
		// again would take twice.
		s.mu.Lock()
		committed := sl.body != nil
		s.mu.Unlock()
		if committed {
			return sl, false, false
		}
	}
}

// idemPruneLocked drops expired slots, then the oldest committed one if the map is still
// full. A slot in flight stays, so the map may pass idemMax by the requests running now.
// Caller holds s.mu.
func (s *apiState) idemPruneLocked(now time.Time) {
	var oldest string
	var at time.Time
	for id, sl := range s.idem {
		switch {
		case sl.body == nil:
		case now.Sub(sl.at) >= idemTTL:
			delete(s.idem, id)
		case oldest == "" || sl.at.Before(at):
			oldest, at = id, sl.at
		}
	}
	if len(s.idem) >= idemMax && oldest != "" {
		delete(s.idem, oldest)
	}
}

// idemCommit stores the first answer and wakes repeats.
func (s *apiState) idemCommit(sl *idemSlot, status int, body []byte) {
	s.mu.Lock()
	sl.status, sl.body = status, body
	s.mu.Unlock()
	close(sl.done)
}

// idemAbandon forgets a slot whose request did nothing (bad input, cap), so a repeat runs.
func (s *apiState) idemAbandon(subject, key string, sl *idemSlot) {
	s.mu.Lock()
	if s.idem[subject+"\x00"+key] == sl {
		delete(s.idem, subject+"\x00"+key)
	}
	s.mu.Unlock()
	close(sl.done)
}
