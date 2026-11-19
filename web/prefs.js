// Resolves a pref: this browser's value (localStorage), else the global default (browserDefaults), else built-in; missing or unknown means follow global.
// Pure; device.js does storage. prepaint.js repeats the lookup inline; prepaint.test.mjs checks they agree.

// Stored as strings; the on/off ones are booleans in the config.
const browserClock = () => (/h1[12]/.test(new Intl.DateTimeFormat([], { hour: 'numeric' }).resolvedOptions().hourCycle || '') ? '12-hour' : '24-hour');

export const PREFS = {
  theme: { label: 'Theme', key: 'sode-theme', def: 'system', values: { system: 'System', light: 'Light', dark: 'Dark' } },
  layout: { label: 'Layout', key: 'sode-layout', def: 'auto', values: { auto: 'Automatic', phone: 'Phone', desktop: 'Desktop' } },
  clock: { label: 'Clock', key: 'sode-clock', def: '24', values: { 24: '24-hour', 12: '12-hour', locale: 'Browser language' } },
  flip: { label: 'Destination before Source', key: 'sode-flip', def: 'off', bool: true, values: { on: 'Yes', off: 'No' } },
  scrollTop: { label: 'Scroll to top on focus', key: 'sode-scroll-top', def: 'on', bool: true, values: { on: 'Yes', off: 'No' } },
  setStack: { label: 'Stacked filters behavior', key: 'sode-set-stack', def: 'and', values: { and: 'All must match', or: 'At least one must match' } },
  nextMode: { label: 'Next match behavior', key: 'sode-next-mode', def: 'number', values: { number: 'Next number', fuzzy: 'Next match of the name' } },
  stackGap: {
    label: 'Stack quick changes to one destination',
    key: 'sode-stack-gap',
    def: '30',
    values: { 0: 'Off', 10: '10 seconds', 30: '30 seconds', 60: '1 minute', 120: '2 minutes', 300: '5 minutes' },
  },
  searchFilters: { label: 'Searching history respects filters', key: 'sode-search-filters', def: 'off', bool: true, values: { on: 'Yes', off: 'No' } },
  compact: { label: 'Compact view', key: 'sode-compact', def: 'off', bool: true, values: { on: 'Yes', off: 'No' } },
};

// The Browser language option says what it gives (still one table entry, as the Go test reads it).
Object.defineProperty(PREFS.clock.values, 'locale', { get: () => `Browser language (${browserClock()})`, enumerable: true });

export const CACHE_KEY = 'sode-browser-defaults';

const valid = (name, v) => typeof v === 'string' && Object.hasOwn(PREFS[name].values, v);

export function fromGlobal(name, g) {
  if (PREFS[name].bool && typeof g === 'boolean') return g ? 'on' : 'off';
  return valid(name, g) ? g : null;
}

export const PROFILE_CACHE_KEY = 'sode-profile-prefs';

// Cached profile prefs count only while a session token is kept (sode-session, sode-tab-session); else the next load is a guest.
// prepaint.js repeats this inline (prepaint.test.mjs).
export function cachedProfile(raw, hasSession) {
  if (!hasSession || !raw || typeof raw.user !== 'string' || !raw.user || !raw.prefs || typeof raw.prefs !== 'object') {
    return { user: '', prefs: {} };
  }
  return { user: raw.user, prefs: raw.prefs };
}

export const own = (name, stored) => (valid(name, stored) ? stored : '');

export const mine = (name, profilePrefs) => fromGlobal(name, profilePrefs?.[name]) || '';

export const resolve = (name, stored, profilePrefs, globals) =>
  own(name, stored) || mine(name, profilePrefs) || fromGlobal(name, globals?.[name]) || PREFS[name].def;

export const toGlobal = (name, v) => (PREFS[name].bool ? v === 'on' : v);
