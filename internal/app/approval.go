package app

import (
	"cmp"
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// Approval in person (GLOSSARY.md, docs/agents/users-locks.md). Nothing stays on the connection, so the next message
// is under the browser's own sign-in again.

type approvalReq struct {
	User string `json:"user"`
	Pin  string `json:"pin"`
}

// The Approval itself is refused (wrong PIN, the approver's Role). The page shows it in the open Approval popup.
type approvalErr struct{ msg string }

func (e approvalErr) Error() string { return e.msg }

func isApprovalErr(err error) bool {
	var e approvalErr
	return errors.As(err, &e)
}

var errApprovalTake = approvalErr{"takes use this browser's own sign-in: an Approval does not cover them"}

// The browser's own state is checked first, so a browser under the Lock uses no PIN try. A wrong PIN counts toward
// the wait of the approver's name.
func (a *App) approve(c *client, r rule, m *inMsg) error {
	switch {
	case m.T == "take":
		return errApprovalTake
	case !r.approvable:
		return approvalErr{"an Approval does not cover this"}
	}
	a.mu.Lock()
	open := !c.u.lockedMu() && a.roleMu(c) != ""
	cfg := a.r().cfg
	a.mu.Unlock()
	if !open {
		return approvalErr{"dismiss the Lock first: an Approval needs it dismissed"}
	}
	hit := pinCandidate(cfg, nil, m.Approval.User)
	if hit != nil && !hit.HasPin() {
		return approvalErr{hit.Name + " has no PIN: only a user with a PIN can approve"}
	}
	if err := a.provePin(c, nil, hit, m.Approval.Pin); err != nil {
		return approvalErr{err.Error()}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	live, ok := a.livePinMu(hit)
	if !ok {
		return approvalErr{"that PIN changed; try again"}
	}
	m.approver = &live
	return nil
}

// "" when the approver is gone or changed their PIN.
func approverRole(cfg config.Config, ap *config.User) string {
	u, ok := cfg.FindUserByID(ap.ID)
	if !ok || u.Pin != ap.Pin {
		return ""
	}
	role, _ := cfg.ActsAs(u)
	return cmp.Or(role, config.RoleStandard)
}

func cannotApprove(ap *config.User, need string) error {
	who := "an Administrator"
	if need != config.RoleAdministrator {
		who = "a " + config.RoleLabel(need) + " or an Administrator"
	}
	return approvalErr{ap.Name + " cannot approve this: it needs " + who}
}

func approvedNote(ap *config.User, note string) string {
	if ap == nil {
		return note
	}
	if note == "" {
		return "approved by " + ap.Name
	}
	return note + ", approved by " + ap.Name
}
