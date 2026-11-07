package app

import (
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/config"
)

// guardSave repeats the policy check because a PIN can change before cfgMu is ours; it is
// the only guard (the main lock doesn't cover settings). Caller holds cfgMu.
func (a *App) guardSave(c *client, trusted bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	ok := a.settingsUnlockedLocked(c)
	if trusted {
		ok = a.trustedLocked(c)
	}
	if !ok {
		return errLocked
	}
	return nil
}

// saveShared: mutate edits a copy, then update. broadcast builds one message from the new
// config (nil: none), not a full snapshot, so Recent isn't reset.
func (a *App) saveShared(c *client, mutate func(*config.Config) error, broadcast func(config.Config) any) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, true); err != nil {
		return err
	}
	return a.commit(mutate, func(_, nr *runtime) {
		if broadcast != nil {
			a.broadcast(broadcast(nr.cfg))
		}
	})
}

// replyOp: id < 0 = the request has none; reports whether the PIN was why it failed.
func (a *App) replyOp(c *client, kind string, id, n int, noun string, err error) {
	note := ""
	if noun != "" {
		note = fmt.Sprintf("%d %s", n, plural(n, noun, noun+"s"))
	}
	a.replyOpNote(c, kind, id, note, err)
}

func (a *App) replyOpNote(c *client, kind string, id int, note string, err error) {
	var idp *int
	if id >= 0 {
		idp = &id
	}
	res := opResultFor(kind, idp, err)
	a.command(c, kind, outcome(err), note, err)
	if err != nil {
		res.Locked = locked(err)
	} else {
		log.Printf("config: %s saved by %s", kind, c.who())
	}
	a.reply(c, res)
}

// opResultFor: the "<kind>Result" reply without the PIN-failure flag; replyOpNote adds it,
// and callers that log their own outcome (unlock, userSetPin) send it as is.
func opResultFor(kind string, id *int, err error) opResult {
	res := opResult{T: kind + "Result", OK: err == nil, ID: id}
	if err != nil {
		res.Err = err.Error()
	}
	return res
}
