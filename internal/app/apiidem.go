package app

import (
	"context"
	"sync"
	"time"
	"unicode"
)

const (
	idemTTL    = 10 * time.Minute // how long a repeated Idempotency-Key gets the first answer
	idemMax    = 2048             // answers kept at once; the oldest goes first
	idemKeyMax = 128
)

// apiTakeState: the global cap on API takes in flight and the Idempotency-Key answers.
// Guarded by its own mutex, never held with App.mu or across a take.
type apiTakeState struct {
	mu       sync.Mutex
	inflight int
	idem     map[string]*idemSlot
}

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
	s.take.mu.Lock()
	defer s.take.mu.Unlock()
	if limit > 0 && s.take.inflight >= limit {
		return false
	}
	s.take.inflight++
	return true
}

func (s *apiState) takeEnd() {
	s.take.mu.Lock()
	s.take.inflight--
	s.take.mu.Unlock()
}

// idemBegin returns the slot and whether the caller owns it. A non-owner gets the finished
// slot to replay (or mismatch=true). A slot the owner abandoned is gone, so the wait loops.
func (s *apiState) idemBegin(ctx context.Context, subject, key, fp string, now time.Time) (slot *idemSlot, owner, mismatch bool) {
	id := subject + "\x00" + key
	for {
		s.take.mu.Lock()
		if s.take.idem == nil {
			s.take.idem = map[string]*idemSlot{}
		}
		sl := s.take.idem[id]
		if sl != nil && now.Sub(sl.at) >= idemTTL {
			delete(s.take.idem, id)
			sl = nil
		}
		if sl == nil {
			if len(s.take.idem) >= idemMax {
				s.idemPruneLocked(now)
			}
			sl = &idemSlot{fp: fp, at: now, done: make(chan struct{})}
			s.take.idem[id] = sl
			s.take.mu.Unlock()
			return sl, true, false
		}
		s.take.mu.Unlock()
		if sl.fp != fp {
			return nil, false, true
		}
		select {
		case <-sl.done:
		case <-ctx.Done():
			return nil, false, false
		}
		s.take.mu.Lock()
		cur := s.take.idem[id]
		s.take.mu.Unlock()
		if cur == sl && sl.body != nil {
			return sl, false, false
		}
	}
}

// Caller holds take.mu.
func (s *apiState) idemPruneLocked(now time.Time) {
	var oldest string
	var at time.Time
	for id, sl := range s.take.idem {
		if now.Sub(sl.at) >= idemTTL {
			delete(s.take.idem, id)
		} else if oldest == "" || sl.at.Before(at) {
			oldest, at = id, sl.at
		}
	}
	if len(s.take.idem) >= idemMax {
		delete(s.take.idem, oldest)
	}
}

// idemCommit stores the first answer and wakes repeats.
func (s *apiState) idemCommit(sl *idemSlot, status int, body []byte) {
	s.take.mu.Lock()
	sl.status, sl.body = status, body
	s.take.mu.Unlock()
	close(sl.done)
}

// idemAbandon forgets a slot whose request did nothing (bad input, cap), so a repeat runs.
func (s *apiState) idemAbandon(subject, key string, sl *idemSlot) {
	s.take.mu.Lock()
	if s.take.idem[subject+"\x00"+key] == sl {
		delete(s.take.idem, subject+"\x00"+key)
	}
	s.take.mu.Unlock()
	close(sl.done)
}
