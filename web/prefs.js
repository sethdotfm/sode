// The per-browser ("This device") settings and how one is resolved: this
// browser's own choice (a localStorage value), else the global default from
// Settings (config deviceDefaults), else the built-in one. A missing or
// unknown stored value means "follow the global default". Pure, so Node
// tests it; core.js does the storage. index.html repeats the lookup for
// theme, layout and flip before first paint (keep them in step).

// Values are strings in localStorage; the on/off ones are booleans in the
// config. `values` maps each to how it reads in Settings.
export const PREFS = {
  theme: { key: 'sode-theme', def: 'system', values: { system: 'System', light: 'Light', dark: 'Dark' } },
  layout: { key: 'sode-layout', def: 'auto', values: { auto: 'Automatic', phone: 'Phone', desktop: 'Desktop' } },
  clock: { key: 'sode-clock', def: '24', values: { 24: '24-hour', 12: '12-hour', locale: 'Browser language' } },
  flip: { key: 'sode-flip', def: 'off', bool: true, values: { on: 'On', off: 'Off' } },
  scrollTop: { key: 'sode-scroll-top', def: 'on', bool: true, values: { on: 'On', off: 'Off' } },
  searchFilters: { key: 'sode-search-filters', def: 'off', bool: true, values: { on: 'On', off: 'Off' } },
};

// Where the last global defaults are kept, so the next page load follows
// them before the server's hello arrives.
export const CACHE_KEY = 'sode-device-defaults';

const valid = (name, v) => typeof v === 'string' && Object.hasOwn(PREFS[name].values, v);

// A config value (a string, or a boolean for the on/off ones) as a pref
// value, or null if it isn't one.
export function fromGlobal(name, g) {
  if (PREFS[name].bool && typeof g === 'boolean') return g ? 'on' : 'off';
  return valid(name, g) ? g : null;
}

// This browser's stored value if it is one, else ''.
export const own = (name, stored) => (valid(name, stored) ? stored : '');

// The value in force: own, else the global default, else the built-in.
export const resolve = (name, stored, globals) =>
  own(name, stored) || fromGlobal(name, globals?.[name]) || PREFS[name].def;

// The config value for a pref value, the reverse of fromGlobal.
export const toGlobal = (name, v) => (PREFS[name].bool ? v === 'on' : v);
