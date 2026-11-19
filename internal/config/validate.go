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

	if c.Address != "" {
		host, port, err := net.SplitHostPort(c.Address)
		if p, perr := strconv.Atoi(port); err != nil || host == "" || perr != nil || p < 1 || p > 65535 {
			bad("router must be host:port, for example 10.0.0.50:8910")
		}
	}
	if !slices.Contains(PanelActs, c.NewPanelActs) {
		bad("the new-panel Authentication mode is session, api, both or public")
	}
	switch c.NameRequests {
	case "std":
		// Standard messages carry the matrix in a 4-bit field.
		if c.Matrix < 0 || c.Matrix > 15 {
			bad("matrix must be 0-15 with standard name requests")
		}
	case "ext":
		if c.Matrix < 0 || c.Matrix > 255 {
			bad("matrix must be 0-255")
		}
	default:
		bad("name requests must be std or ext")
	}
	switch c.FireMode {
	case "sequential":
	case "native":
		// Group salvos carry the matrix in a 4-bit field.
		if c.Matrix < 0 || c.Matrix > 15 {
			bad("native salvos need matrix 0-15")
		}
	default:
		bad("fire mode must be sequential or native")
	}
	if c.SalvoGroup < 0 || c.SalvoGroup > 127 {
		bad("salvo group must be 0-127")
	}
	if c.Levels < 1 || c.Levels > 256 {
		bad("levels must be 1-256")
	} else if lo, hi := 1+c.LevelWireOffset, c.Levels+c.LevelWireOffset; lo < 0 || hi > 255 {
		bad("level wire offset puts levels at wire %d-%d; they must fall within 0-255", lo, hi)
	} else if _, err := ParseLevels(c.DefaultLevels, c.Levels); err != nil {
		bad("default levels: %v", err)
	}
	switch c.NameLength {
	case 4, 8, 12, 16:
	default:
		bad("name length must be 4, 8, 12 or 16")
	}
	if c.NumberOffset < 0 || c.NumberOffset > 1 {
		bad("number offset must be 0 or 1")
	}
	if c.ConfirmLockoutMs < 0 || c.ConfirmLockoutMs > 10000 {
		bad("confirm lockout must be 0-10000 ms")
	}
	if c.ResultDismissSeconds < 0 || c.ResultDismissSeconds > 3600 {
		bad("result dismiss must be 0 (never) or 1-3600 seconds")
	}
	if s := c.NameRefreshSeconds; s != 0 && (s < 10 || s > 86400) {
		bad("name refresh must be 0 (off) or 10-86400 seconds")
	}
	if s := c.TallyResyncSeconds; s != 0 && (s < 30 || s > 86400) {
		bad("tally resync must be 0 (off) or 30-86400 seconds")
	}
	switch c.LevelNumbering {
	case "0", "1", "va":
	default:
		bad("level numbering must be 0, 1 or va")
	}
	switch c.RouterShow {
	case "address", "name", "none":
	default:
		bad("router status must show address, name or none")
	}
	if len(c.Prefix) != 1 || c.Prefix[0] < 'A' || c.Prefix[0] > 'Z' {
		bad("router prefix must be one letter, A to Z")
	}
	if n := len([]rune(c.Title)); n > 32 {
		bad("router name must be at most 32 characters")
	}
	if n := len([]rune(c.BrandText)); n > 24 {
		bad("logo text must be at most 24 characters")
	}
	if _, ok := BrandColorByName(c.BrandBorder); c.BrandBorder != "" && !ok {
		bad("border colour must be one of %s", strings.Join(BrandNames(), ", "))
	}
	switch c.DeviceDefaults.Theme {
	case "system", "light", "dark":
	default:
		bad("default theme must be system, light or dark")
	}
	switch c.DeviceDefaults.Layout {
	case "auto", "phone", "desktop":
	default:
		bad("default layout must be auto, phone or desktop")
	}
	switch c.DeviceDefaults.Clock {
	case "24", "12", "locale":
	default:
		bad("default clock must be 24, 12 or locale")
	}
	switch c.DeviceDefaults.SetStack {
	case "and", "or":
	default:
		bad("default set stacking must be and or or")
	}
	switch c.DeviceDefaults.NextMode {
	case "number", "fuzzy":
	default:
		bad("default next must be number or fuzzy")
	}
	switch c.DeviceDefaults.StackGap {
	case "0", "10", "30", "60", "120", "300":
	default:
		bad("default stacked rows gap must be 0, 10, 30, 60, 120 or 300 seconds")
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
