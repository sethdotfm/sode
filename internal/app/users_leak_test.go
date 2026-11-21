package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUsersListNeedsPin(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := admitMsg(a, c, inMsg{T: "usersList"}); err == nil {
		t.Fatal("usersList admitted for a Guest")
	}
	r := a.usersList(&client{}, 3)
	b, _ := json.Marshal(r)
	if !r.OK || r.ID != 3 || len(r.Users) != 3 || r.Users[0].Name != "Ana" || r.Users[0].Len != 4 || strings.Contains(string(b), `"pin":`) || strings.Contains(string(b), "pbkdf2") {
		t.Fatalf("list %s", b)
	}
}
