package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

func (c Config) Validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	errs = append(errs, c.validateRouters()...)
	switch c.RouterShow {
	case "address", "name", "none":
	default:
		bad("router status must show address, name or none")
	}
	if !slices.Contains(PanelActs, c.NewPanelActs) {
		bad("the new-panel Authentication mode is session, api, both or public")
	}
	if c.ConfirmLockoutMs < 0 || c.ConfirmLockoutMs > 10000 {
		bad("confirm lockout must be 0-10000 ms")
	}
	if c.ResultDismissSeconds < 0 || c.ResultDismissSeconds > 3600 {
		bad("result dismiss must be 0 (never) or 1-3600 seconds")
	}
	if c.TieHopWeight < 0 || c.TieHopWeight > MaxTieHopWeight {
		bad("the hop weight must be 0-%d", MaxTieHopWeight)
	}
	if n := len([]rune(c.BrandText)); n > 24 {
		bad("logo text must be at most 24 characters")
	}
	if _, ok := BrandHex(c.BrandBorder); c.BrandBorder != "" && !ok {
		bad("border colour must be one of %s, or #rrggbb", strings.Join(BrandNames(), ", "))
	}
	for _, e := range deviceEnums {
		if !slices.Contains(PrefValues[e.pref], *e.get(&c.DeviceDefaults)) {
			errs = append(errs, errors.New(e.msg))
		}
	}
	for i, v := range c.LevelColors {
		if h, ok := strings.CutPrefix(v, "#"); v != "" && (!ok || len(h) != 6 || strings.Trim(h, "0123456789abcdef") != "") {
			bad("level %d color %q must be a hex color like #3a7bd5", i+1, v)
		}
	}
	if n := len([]rune(c.HiddenDsts)); n > 500 {
		bad("globally hidden destinations must be at most 500 characters")
	}
	errs = append(errs, validateFilterSets(c.FilterSets)...)
	errs = append(errs, c.validatePresets()...)
	errs = append(errs, validateBookmarks(c.Bookmarks)...)
	errs = append(errs, c.validateTieLines()...)
	if _, err := c.Proxies(); err != nil {
		bad("trusted proxies: %v", err)
	}
	errs = append(errs, c.validateUsers()...)
	errs = append(errs, c.validateAPI()...)
	errs = append(errs, c.validateProfiles()...)
	return errors.Join(errs...)
}

func (c Config) Proxies() ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, s := range c.TrustedProxies {
		if p, err := netip.ParsePrefix(s); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("%q is not an IP or CIDR", s)
		}
		out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return out, nil
}

func ParseLevels(s string, max int) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		a, b, isRange := strings.Cut(part, "-")
		lo, err1 := strconv.Atoi(a)
		hi := lo
		var err2 error
		if isRange {
			hi, err2 = strconv.Atoi(b)
		}
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("%q is not a level or range", part)
		}
		if lo > hi {
			lo, hi = hi, lo
		}
		if lo < 1 || hi > max {
			return nil, fmt.Errorf("%q is outside 1-%d", part, max)
		}
		for i := lo; i <= hi; i++ {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no levels")
	}
	return out, nil
}

// validateRouters checks each router and that no two share a UID or a prefix. With two or more routers, each problem
// names its router by prefix and Router title, the first router too.
func (c Config) validateRouters() []error {
	var errs []error
	uids, prefixes := map[string]bool{}, map[string]bool{}
	routers := c.Routers
	for _, r := range routers {
		name := ""
		if len(routers) > 1 {
			name = r.Display() + ": "
		}
		for _, err := range r.problems() {
			errs = append(errs, fmt.Errorf("%s%w", name, err))
		}
		if r.UID != "" && uids[r.UID] {
			errs = append(errs, fmt.Errorf("%stwo routers have the UID %s", name, r.UID))
		}
		if prefixes[r.Prefix] {
			errs = append(errs, fmt.Errorf("%stwo routers have the prefix %s", name, r.Prefix))
		}
		uids[r.UID], prefixes[r.Prefix] = true, true
	}
	return errs
}

func (r Router) problems() []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if r.Address != "" {
		host, port, err := net.SplitHostPort(r.Address)
		if p, perr := strconv.Atoi(port); err != nil || host == "" || perr != nil || p < 1 || p > 65535 {
			bad("address must be host:port, for example 10.0.0.50:8910")
		}
	}
	switch r.NameRequests {
	case "std":
		// Standard messages carry the matrix in a 4-bit field.
		if r.Matrix < 0 || r.Matrix > 15 {
			bad("matrix must be 0-15 with standard name requests")
		}
	case "ext":
		if r.Matrix < 0 || r.Matrix > 255 {
			bad("matrix must be 0-255")
		}
	default:
		bad("name requests must be std or ext")
	}
	switch r.FireMode {
	case "sequential":
	case "native":
		// Group salvos carry the matrix in a 4-bit field.
		if r.Matrix < 0 || r.Matrix > 15 {
			bad("native salvos need matrix 0-15")
		}
	default:
		bad("fire mode must be sequential or native")
	}
	if r.SalvoGroup < 0 || r.SalvoGroup > 127 {
		bad("salvo group must be 0-127")
	}
	// The dump packs the device in 12 bits. Not proved on the router: whether other messages split at 128 or 256.
	if r.ProtectDevice < 0 || r.ProtectDevice > MaxProtectDevice {
		bad("Protect device number must be 0 (off) to %d", MaxProtectDevice)
	}
	if r.Levels < 1 || r.Levels > 256 {
		bad("levels must be 1-256")
	} else if lo, hi := 1+r.LevelWireOffset, r.Levels+r.LevelWireOffset; lo < 0 || hi > 255 {
		bad("level wire offset puts levels at wire %d-%d; they must fall within 0-255", lo, hi)
	} else if _, err := ParseLevels(r.DefaultLevels, r.Levels); r.DefaultLevels != "" && err != nil {
		bad("default levels: %v", err)
	}
	switch r.NameLength {
	case 4, 8, 12, 16:
	default:
		bad("name length must be 4, 8, 12 or 16")
	}
	if r.NumberOffset < 0 || r.NumberOffset > 1 {
		bad("number offset must be 0 or 1")
	}
	if s := r.NameRefreshSeconds; s != 0 && (s < 10 || s > 86400) {
		bad("name refresh must be 0 (off) or 10-86400 seconds")
	}
	if s := r.TallyResyncSeconds; s != 0 && (s < 30 || s > 86400) {
		bad("tally resync must be 0 (off) or 30-86400 seconds")
	}
	switch r.LevelNumbering {
	case "0", "1", "va":
	default:
		bad("level numbering must be 0, 1 or va")
	}
	if !validPrefix(r.Prefix) {
		bad("router prefix must be one or two letters, A to Z")
	}
	if n := len([]rune(r.Title)); n > 32 {
		bad("router title must be at most 32 characters")
	}
	return errs
}

// Display is the one shown form of a router: the Router prefix and then the Router title ("B Studio B"). A router with
// no title, or one that only repeats the default ("Router B"), is "Router B". Keep in step with routerDisplay in
// web/routerlistrules.js.
func (r Router) Display() string {
	title := strings.TrimSpace(r.Title)
	if title == "" || title == "Router "+r.Prefix {
		return "Router " + r.Prefix
	}
	return r.Prefix + " " + title
}

// DisplayName is the router in log lines and error texts. With two or more routers (many) it is Display. With one, the
// prefix is hidden: the Router title, or "Router A" when there is no title.
func (r Router) DisplayName(many bool) string {
	if many {
		return r.Display()
	}
	if title := strings.TrimSpace(r.Title); title != "" {
		return title
	}
	return "Router " + r.Prefix
}

// RouterName is DisplayName for a router of c.
func (c Config) RouterName(r Router) string { return r.DisplayName(len(c.Routers) > 1) }

// validPrefix: a router prefix is one or two letters A to Z (A, B, BA).
func validPrefix(p string) bool {
	if len(p) < 1 || len(p) > 2 {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] < 'A' || p[i] > 'Z' {
			return false
		}
	}
	return true
}
