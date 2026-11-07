package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The hello goes to every browser, locked or not: it must carry no user.
func TestHelloHasNoUsers(t *testing.T) {
	a := userApp(t, true)
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	for _, s := range []string{"Ana", "Bo", "trusted", "persistent"} {
		if strings.Contains(string(b), `"`+s+`"`) {
			t.Fatalf("hello mentions %q: %s", s, b)
		}
	}
	var h struct {
		Cfg struct {
			Users []any `json:"users"`
		} `json:"cfg"`
	}
	if err := json.Unmarshal(b, &h); err != nil || len(h.Cfg.Users) != 0 {
		t.Fatalf("hello users %v (%v)", h.Cfg.Users, err)
	}
}

func TestLockStateNames(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	a.mu.Lock()
	m := a.lockStateLocked(c)
	a.mu.Unlock()
	if !m.Locked || len(m.Names) != 0 {
		t.Fatalf("locked page got %+v", m)
	}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	m = a.lockStateLocked(c)
	a.mu.Unlock()
	if m.Locked || !reflect.DeepEqual(m.Names, []string{"Ana", "Bo"}) {
		t.Fatalf("unlocked page got %+v", m)
	}
}

func TestUsersListNeedsPin(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := admitMsg(a, c, inMsg{T: "usersList"}); err == nil {
		t.Fatal("usersList admitted without the Advanced Settings PIN")
	}
	r := a.usersList(3)
	b, _ := json.Marshal(r)
	if !r.OK || r.ID != 3 || len(r.Users) != 2 || r.Users[0].Name != "Ana" || r.Users[0].Len != 4 || strings.Contains(string(b), "pin") {
		t.Fatalf("list %s", b)
	}
}
