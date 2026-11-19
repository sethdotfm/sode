package app

import (
	"errors"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// The setup wizard (web/setupwizard.js). With a config path and no users, Sode takes only the
// setup messages until Finish, and nothing is on disk before it. Env-only mode has no wizard;
// roleMu's interim rule still covers it.

//lint:ignore ST1005 the page shows it as is
var errSetup = errors.New("Sode is not set up yet: finish the setup on this page first")

// setupPendingCfg: a config file path and no users, whether the file exists or not.
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

// setupHello rides the hello only while setup is pending. Preset is SODE_ACCESS_PRESET's key,
// else Standard access.
type setupHello struct {
	Preset  string            `json:"preset"`
	Presets []setupPresetView `json:"presets"`
}

func (a *App) setupHelloFor(cfg config.Config) *setupHello {
	if !a.setupPendingCfg(cfg) {
		return nil
	}
	h := &setupHello{Preset: "standard"}
	if p, _ := config.AccessPresetFromEnv(); p != nil {
		h.Preset = p.Key
	}
	for _, p := range config.AccessPresets {
		h.Presets = append(h.Presets, setupPresetView{p.Key, p.Name, setupRows(p.Values)})
	}
	return h
}

// setupReq: the wizard's answers, sent once at Finish.
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

// setupRouter moves the live link to the address the Router step typed, without saving, so the
// page can say whether it answers.
func (a *App) setupRouter(addr string) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.setupPending() {
		//lint:ignore ST1005 the page shows it as is
		return errors.New("Sode is already set up")
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return errors.New("enter the router's address")
	}
	return a.applyConfigWith(func(c *config.Config) error {
		c.Address = addr
		return nil
	}, false)
}

const setupAddKey = "user/new:0/add"

// setupFinish writes every answer in one save, then signs c in as the new Administrator.
func (a *App) setupFinish(c *client, s setupReq) (config.User, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.setupPending() {
		//lint:ignore ST1005 the page shows it as is
		return config.User{}, errors.New("Sode is already set up: sign in instead")
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
	// flush kept the names table back until now (setupPending).
	if len(src)+len(dst) > 0 {
		a.names.save(after.UID, a.now(), src, dst)
	}
	log.Printf("setup: finished, %s is the first Administrator", name)
	return signed, nil
}

// recordSetup: one Settings saves row with every value new.
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
	err := a.setupRouter(m.Router)
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
