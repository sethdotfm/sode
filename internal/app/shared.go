package app

import (
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/config"
)

// guardSave repeats the policy's Role check, because a Role or sign-in can change before
// cfgMu is ours. Caller holds cfgMu.
func (a *App) guardSave(c *client, need string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.needRoleMu(c, need)
}

// saveList writes a shared list (filter sets, presets) for a Trusted user. msg builds the one
// message the new config sends, not a full hello, so History is not reset.
func (a *App) saveList(c *client, mutate func(*config.Config) error, msg func(config.Config) any) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, config.RoleTrusted); err != nil {
		return err
	}
	return a.commit(mutate, func(_, nr *runtime) { a.broadcast(msg(nr.cfg)) })
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
	a.replyOpResult(c, kind, note, opResultFor(kind, idp, err), err)
}

func (a *App) replyOpResult(c *client, kind, note string, res opResult, err error) {
	a.command(c, kind, outcome(err), note, err)
	if err == nil {
		log.Printf("config: %s saved by %s", kind, c.who())
	}
	a.reply(c, res)
}

// opResultFor: the "<kind>Result" reply without the PIN-failure flag; replyOpNote adds it,
// and callers that log their own outcome (userSetPin) send it as is.
func opResultFor(kind string, id *int, err error) opResult {
	res := opResult{T: kind + "Result", OK: err == nil, ID: id}
	if err != nil {
		res.Err = err.Error()
		res.Recheck = isRecheckErr(err)
	}
	return res
}
