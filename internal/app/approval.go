package app

import (
	"cmp"
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// Approval in person (GLOSSARY.md, docs/agents/users-locks.md): a message carries the approver's
// name and PIN, and the server checks that one action against the approver's Role. Nothing stays
// on the connection, so the next message is under the browser's own sign-in again.

type approvalReq struct {
	User string `json:"user"`
	Pin  string `json:"pin"`
}

// approvalErr: the Approval itself is refused (wrong PIN, the approver's Role). The page keeps its
// Approval popup open and shows it there.
type approvalErr struct{ msg string }

func (e approvalErr) Error() string { return e.msg }

func isApprovalErr(err error) bool {
	var e approvalErr
	return errors.As(err, &e)
}

var errApprovalTake = approvalErr{"takes use this browser's own sign-in: an Approval does not cover them"}

// approve checks m's Approval and sets m.approver. The browser's own state comes first, so a
// browser under the Lock spends no PIN try. Wrong PINs count toward the approver name's wait.
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
	if err := a.tryUserPin(hit, c.ip, func() bool { return pinMatches(hit, m.Approval.Pin) }); err != nil {
		return approvalErr{err.Error()}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	live, ok := a.r().cfg.FindUserByID(hit.ID) // the PIN check can wait
	if !ok || live.Pin != hit.Pin {
		return approvalErr{"that PIN changed; try again"}
	}
	m.approver = &live
	return nil
}

// approverRole: the approver's live Role; "" when the approver is gone or changed their PIN.
func approverRole(cfg config.Config, ap *config.User) string {
	u, ok := cfg.FindUserByID(ap.ID)
	if !ok || u.Pin != ap.Pin {
		return ""
	}
	role, _ := cfg.ActsAs(u)
	return cmp.Or(role, config.RoleStandard)
}

// cannotApprove names the approver and the Role the action needs.
func cannotApprove(ap *config.User, need string) error {
	who := "an Administrator"
	if need != config.RoleAdministrator {
		who = "a " + config.RoleLabel(need) + " or an Administrator"
	}
	return approvalErr{ap.Name + " cannot approve this: it needs " + who}
}

// approvedNote: the Communication Log names the approver beside the browser's user.
func approvedNote(ap *config.User, note string) string {
	if ap == nil {
		return note
	}
	if note == "" {
		return "approved by " + ap.Name
	}
	return note + ", approved by " + ap.Name
}
