package app

import (
	"errors"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Connection states the matrix runs every rule against. A Guest is nobody signed in on an
// app with the locks off.
const (
	stNoAccess     = iota // locks on, nobody signed in
	stGuest               // locks off, nobody signed in, Guest role Standard user
	stGuestTrusted        // the same with Guest role Trusted user
	stStandard            // Ana, a Standard user
	stTrusted             // Ana, a Trusted user
	stAdmin               // Ada, an Administrator
	stLocked              // Ada signed in, then the held Lock
	stNoUsers             // no users: every browser acts as an Administrator
	nStates
)

var stateNames = [nStates]string{"noAccess", "guest", "guestTrusted", "standard", "trusted", "administrator", "locked", "noUsers"}

func stateClient(t *testing.T, st int) (*App, *client) {
	t.Helper()
	if st == stNoUsers {
		return newTestApp(t), &client{ip: "10.0.0.1"}
	}
	a := userApp(t, st == stNoAccess || st == stLocked)
	c := &client{ip: "10.0.0.1"}
	switch st {
	case stGuestTrusted:
		cfg := a.r().cfg.Clone()
		cfg.GuestRole = config.RoleTrusted
		if err := a.ApplyConfig(cfg); err != nil {
			t.Fatal(err)
		}
	case stTrusted:
		setFlags(t, a, "Ana", true, false)
	}
	pin := map[int]string{stStandard: "1234", stTrusted: "1234", stAdmin: "2468", stLocked: "2468"}[st]
	if pin != "" {
		if err := a.UserUnlock(c, pin); err != nil {
			t.Fatal(err)
		}
	}
	if st == stLocked {
		a.UserLock(c)
	}
	return a, c
}

var (
	pl = errPageLocked
	ro = roleErr{} // any Role refusal
	// per-state results, in state order
	openRule    = [nStates]error{nil, nil, nil, nil, nil, nil, nil, nil}
	mainRule    = [nStates]error{pl, nil, nil, nil, nil, nil, pl, nil}
	trustedRule = [nStates]error{pl, ro, nil, ro, nil, nil, pl, nil}
	adminRule   = [nStates]error{pl, ro, ro, ro, ro, nil, pl, nil}
	selfRule    = [nStates]error{pl, ro, nil, nil, nil, nil, pl, nil}
)

// One row per policy rule. A rule added to or removed from the table fails until its row
// follows. profileSave runs with Name "Ana" here; the Name branches are below.
var policyExpect = map[string][nStates]error{
	"hi": openRule, "userSignIn": openRule, "userLock": openRule, "userSignOut": openRule,
	"userSetPin": openRule, "unlock": openRule, "lock": openRule, "history": openRule,
	"keyCreate": openRule, "keyRevoke": openRule, "keyListOwn": openRule,
	"myClients": openRule, "myClientSave": openRule, "myClientDelete": openRule, "myClientKey": openRule, "myClientKeyRevoke": openRule,
	"sessionKey": openRule, "panelAlarm": openRule, "activity": openRule,

	"take": mainRule, "rereadNames": mainRule, "log": mainRule, "uptime": mainRule,

	"config": trustedRule, "filterSets": trustedRule, "presets": trustedRule,
	"profileGet": trustedRule, "profileSave": selfRule,

	"clearHistory": adminRule,
	"configExport": adminRule, "configImportBegin": adminRule, "configImport": adminRule,
	"userSignOutAll": adminRule, "usersList": adminRule, "users": adminRule,
	"apiSettings": adminRule, "keyListAll": adminRule, "keyCreateFor": adminRule,
	"keyRevokeAny": adminRule, "keyRevokeAll": adminRule,
}

func sameRefusal(got, want error) bool {
	switch {
	case want == nil:
		return got == nil
	case want == ro:
		return isRoleErr(got)
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

// The branches that depend on the message body: config needs an Administrator when it sets or
// clears the PIN. profileSave passes for the signed-in user's own name.
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
		{"profileSave other", inMsg{T: "profileSave", Name: "Bo"}, trustedRule},
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
