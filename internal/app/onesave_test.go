package app

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// saveFields sends the one Save with the versions from the peer's last hello; set maps a
// field to its new value.
func (p *wsPeer) saveFields(id int, set map[string]any) map[string]any {
	p.t.Helper()
	vers, _ := p.hello["versions"].(map[string]any)
	fields := map[string]any{}
	for k, v := range set {
		fields[k] = map[string]any{"value": v, "version": vers[k]}
	}
	return p.ask("settingsSave", id, map[string]any{"fields": fields})
}

func resultFor(t *testing.T, r map[string]any, key string) map[string]any {
	t.Helper()
	res, _ := r["results"].(map[string]any)
	f, _ := res[key].(map[string]any)
	if f == nil {
		t.Fatalf("no result for %s: %v", key, r)
	}
	return f
}

func wantSaved(t *testing.T, r map[string]any, key string) {
	t.Helper()
	if f := resultFor(t, r, key); f["status"] != "saved" {
		t.Fatalf("%s: want saved, got %v", key, f)
	}
}

func wantRefused(t *testing.T, r map[string]any, key, reason string) map[string]any {
	t.Helper()
	f := resultFor(t, r, key)
	if f["status"] != "refused" || f["reason"] != reason {
		t.Fatalf("%s: want refused with %q, got %v", key, reason, f)
	}
	return f
}

func TestOneSaveTrustedSavesBrandingNotLevelColors(t *testing.T) {
	a := userApp(t, false)
	setFlags(t, a, "Ana", true, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	r := p.saveFields(1, map[string]any{"brandText": "Studio B", "levelColors": []string{"#abcdef"}})
	wantSaved(t, r, "brandText")
	wantRefused(t, r, "levelColors", "needs Approval")
	got := a.r().cfg
	if got.BrandText != "Studio B" {
		t.Fatalf("brandText not saved: %q", got.BrandText)
	}
	if len(got.LevelColors) == 1 && got.LevelColors[0] == "#abcdef" {
		t.Fatal("a refused field was written")
	}
}

func TestConfigMessageKeepsSectionRoles(t *testing.T) {
	a := userApp(t, false)
	setFlags(t, a, "Ana", true, false)
	ana := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(ana, "1234"); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.LevelColors = []string{"#abcdef"}
	if err := a.SaveSettings(ana, cfg); !isRoleErr(err) {
		t.Fatalf("Trusted user saving Level colors: %v", err)
	}
	cfg = a.r().cfg
	cfg.BrandText = "Studio B"
	if err := a.SaveSettings(ana, cfg); err != nil {
		t.Fatalf("Trusted user saving Branding: %v", err)
	}
}

func TestOneSaveStandardUserGetsRoleForAccess(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	r := p.saveFields(1, map[string]any{"autoLockMinutes": 5, "brandText": "X"})
	if f := wantRefused(t, r, "autoLockMinutes", "Role"); f["value"] != nil {
		t.Fatalf("a Hidden field's value went to a Standard user: %v", f)
	}
	wantRefused(t, r, "brandText", "needs Approval")
	if got := a.r().cfg; got.AutoLockMinutes == 5 || got.BrandText == "X" {
		t.Fatalf("a refused field was written: %+v", got)
	}
}

func TestOneSaveConflictIsRefusedAlone(t *testing.T) {
	a := userApp(t, false)
	p1, p2 := dialPeer(t, a), dialPeer(t, a)
	p1.signIn("Ada", "2468")
	p2.signIn("Ada", "2468")
	wantSaved(t, p1.saveFields(1, map[string]any{"brandText": "First"}), "brandText")
	r := p2.saveFields(2, map[string]any{"brandText": "Second", "resultDismissSeconds": 9})
	f := wantRefused(t, r, "brandText", "conflict")
	if f["value"] != "First" {
		t.Fatalf("conflict does not carry the saved value: %v", f)
	}
	wantSaved(t, r, "resultDismissSeconds")
	if got := a.r().cfg; got.BrandText != "First" || got.ResultDismissSeconds != 9 {
		t.Fatalf("got brandText %q, resultDismissSeconds %d", got.BrandText, got.ResultDismissSeconds)
	}
	p2.hello["versions"].(map[string]any)["brandText"] = f["version"]
	wantSaved(t, p2.saveFields(3, map[string]any{"brandText": "Second"}), "brandText")
}

func TestOneSaveRefusesOnlyTheFieldsThatFailTogether(t *testing.T) {
	a := userApp(t, false)
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Users[1].Role = config.RoleAdministrator
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	ada, bo := "user/"+userID(t, a, "Ada")+"/role", "user/"+userID(t, a, "Bo")+"/role"
	r := saveLive(p, a, 1, map[string]any{ada: config.RoleStandard, bo: config.RoleStandard, "brandText": "X"}, nil)
	wantSaved(t, r, "brandText")
	wantRefused(t, r, ada, "invalid")
	wantRefused(t, r, bo, "invalid")
}

func TestOneSaveWritesOnlyChangedFields(t *testing.T) {
	a := userApp(t, false)
	p1, p2 := dialPeer(t, a), dialPeer(t, a)
	p1.signIn("Ada", "2468")
	p2.signIn("Ada", "2468")
	wantSaved(t, p1.saveFields(1, map[string]any{"brandText": "First"}), "brandText")
	wantSaved(t, p2.saveFields(2, map[string]any{"resultDismissSeconds": 9}), "resultDismissSeconds")
	if got := a.r().cfg.BrandText; got != "First" {
		t.Fatalf("a field not sent was written: %q", got)
	}
	before := a.r()
	r := p1.saveFields(3, map[string]any{"brandText": "First"})
	if f := resultFor(t, r, "brandText"); f["status"] != "unchanged" {
		t.Fatalf("want unchanged, got %v", f)
	}
	if a.r() != before {
		t.Fatal("a save with no change was written")
	}
}

func TestOneSaveCriticalFieldNeedsRecheck(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	r := p.saveFields(1, map[string]any{"autoLockMinutes": 5, "levelColors": []string{"#abcdef"}})
	wantRefused(t, r, "autoLockMinutes", "needs PIN recheck")
	wantSaved(t, r, "levelColors")
	passed(t, "recheck", p.recheck("2468"))
	wantSaved(t, p.saveFields(2, map[string]any{"autoLockMinutes": 5}), "autoLockMinutes")
	if got := a.r().cfg.AutoLockMinutes; got != 5 {
		t.Fatalf("autoLockMinutes %d", got)
	}
}

func TestOneSaveUnderLockIsRefused(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	p.send(map[string]any{"t": "userLock"})
	p.until("lockState")
	r := p.saveFields(1, map[string]any{"brandText": "X"})
	if e, _ := r["err"].(string); r["ok"] == true || !strings.Contains(e, "locked") {
		t.Fatalf("want a locked refusal: %v", r)
	}
	if a.r().cfg.BrandText == "X" {
		t.Fatal("saved under the Lock")
	}
}

// savedApart: Config JSON keys with their own messages, never the one Save's.
var savedApart = map[string]bool{"filterSets": true, "presets": true, "users": true, "bookmarks": true, "tieLines": true}

func TestEveryFormFieldHasSection(t *testing.T) {
	for _, f := range config.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" || savedApart[name] {
			continue
		}
		sec, ok := saveFields[name]
		if !ok {
			t.Errorf("%s (%s) has no section in saveFields", f.Name, name)
			continue
		}
		if _, ok := sectionRoles[sec]; !ok {
			t.Errorf("%s: section %q has no role tag", name, sec)
		}
	}
	for k := range saveFields {
		if _, hidden := hiddenFields[k]; hidden {
			continue
		}
		if _, ok := jsonFields()[k]; !ok {
			t.Errorf("saveFields key %q is not a Config JSON key", k)
		}
	}
}

func TestSectionRolesMatchPage(t *testing.T) {
	b, err := os.ReadFile("../../web/settingsrolerules.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	consts := map[string]sectionTag{}
	for _, m := range regexp.MustCompile(`const (\w+) = tag\('(\w+)', '(\w+)'\)`).FindAllStringSubmatch(src, -1) {
		consts[m[1]] = sectionTag{edit: m[2], shown: m[3] == "shown"}
	}
	start := strings.Index(src, "SECTION_ROLES = Object.freeze({")
	end := strings.Index(src[start:], "});")
	page := map[string]sectionTag{}
	for _, m := range regexp.MustCompile(`'?([\w-]+)'?: (\w+),`).FindAllStringSubmatch(src[start:start+end], -1) {
		tag, ok := consts[m[2]]
		if !ok {
			t.Fatalf("unknown tag %s", m[2])
		}
		page[m[1]] = tag
	}
	if len(page) == 0 || !reflect.DeepEqual(page, sectionRoles) {
		t.Fatalf("page %v\nserver %v", page, sectionRoles)
	}
}

// The page's copy of saveFields, and its reconnect list against the reconnect tags.
func TestSaveFieldsMatchPage(t *testing.T) {
	b, err := os.ReadFile("../../web/savereviewrules.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "SAVE_FIELDS = Object.freeze({")
	end := strings.Index(src[start:], "});")
	page := map[string]string{}
	for _, m := range regexp.MustCompile(`(\w+): '([\w-]+)',`).FindAllStringSubmatch(src[start:start+end], -1) {
		page[m[1]] = m[2]
	}
	if len(page) == 0 || !reflect.DeepEqual(page, saveFields) {
		t.Fatalf("page %v\nserver %v", page, saveFields)
	}
	if !strings.Contains(src, "ITEM_SECTIONS = Object.freeze({ user: '"+usersSection+"', client: '"+usersSection+"', "+swapKey+": '"+swapSpec().section+"', router: '"+routersSection+"' })") {
		t.Fatal("ITEM_SECTIONS in savereviewrules.js does not match the user/, client/, swap and router/ keys of onesavefields.go")
	}
	attrs := jsObject(t, src, "ROUTER_ATTRS")
	want := map[string]string{}
	for _, a := range routerAttrs {
		want[a.name] = a.section
	}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("page ROUTER_ATTRS %v\nserver routerAttrs %v", attrs, want)
	}
	m := regexp.MustCompile(`ROUTER_RECONNECT = /\^router\\/\[\^/\]\+\\/\(([^)]*)\)\$/`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no ROUTER_RECONNECT in savereviewrules.js")
	}
	reconnect := strings.Split(m[1], "|")
	var tagged []string
	for _, a := range routerAttrNames() {
		var r config.Router
		p := routerField(&r, a)
		v := reflect.ValueOf(&r).Elem()
		for i := range v.NumField() {
			if v.Field(i).Addr().Interface() == p && v.Type().Field(i).Tag.Get("reconnect") == "true" {
				tagged = append(tagged, a)
			}
		}
	}
	slices.Sort(reconnect)
	slices.Sort(tagged)
	if !slices.Equal(reconnect, tagged) {
		t.Fatalf("page reconnects %v, reconnect fields are %v", reconnect, tagged)
	}
}

// jsObject reads `name = Object.freeze({ key: 'value', ... })` of a page module: each key with its string value.
func jsObject(t *testing.T, src, name string) map[string]string {
	t.Helper()
	start := strings.Index(src, name+" = Object.freeze({")
	if start < 0 {
		t.Fatalf("no %s in the page", name)
	}
	end := strings.Index(src[start:], "});")
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`(\w+): '([\w-]+)',`).FindAllStringSubmatch(src[start:start+end], -1) {
		out[m[1]] = m[2]
	}
	return out
}

// A setting's name in Settings saves (settingFields) is its label on the Settings form, where the form has one.
func TestSettingFieldLabelsMatchForm(t *testing.T) {
	b, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	checked := 0
	for _, f := range settingFields {
		m := regexp.MustCompile(`id="([\w-]+)"[^>]*data-key="` + f.key + `"[^>]*>(?:<span class="pb-check__label">([^<]*)<)?`).FindStringSubmatch(page)
		if m == nil {
			continue
		}
		label := m[2]
		if label == "" {
			l := regexp.MustCompile(`<label for="` + m[1] + `">([^<]*)</label>`).FindStringSubmatch(page)
			if l == nil {
				continue
			}
			label = l[1]
		}
		checked++
		if label != f.label {
			t.Errorf("%s: settingFields says %q, the form %q", f.key, f.label, label)
		}
	}
	if checked < 10 {
		t.Errorf("only %d labels found on the form", checked)
	}
}
