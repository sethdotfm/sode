// Resolves a pref: this browser's value (localStorage), else the global default (deviceDefaults), else built-in; missing or unknown means follow global.
// Pure; device.js does storage. prepaint.js repeats the lookup inline: keep in step.

// Stored as strings; the on/off ones are booleans in the config.
const browserClock = () => (/h1[12]/.test(new Intl.DateTimeFormat([], { hour: 'numeric' }).resolvedOptions().hourCycle || '') ? '12-hour' : '24-hour');

export const PREFS = {
  theme: { key: 'sode-theme', def: 'system', values: { system: 'System', light: 'Light', dark: 'Dark' } },
  layout: { key: 'sode-layout', def: 'auto', values: { auto: 'Automatic', phone: 'Phone', desktop: 'Desktop' } },
  clock: { key: 'sode-clock', def: '24', values: { 24: '24-hour', 12: '12-hour', locale: 'Browser language' } },
  flip: { key: 'sode-flip', def: 'off', bool: true, values: { on: 'Yes', off: 'No' } },
  scrollTop: { key: 'sode-scroll-top', def: 'on', bool: true, values: { on: 'Yes', off: 'No' } },
  setStack: { key: 'sode-set-stack', def: 'and', values: { and: 'All must match', or: 'Any may match' } },
  nextMode: { key: 'sode-next-mode', def: 'number', values: { number: 'Next number', fuzzy: 'Next match of the name' } },
  searchFilters: { key: 'sode-search-filters', def: 'off', bool: true, values: { on: 'Yes', off: 'No' } },
};

// The Browser language option says what it gives (still one table entry, as the Go test reads it).
Object.defineProperty(PREFS.clock.values, 'locale', { get: () => `Browser language (${browserClock()})`, enumerable: true });

export const CACHE_KEY = 'sode-device-defaults';

const valid = (name, v) => typeof v === 'string' && Object.hasOwn(PREFS[name].values, v);

export function fromGlobal(name, g) {
  if (PREFS[name].bool && typeof g === 'boolean') return g ? 'on' : 'off';
  return valid(name, g) ? g : null;
}

export const PROFILE_CACHE_KEY = 'sode-profile-prefs';

// Cached profile prefs count only while a session token is kept (sode-session, sode-tab-session); else the next load is a guest.
// prepaint.js repeats this inline: keep in step.
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
