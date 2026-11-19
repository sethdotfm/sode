package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// Card states. A card leaves cardWaiting once and never comes back.
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
	art map[string][]byte // memory only: "<card>/<panel>/<art>"
}

func loadInbox(cfgPath string) *inboxStore {
	s := &inboxStore{art: map[string][]byte{}}
	s.by = map[string]inboxCard{}
	s.prefix = "c-"
	s.idOf = func(c *inboxCard) *string { return &c.ID }
	s.nameOf = func(c inboxCard) string { return c.At.UTC().Format(time.RFC3339Nano) }
	s.pathOf = func(dir string, c inboxCard) string { return filepath.Join(dir, c.ID+".yaml") }
	if cfgPath == "" {
		return s
	}
	s.dir = filepath.Join(filepath.Dir(cfgPath), inboxDir)
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

func (s *inboxStore) artDir(card string) string { return filepath.Join(s.dir, card+".art") }

func (s *inboxStore) artKey(card, panel, art string) string { return card + "/" + panel + "/" + art }

// artData is one frozen artwork file of a waiting card.
func (s *inboxStore) artData(card, panel, art string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		b, ok := s.art[s.artKey(card, panel, art)]
		return b, ok
	}
	b, err := os.ReadFile(filepath.Join(s.artDir(card), panel, art))
	return b, err == nil
}

// waitingFor counts to's waiting cards. Caller holds mu.
func (s *inboxStore) waitingForLocked(to string) int {
	n := 0
	for _, c := range s.by {
		if c.To == to && c.State == cardWaiting {
			n++
		}
	}
	return n
}

// add stores one card per recipient, or none: full names the first recipient whose Inbox is full.
// art is keyed "<panel>/<art>" and goes with every card.
func (s *inboxStore) add(cards []inboxCard, art map[string][]byte) (stored []inboxCard, full string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range cards {
		if s.waitingForLocked(c.To) >= inboxWaitingMax {
			return nil, c.ToName, nil
		}
	}
	for _, c := range cards {
		c.ID = s.newIDLocked()
		if err := s.putArtLocked(c.ID, art); err != nil {
			s.dropArtLocked(c.ID)
			return stored, "", err
		}
		if err := s.writeLocked(c); err != nil {
			s.dropArtLocked(c.ID)
			return stored, "", err
		}
		s.by[c.ID] = c
		stored = append(stored, c)
	}
	return stored, "", nil
}

func (s *inboxStore) putArtLocked(card string, art map[string][]byte) error {
	for k, b := range art {
		if s.dir == "" {
			s.art[card+"/"+k] = b
			continue
		}
		path := filepath.Join(s.artDir(card), filepath.FromSlash(k))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := config.AtomicWrite(path, b); err != nil {
			return err
		}
	}
	return nil
}

func (s *inboxStore) dropArtLocked(card string) {
	if s.dir != "" {
		os.RemoveAll(s.artDir(card))
		return
	}
	for k := range s.art {
		if strings.HasPrefix(k, card+"/") {
			delete(s.art, k)
		}
	}
}

// end moves a waiting card to state and drops its items and artwork.
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

// sweep expires waiting cards older than ttl and forgets answered cards older than keep. It returns
// the cards it expired.
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

// usersRemoved: a removed recipient's waiting cards end as Expired for the sender, and the cards
// the recipient answered go. A removed sender's cards stay. It returns the cards it expired.
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

// cards is every card that keep accepts, oldest first.
func (s *inboxStore) cards(keep func(inboxCard) bool) []inboxCard {
	out := s.sorted(keep)
	slices.SortStableFunc(out, func(a, b inboxCard) int { return a.At.Compare(b.At) })
	return out
}
