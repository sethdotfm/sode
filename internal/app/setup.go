package app

import (
	"errors"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// With a config path and no users, Sode admits only the setup messages until Finish, and writes nothing to disk
// before it (docs/agents/users-locks.md).

//lint:ignore ST1005 the page shows it as is
var errSetup = errors.New("Sode is not set up yet")

//lint:ignore ST1005 the page shows it as is
var errSetUpDone = errors.New("Sode is already set up")

func (a *App) setupPendingCfg(cfg config.Config) bool {
	return a.cfgPath != "" && len(cfg.Users) == 0
}

func (a *App) setupPending() bool { return a.setupPendingCfg(a.r().cfg) }

// setupRows: each preset's Yes or No for the rows of the Access step. The page holds the words.
func setupRows(v config.AccessValues) map[string]bool {
	return map[string]bool{
		"takeNoSignIn":   !v.Locks.Main,
		"sharedNoSignIn": v.GuestRole == config.RoleTrusted,
		"panelLinks":     v.PublicPanels,
		"appsRead":       v.KeylessAPI && slices.Contains(v.KeylessScopes, config.ScopeNames),
		"appsTake":       v.KeylessAPI && slices.Contains(v.KeylessScopes, config.ScopeTake),
		"pinWaits":       v.PinWaits,
	}
}

type setupPresetView struct {
	Key  string          `json:"key"`
	Name string          `json:"name"`
	Rows map[string]bool `json:"rows"`
}

// setupHello rides the hello only while setup is pending. Preset is SODE_ACCESS_PRESET's key on
// first start, else Standard access. Env: setupEnv, for the "Filled from" hints.
type setupHello struct {
	Preset  string            `json:"preset"`
	Presets []setupPresetView `json:"presets"`
	Env     map[string]string `json:"env,omitempty"`
}

func (a *App) setupHelloFor(cfg config.Config) *setupHello {
	if !a.setupPendingCfg(cfg) {
		return nil
	}
	h := &setupHello{Preset: "standard", Env: a.setupEnv()}
	if p, _ := config.AccessPresetFromEnv(); p != nil && h.Env["preset"] != "" {
		h.Preset = p.Key
	}
	for _, p := range config.AccessPresets {
		h.Presets = append(h.Presets, setupPresetView{p.Key, p.Name, setupRows(p.Values)})
	}
	return h
}

// setupReq: the wizard's answers, sent one time at Finish.
type setupReq struct {
	BrandText      string `json:"brandText"`
	BrandBorder    string `json:"brandBorder"`
	Name           string `json:"name"`
	Pin            string `json:"pin"`
	Router         string `json:"router"`
	Levels         int    `json:"levels"`
	LevelNumbering string `json:"levelNumbering"`
	NumberOffset   int    `json:"numberOffset"`
	FireMode       string `json:"fireMode"`
	SalvoGroup     int    `json:"group"`
	Preset         string `json:"preset"`
}

// setupKeys: the one Save keys Finish writes, for the Settings saves row.
var setupKeys = []string{"brandText", "brandBorder", "router", "levels", "levelNumbering", "numberOffset",
	"fireMode", "salvoGroup", "locks", "autoLockMinutes", "guestRole", "pinWaits", "publicPanels",
	"api", "guestApiScopes", "newPanelActs", "newUserDefaults"}

// setupHold: the one browser that runs setup. addr is the address before its Router step moved
// the link. Sode puts it back when that browser closes or loses control, since its answers go.
type setupHold struct {
	owner *client
	addr  string
	moved bool
	key   string // for POST /api/setup/restore (setuprestore.go); a new owner gets a new one
}

var errSetupElsewhere = errors.New("another browser runs setup now: take control to run it here")

// setupOwnsLocked: the first browser to claim or act runs setup. Caller holds cfgMu.
func (a *App) setupOwnsLocked(c *client) bool {
	if a.setup.owner == nil {
		a.setup.owner = c
	}
	return a.setup.owner == c
}

// Caller holds cfgMu.
func (a *App) setupDropLocked() {
	if !a.setup.moved {
		return
	}
	addr := a.setup.addr
	a.setup.moved = false
	if err := a.applyConfigWith(func(cfg *config.Config) error {
		cfg.Address = addr
		return nil
	}, false); err != nil {
		log.Printf("setup: put back the router address: %v", err)
	}
}

// setupClaim: the page asks to run setup. take is Take control: the browser that ran it is told
// and its answers are dropped. A phone never sends it (web/setupwizard.js).
func (a *App) setupClaim(c *client, take bool) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.setupPending() {
		return errSetUpDone
	}
	was := a.setup.owner
	if a.setupOwnsLocked(c) {
		return nil
	}
	if !take {
		return errSetupElsewhere
	}
	a.setup.owner, a.setup.key = c, ""
	a.setupDropLocked()
	a.reply(was, struct {
		T string `json:"t"`
	}{"setupTaken"})
	log.Printf("setup: %s took control from %s", c.ip, was.ip)
	return nil
}

func (a *App) setupLeft(c *client) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.setup.owner != c {
		return
	}
	a.setup.owner, a.setup.key = nil, ""
	if !a.setupPending() {
		return
	}
	a.setupDropLocked()
	free := []byte(`{"t":"setupFree"}`)
	a.mu.Lock()
	for cl := range a.clients {
		a.deliver(cl, free)
	}
	a.mu.Unlock()
}

// setupRouter moves the live link to the address from the Router step and does not save.
// The page can then say if the router answers.
func (a *App) setupRouter(c *client, addr string) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.setupPending() {
		return errSetUpDone
	}
	if !a.setupOwnsLocked(c) {
		return errSetupElsewhere
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return errors.New("enter the router's address")
	}
	was := a.r().cfg.Address
	err := a.applyConfigWith(func(c *config.Config) error {
		c.Address = addr
		return nil
	}, false)
	if err == nil && !a.setup.moved {
		a.setup.addr, a.setup.moved = was, true
	}
	return err
}

const setupAddKey = "user/new:0/add"

func (a *App) setupFinish(c *client, s setupReq) (config.User, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.setupPending() {
		//lint:ignore ST1005 the page shows it as is
		return config.User{}, errors.New("Sode is already set up: sign in instead")
	}
	if !a.setupOwnsLocked(c) {
		return config.User{}, errSetupElsewhere
	}
	preset, err := config.AccessPresetByName(s.Preset)
	if err != nil {
		return config.User{}, err
	}
	name := strings.TrimSpace(s.Name)
	if err := userNameRefusal(name); err != nil {
		return config.User{}, err
	}
	if strings.TrimSpace(s.Router) == "" {
		return config.User{}, errors.New("enter the router's address")
	}
	was := a.r().cfg
	if err := config.CheckUserPinRules(s.Pin, was.MaxPinLen); err != nil {
		return config.User{}, err
	}
	hash, err := config.HashPin(s.Pin)
	if err != nil {
		return config.User{}, err
	}
	u := config.User{ID: config.NewUserID(), Name: name, Role: config.RoleAdministrator, Pin: hash, Len: len(s.Pin)}
	err = a.applyConfig(func(cfg *config.Config) error {
		cfg.BrandText, cfg.BrandBorder = s.BrandText, s.BrandBorder
		cfg.Address = strings.TrimSpace(s.Router)
		if cfg.Levels != s.Levels {
			cfg.DefaultLevels = "" // Normalize fills it for the new count
		}
		cfg.Levels, cfg.LevelNumbering, cfg.NumberOffset = s.Levels, s.LevelNumbering, s.NumberOffset
		cfg.FireMode, cfg.SalvoGroup = s.FireMode, s.SalvoGroup
		preset.Apply(cfg)
		u.Persistent, u.NoAutoLock = cfg.NewUserDefaults.Persistent, cfg.NewUserDefaults.NoAutoLock
		cfg.Users = []config.User{u}
		return nil
	})
	if err != nil {
		return config.User{}, err
	}
	a.setup = setupHold{}
	after := a.r().cfg
	a.recordSetup(c, was, after, u.ID)
	a.mu.Lock()
	src, dst := maps.Clone(a.router.srcNames), maps.Clone(a.router.dstNames)
	signed, _ := after.FindUserByID(u.ID)
	c.u.sess = &session{seen: a.now()}
	c.u.recheck = recheck{}
	c.u.user.Store(&signed)
	a.syncAccessMu(c)
	a.mu.Unlock()
	// flush writes no names table while setup is pending.
	if len(src)+len(dst) > 0 {
		a.names.save(after.UID, a.now(), src, dst)
	}
	log.Printf("setup: finished, %s is the first Administrator", name)
	return signed, nil
}

func (a *App) recordSetup(c *client, was, now config.Config, id string) {
	var fields []SavedField
	for _, key := range setupKeys {
		spec, _ := lookupField(key)
		f := newSavedField(key, spec.section, fieldLabels[key])
		if spec.secret {
			f.Changed = true
		} else {
			f.New = fieldJSON(now, key)
		}
		fields = append(fields, f)
	}
	add := newSavedField(setupAddKey, usersSection, "")
	fields = append(fields, a.addedFields(add, setupAddKey, now, &saveCtx{made: map[string]string{setupAddKey: id}})...)
	actor := connActor(c)
	if u, ok := now.FindUserByID(id); ok {
		actor.Name, actor.User = u.Name, u.ID
	}
	a.recordSave(actor, "setup", fields, len(criticalEdits(was, now, config.User{})) > 0)
}

func (a *App) serveSetupRouter(c *client, m *inMsg) {
	err := a.setupRouter(c, m.Router)
	a.command(c, "setupRouter", outcome(err), "", err)
	a.reply(c, opResultFor("setupRouter", &m.ID, err))
}

func (a *App) serveSetupFinish(c *client, m *inMsg) {
	defer m.finish()
	res := userResult{T: "setupFinishResult", OK: true, ID: m.ID}
	var s setupReq
	if m.Setup != nil {
		s = *m.Setup
	}
	u, err := a.setupFinish(c, s)
	if err != nil {
		res.OK, res.Err = false, err.Error()
		a.command(c, "setupFinish", "refused", "", err)
		a.reply(c, res)
		return
	}
	a.command(c, "setupFinish", "ok", "", nil)
	res.Token = a.issueToken(c, true)
	res.Tab = res.Token != "" && !u.Persistent
	a.mu.Lock()
	a.syncAccessMu(c)
	a.mu.Unlock()
	a.reply(c, res)
}

func (a *App) serveSetupClaim(c *client, m *inMsg) {
	err := a.setupClaim(c, m.TakeOver)
	if m.TakeOver {
		a.command(c, "setupClaim", outcome(err), "take control", err)
	}
	a.reply(c, opResultFor("setupClaim", &m.ID, err))
}
