// Per-browser prefs (prefs.js): follow cfg.deviceDefaults (cached here for the next load) until this browser picks its own.
// applyPrefs tells the views ('prefs', 'flip'); layout.js follows.

import { el, on, emit } from './core.js';
import { PREFS, CACHE_KEY, PROFILE_CACHE_KEY, resolve, own, mine, cachedProfile } from './prefs.js';
import { readJSON, writeJSON, readStr, writeStr, readTab } from './store.js';

let globals = readJSON(CACHE_KEY);
// {user, prefs} of the logged-in user, cached until the profile arrives. Used only while a session token is kept
// (same keys as lock.js TOKEN_KEY / TAB_KEY).
const cached = cachedProfile(readJSON(PROFILE_CACHE_KEY), !!(readStr('sode-session') || readTab('sode-tab-session')));
let mineUser = cached.user;
let mineMap = cached.prefs;
const stored = (name) => readStr(PREFS[name].key);
const resolveAll = () => Object.fromEntries(Object.keys(PREFS).map((n) => [n, resolve(n, stored(n), mineMap, globals)]));
let applied = resolveAll();

export const pref = (name) => applied[name];
export const ownPref = (name) => own(name, stored(name));
export const globalPref = (name) => resolve(name, null, null, globals);
export const defaultPref = (name) => resolve(name, null, mineMap, globals);
export const myPref = (name) => mine(name, mineMap);

function setProfilePrefs(p) {
  const user = p?.name || '';
  mineUser = user;
  mineMap = user ? { ...p.prefs } : {};
  writeJSON(PROFILE_CACHE_KEY, user ? { user, prefs: mineMap } : null);
  applyPrefs();
}
on('profile', setProfilePrefs);
// Nobody or somebody else is in: drop the cached prefs; `profile` brings the right ones.
on('access', (a) => {
  if (mineUser && a.user !== mineUser) setProfilePrefs(null);
});

// '' or unknown goes back to the global default.
export function setPref(name, v) {
  writeStr(PREFS[name].key, own(name, v) ? v : null);
  applyPrefs();
}

function setGlobals(d) {
  globals = d || null;
  writeJSON(CACHE_KEY, globals);
  applyPrefs();
}
on('helloCfg', (cfg) => setGlobals(cfg.deviceDefaults));

// View Transition where available, else patchbay's fade class.
function setTheme(next) {
  const root = document.documentElement;
  const apply = () => {
    if (next === 'system') delete root.dataset.theme;
    else root.dataset.theme = next;
  };
  if (document.startViewTransition && !matchMedia('(prefers-reduced-motion: reduce)').matches) {
    document.startViewTransition(apply);
    return;
  }
  const ms = parseFloat(getComputedStyle(root).getPropertyValue('--pb-motion-theme')) || 0;
  root.classList.add('pb-theme-transition');
  apply();
  setTimeout(() => root.classList.remove('pb-theme-transition'), ms + 50);
}

function applyPrefs() {
  const was = applied;
  applied = resolveAll();
  const changed = (n) => was[n] !== applied[n];
  const root = document.documentElement;
  if (changed('theme')) setTheme(applied.theme);
  if (changed('flip')) {
    root.toggleAttribute('data-flip', applied.flip === 'on');
    emit('flip');
  }
  if (changed('compact')) root.toggleAttribute('data-compact', applied.compact === 'on');
  if (changed('clock')) {
    timeFmt = null;
    emit('namesChecked');
  }
  if (changed('clock') || changed('searchFilters') || changed('setStack') || changed('nextMode')) emit('history');
  emit('prefs');
}

// 'locale' follows the browser language; pages can't read the OS 12/24-hour switch.
export const clockPref = () => pref('clock');

export const flipped = () => pref('flip') === 'on';
export const setFlip = (on) => setPref('flip', on ? 'on' : 'off');

export const scrollTopPref = () => pref('scrollTop') === 'on';

// Desktop only: desktop.css hides History under html[data-layout="desktop"][data-compact].
export const compactOn = () => pref('compact') === 'on';
// Writes the opposite, never '': with Everyone on, '' would follow Everyone and change nothing.
export const toggleCompact = () => setPref('compact', compactOn() ? 'off' : 'on');

let timeFmt = null;
function timeFormat() {
  if (timeFmt) return timeFmt;
  const pref = clockPref();
  const opts = { minute: '2-digit', second: '2-digit' };
  // hourCycle, not hour12: Chrome shows 24:05 for hour12: false.
  if (pref !== 'locale') opts.hourCycle = pref === '12' ? 'h12' : 'h23';
  // 3:04:05 PM but 03:04:05 / 15:04:05.
  const h12 = /h1[12]/.test(new Intl.DateTimeFormat([], { hour: 'numeric', ...opts }).resolvedOptions().hourCycle);
  timeFmt = new Intl.DateTimeFormat([], { hour: h12 ? 'numeric' : '2-digit', ...opts });
  return timeFmt;
}

const isoDate = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const dateOf = (d) => (d.toDateString() === new Date().toDateString() ? '' : isoDate(d));
export const fmtTime = (iso) => {
  const d = new Date(iso);
  return [dateOf(d), timeFormat().format(d)].filter(Boolean).join(' ');
};
export const fmtClock = (iso) => timeFormat().format(new Date(iso));
export const timeNodes = (iso) => {
  const d = new Date(iso);
  return [el('span', 'sode__date', `${isoDate(d)} `), timeFormat().format(d)];
};

// ---- theme

export function initTheme() {
  const root = document.documentElement;
  const osDark = matchMedia('(prefers-color-scheme: dark)');
  const isDark = () => (root.dataset.theme ? root.dataset.theme === 'dark' : osDark.matches);

  // Alt/Option+Shift+T: browsers keep Mod+Shift+T for reopening tabs.
  // Matched by e.code because Option changes e.key on macOS.
  document.addEventListener(
    'keydown',
    (e) => {
      if (e.code !== 'KeyT' || !e.shiftKey || !e.altKey || e.ctrlKey || e.metaKey || e.repeat) return;
      e.preventDefault();
      e.stopImmediatePropagation();
      setPref('theme', isDark() ? 'light' : 'dark');
    },
    true,
  );
}
