package app

import (
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings that several browsers edit apart from the Settings form (filter
// sets, salvos) all save the same way, through saveShared, and
// answer through replyOp.

// guardSave refuses c a settings change unless its settings PIN is unlocked,
// or, when trusted is true (a low-risk save), c is a trusted user.
// The policy table (policy.go) already checked this on arrival; the check is
// repeated here because a PIN can change before cfgMu is ours. It is the only
// guard: the main lock does not cover settings, the settings PIN does. Caller
// holds cfgMu, so the PIN can't change between this check and the save.
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

// saveShared saves a change to the live config for everyone, from c: one at
// a time (cfgMu), after the PIN check, mutate edits a copy, which is then
// normalized, validated and saved (update). Browsers are told by broadcast,
// which builds one message from the new config (nil: no message) and is not a
// full snapshot, so it doesn't reset their Recent.
func (a *App) saveShared(c *client, mutate func(*config.Config) error, broadcast func(config.Config) any) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, true); err != nil {
		return err
	}
	_, nr, err := a.update(mutate)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.rt.Store(nr)
	if broadcast != nil {
		a.broadcast(broadcast(nr.cfg))
	}
	a.mu.Unlock()
	return nil
}

// replyOp finishes a save from c: it logs the command (as kind) with a note
// of n things named noun ("" for none), and answers "<kind>Result" with the
// outcome, echoing id (id < 0: the request has none) and saying whether the
// PIN was why it failed.
func (a *App) replyOp(c *client, kind string, id, n int, noun string, err error) {
	note := ""
	if noun != "" {
		note = fmt.Sprintf("%d %s", n, plural(n, noun, noun+"s"))
	}
	a.replyOpNote(c, kind, id, note, err)
}

// replyOpNote is replyOp with the command's note given as is.
func (a *App) replyOpNote(c *client, kind string, id int, note string, err error) {
	res := opResult{T: kind + "Result", OK: err == nil}
	if id >= 0 {
		res.ID = &id
	}
	a.command(c, kind, outcome(err), note, err)
	if err != nil {
		res.Err = err.Error()
		res.Locked = locked(err)
	} else {
		log.Printf("config: %s saved by %s", kind, c.who())
	}
	a.reply(c, res)
}
