package app

import (
	"errors"
	"testing"
)

// Connection states the matrix runs every rule against. Terms follow the Go code: a guest
// is nobody signed in on an app with the locks off.
const (
	stNoAccess = iota // locks on, nobody signed in
	stGuest           // locks off, nobody signed in
	stSignedIn        // Ana, untrusted
	stTrusted         // Ana, trusted
	stLocked          // Ana signed in, then the held Lock
	stPin             // Advanced Settings PIN entered, nobody signed in, locks on
	stPinAna          // PIN entered and Ana signed in
	nStates
)

var stateNames = [nStates]string{"noAccess", "guest", "signedIn", "trusted", "locked", "pin", "pinAna"}

func stateClient(t *testing.T, st int) (*App, *client) {
	t.Helper()
	a := userApp(t, st != stGuest)
	c := &client{ip: "10.0.0.1"}
	if st == stTrusted {
		setFlags(t, a, "Ana", true, false)
	}
	switch st {
	case stSignedIn, stTrusted, stLocked, stPinAna:
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
	}
	if st == stLocked {
		a.UserLock(c)
	}
	if st == stPin || st == stPinAna {
		if err := a.Unlock(c, "2468"); err != nil {
			t.Fatal(err)
		}
	}
	return a, c
}

var (
	pl, lk = errPageLocked, errLocked
	// per-state results, in state order
	openRule    = [nStates]error{nil, nil, nil, nil, nil, nil, nil}
	mainRule    = [nStates]error{pl, nil, nil, nil, pl, pl, nil}
	trustedRule = [nStates]error{lk, lk, lk, nil, pl, nil, nil}
	adminRule   = [nStates]error{lk, lk, lk, lk, pl, nil, nil}
	// needMain is checked before needAdmin, so the page lock shows first
	mainAdminRule = [nStates]error{pl, lk, lk, lk, pl, pl, nil}
	selfRule      = [nStates]error{lk, lk, nil, nil, pl, nil, nil}
	otherRule     = [nStates]error{lk, lk, lk, nil, pl, nil, nil}
)

// One row per policy rule. A rule added to or removed from the table fails until its row
// follows. profileSave runs with Name "Ana" here; the Name branches are below.
var policyExpect = map[string][nStates]error{
	"hi": openRule, "userUnlock": openRule, "userLock": openRule, "userSignOut": openRule,
	"userSetPin": openRule, "unlock": openRule, "lock": openRule, "history": openRule,
	"keyCreate": openRule, "keyRevoke": openRule, "keyListOwn": openRule,
	"sessionKey": openRule, "panelAlarm": openRule, "activity": openRule,

	"batch": mainRule, "refreshNames": mainRule, "sent": mainRule, "uptime": mainRule,
	"clearHistory": mainAdminRule,

	"config": trustedRule, "filterSets": trustedRule, "presets": trustedRule,
	"profileGet": trustedRule, "profileSave": selfRule,

	"configExport": adminRule, "configImportBegin": adminRule, "configImport": adminRule,
	"userSignOutAll": adminRule, "usersList": adminRule, "users": adminRule,
	"apiSettings": adminRule, "keyListAll": adminRule, "keyCreateFor": adminRule,
	"keyRevokeAny": adminRule, "keyRevokeAll": adminRule, "kill": adminRule,
}

func sameRefusal(got, want error) bool {
	if want == nil {
		return got == nil
	}
	return errors.Is(got, want)
}

func TestPolicyMatrix(t *testing.T) {
	for m := range policy {
		if _, ok := policyExpect[m]; !ok {
			t.Errorf("policy rule %q has no row in policyExpect", m)
		}
	}
	for m := range policyExpect {
		if _, ok := policy[m]; !ok {
			t.Errorf("policyExpect row %q has no policy rule", m)
		}
	}
	for st := range nStates {
		t.Run(stateNames[st], func(t *testing.T) {
			a, c := stateClient(t, st)
			for m, want := range policyExpect {
				if _, ok := policy[m]; !ok {
					continue
				}
				msg := inMsg{T: m}
				if m == "profileSave" {
					msg.Name = "Ana"
				}
				if err := admitMsg(a, c, msg); !sameRefusal(err, want[st]) {
					t.Errorf("%s: got %v, want %v", m, err, want[st])
				}
			}
		})
	}
}

// The branches that depend on the message body: config turns admin-only when it sets or
// clears the PIN, and profileSave passes for the signed-in user's own name.
func TestPolicyMatrixMessageBranches(t *testing.T) {
	for _, r := range []struct {
		name string
		msg  inMsg
		want [nStates]error
	}{
		{"config plain", inMsg{T: "config"}, trustedRule},
		{"config sets PIN", inMsg{T: "config", Pin: "9999"}, adminRule},
		{"config clears PIN", inMsg{T: "config", ClearPin: true}, adminRule},
		{"profileSave own", inMsg{T: "profileSave", Name: "Ana"}, selfRule},
		{"profileSave other", inMsg{T: "profileSave", Name: "Bo"}, otherRule},
	} {
		for st := range nStates {
			t.Run(r.name+"/"+stateNames[st], func(t *testing.T) {
				a, c := stateClient(t, st)
				if err := admitMsg(a, c, r.msg); !sameRefusal(err, r.want[st]) {
					t.Errorf("got %v, want %v", err, r.want[st])
				}
			})
		}
	}
}
