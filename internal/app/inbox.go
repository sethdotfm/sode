package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// Inbox store (Send a copy, GLOSSARY.md): one card per send and recipient, frozen at send time. A card
// keeps its items and their artwork only while it waits; an answered card stays as a Sent list line
// until inboxKeep passes. Cards sit in inbox/<id>.yaml beside the config, which no export reads.

const (
	inboxDir        = "inbox"
	inboxWaitingMax = 100
	inboxTTL        = 30 * 24 * time.Hour
	inboxKeep       = 30 * 24 * time.Hour // how long an answered card stays in the Sent list
)

// Card states. A card that leaves cardWaiting never comes back.
const (
	cardWaiting   = "waiting"
	cardAccepted  = "accepted"
	cardDeclined  = "declined"
	cardExpired   = "expired"
	cardWithdrawn = "withdrawn"
)

var errCardGone = errors.New("this card is closed")

// inboxCard: From and To are user ids. The names are the ones at send time, for the Log after a removal.
type inboxCard struct {
	ID       string    `yaml:"id"`
	From     string    `yaml:"from"`
	FromName string    `yaml:"fromName"`
	To       string    `yaml:"to"`
	ToName   string    `yaml:"toName"`
	At       time.Time `yaml:"at"`
	State    string    `yaml:"state"`
	Ended    time.Time `yaml:"ended,omitempty"`
	Count    int       `yaml:"count"`
	Accepted int       `yaml:"accepted,omitempty"`
	Items    cardItems `yaml:"items,omitempty"`
}

// cardItems: panel Art lists what the card holds under art/<panel id>/<art id>.
type cardItems struct {
	Presets    []config.Preset    `yaml:"presets,omitempty"`
	FilterSets []config.FilterSet `yaml:"filterSets,omitempty"`
	Panels     []Panel            `yaml:"panels,omitempty"`
}

func (it cardItems) count() int { return len(it.Presets) + len(it.FilterSets) + len(it.Panels) }

type inboxStore struct {
	yamlDir[inboxCard]
	art     blobDir         // "<card>.art/<panel>/<art>" under the inbox folder
	pending map[string]bool // card ids add has picked and is writing art for
}

func loadInbox(cfgPath string) *inboxStore {
	s := &inboxStore{pending: map[string]bool{}}
	s.by = map[string]inboxCard{}
	s.prefix = "c-"
	s.idOf = func(c *inboxCard) *string { return &c.ID }
	s.nameOf = func(c inboxCard) string { return c.At.UTC().Format(time.RFC3339Nano) }
	s.pathOf = func(dir string, c inboxCard) string { return filepath.Join(dir, c.ID+".yaml") }
	if cfgPath == "" {
		return s
	}
	s.dir = filepath.Join(filepath.Dir(cfgPath), inboxDir)
	s.art.dir = s.dir
	err := s.loadDir(s.dir, func(path, id string, b []byte, err error) (inboxCard, bool) {
		var c inboxCard
		if err == nil {
			err = yaml.Unmarshal(b, &c)
		}
		if err == nil && c.ID != id {
			err = fmt.Errorf("id %q in a file named %s", c.ID, id)
		}
		if err != nil {
			log.Printf("inbox: skipped %s: %v", path, err)
			return c, false
		}
		return c, true
	})
	if err != nil && !os.IsNotExist(err) {
		log.Printf("inbox: %v", err)
	}
	return s
}

func (s *inboxStore) artData(card, panel, art string) ([]byte, bool) {
	return s.art.get(card + ".art/" + panel + "/" + art)
}

// add stores one card per recipient, or none: full names the first recipient whose Inbox is full.
// art is keyed "<panel>/<art>" and goes with every card. live is read after the write: a user removed
// before it loses the card here, one removed after it is caught by usersRemoved.
// The art is written outside mu, since pushInboxMu takes mu under App.mu; full is asked again after it.
func (s *inboxStore) add(cards []inboxCard, art map[string][]byte, live func(user string) bool) (stored []inboxCard, full string, err error) {
	ids, full := s.reserve(cards)
	if full != "" {
		return nil, full, nil
	}
	for _, id := range ids {
		for k, b := range art {
			if err == nil {
				err = s.art.put(id+".art/"+k, b)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		delete(s.pending, id)
	}
	if err == nil {
		full = s.fullLocked(cards)
	}
	if err != nil || full != "" {
		for _, id := range ids {
			s.dropArtLocked(id)
		}
		return nil, full, err
	}
	for i, c := range cards {
		c.ID = ids[i]
		if err := s.writeLocked(c); err != nil {
			for _, id := range ids[i:] {
				s.dropArtLocked(id)
			}
			for _, k := range stored {
				s.forgetLocked(k)
			}
			return nil, "", err
		}
		s.by[c.ID] = c
		stored = append(stored, c)
	}
	return slices.DeleteFunc(stored, func(c inboxCard) bool {
		if live(c.To) {
			return false
		}
		s.forgetLocked(c)
		return true
	}), "", nil
}

// reserve picks an id per card and keeps it from other adds until add has stored or dropped the card.
func (s *inboxStore) reserve(cards []inboxCard) (ids []string, full string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if full = s.fullLocked(cards); full != "" {
		return nil, full
	}
	for range cards {
		id := s.newIDLocked()
		for s.pending[id] {
			id = s.newIDLocked()
		}
		s.pending[id] = true
		ids = append(ids, id)
	}
	return ids, ""
}

// fullLocked names the first recipient whose Inbox is full, or "".
func (s *inboxStore) fullLocked(cards []inboxCard) string {
	waiting := map[string]int{}
	for _, c := range s.by {
		if c.State == cardWaiting {
			waiting[c.To]++
		}
	}
	for _, c := range cards {
		if waiting[c.To] >= inboxWaitingMax {
			return c.ToName
		}
	}
	return ""
}

func (s *inboxStore) dropArtLocked(card string) { s.art.delPrefix(card + ".art") }

func (s *inboxStore) end(id, state string, accepted int, now time.Time) (inboxCard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endLocked(id, state, accepted, now)
}

func (s *inboxStore) endLocked(id, state string, accepted int, now time.Time) (inboxCard, error) {
	c, ok := s.by[id]
	if !ok || c.State != cardWaiting {
		return c, errCardGone
	}
	c.State, c.Ended, c.Accepted, c.Items = state, now, accepted, cardItems{}
	if err := s.writeLocked(c); err != nil {
		return c, err
	}
	s.by[id] = c
	s.dropArtLocked(id)
	return c, nil
}

// sweep returns the cards it expired.
func (s *inboxStore) sweep(now time.Time, ttl, keep time.Duration) []inboxCard {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []inboxCard
	for id, c := range s.by {
		switch {
		case c.State == cardWaiting && !now.Before(c.At.Add(ttl)):
			if e, err := s.endLocked(id, cardExpired, 0, now); err == nil {
				out = append(out, e)
			}
		case c.State != cardWaiting && !now.Before(c.Ended.Add(keep)):
			s.forgetLocked(c)
		}
	}
	return out
}

func (s *inboxStore) forgetLocked(c inboxCard) {
	if s.dir != "" {
		os.Remove(s.pathOf(s.dir, c))
	}
	s.dropArtLocked(c.ID)
	delete(s.by, c.ID)
}

// usersRemoved: a removed recipient's waiting cards end as Expired for the sender. A card goes only when
// its sender is removed too. It returns the cards it expired.
func (s *inboxStore) usersRemoved(removed map[string]bool, now time.Time) []inboxCard {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []inboxCard
	for id, c := range s.by {
		if !removed[c.To] {
			continue
		}
		if c.State == cardWaiting {
			if e, err := s.endLocked(id, cardExpired, 0, now); err == nil {
				out = append(out, e)
			}
		}
		if removed[c.From] {
			s.forgetLocked(s.by[id])
		}
	}
	return out
}

func (s *inboxStore) cards(keep func(inboxCard) bool) []inboxCard {
	out := s.sorted(keep)
	slices.SortStableFunc(out, func(a, b inboxCard) int { return a.At.Compare(b.At) })
	return out
}
