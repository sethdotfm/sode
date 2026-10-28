// This device: the per-browser settings (prefs.js). Each follows the
// global default (cfg.deviceDefaults, cached here for the next load) until
// this browser chooses its own. applyPrefs puts the resolved values to work
// and tells the views ('prefs', 'flip'); the layout follows in layout.js.
// Also the clock's time formats and the theme key.

import { $, el, on, emit } from './core.js';
import { PREFS, CACHE_KEY, resolve, own } from './prefs.js';
import { readJSON, writeJSON, readStr, writeStr } from './store.js';

let globals = readJSON(CACHE_KEY);
const stored = (name) => readStr(PREFS[name].key);
const resolveAll = () => Object.fromEntries(Object.keys(PREFS).map((n) => [n, resolve(n, stored(n), globals)]));
// What is in force now. index.html applied the same before first paint.
let applied = resolveAll();

export const pref = (name) => applied[name];
// This browser's own choice, or '' when it follows the global default.
export const ownPref = (name) => own(name, stored(name));
export const globalPref = (name) => resolve(name, null, globals);

// v '' (or anything unknown) goes back to the global default.
export function setPref(name, v) {
  writeStr(PREFS[name].key, own(name, v) ? v : null);
  applyPrefs();
}

// The server's deviceDefaults, on every hello (so also after a save).
function setGlobals(d) {
  globals = d || null;
  writeJSON(CACHE_KEY, globals);
  applyPrefs();
}
on('helloCfg', (cfg) => setGlobals(cfg.deviceDefaults));

function applyPrefs() {
  const was = applied;
  applied = resolveAll();
  const changed = (n) => was[n] !== applied[n];
  const root = document.documentElement;
  if (changed('theme')) {
    const ms = parseFloat(getComputedStyle(root).getPropertyValue('--pb-motion-theme')) || 0;
    root.classList.add('pb-theme-transition');
    if (applied.theme === 'system') delete root.dataset.theme;
    else root.dataset.theme = applied.theme;
    setTimeout(() => root.classList.remove('pb-theme-transition'), ms);
  }
  if (changed('flip')) {
    root.toggleAttribute('data-flip', applied.flip === 'on');
    emit('flip');
  }
  if (changed('clock')) {
    timeFmt = null;
    emit('namesChecked');
  }
  if (changed('clock') || changed('searchFilters')) emit('history');
  emit('prefs');
}

// Clock: '24' (default), '12', or 'locale' to follow the browser's
// language. Pages can't read the OS 12/24-hour switch.
export const clockPref = () => pref('clock');

// Destination before Source: the desktop arrow, Settings, and on a phone the
// order of the steps.
export const flipped = () => pref('flip') === 'on';
export const setFlip = (on) => setPref('flip', on ? 'on' : 'off');

// Scroll to the top when a box is focused (desktop).
export const scrollTopPref = () => pref('scrollTop') === 'on';

// The time formatter, built once and again only when the Clock changes.
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

// The time, after an ISO 8601 date (2026-10-22) unless it's today.
const isoDate = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const dateOf = (d) => (d.toDateString() === new Date().toDateString() ? '' : isoDate(d));
export const fmtTime = (iso) => {
  const d = new Date(iso);
  return [dateOf(d), timeFormat().format(d)].filter(Boolean).join(' ');
};
// The time alone, for detail under a row that shows the date.
export const fmtClock = (iso) => timeFormat().format(new Date(iso));
// Always dated, as nodes: the date in its own span so a narrow layout can
// drop it.
export const timeNodes = (iso) => {
  const d = new Date(iso);
  return [el('span', 'sode__date', `${isoDate(d)} `), timeFormat().format(d)];
};

// ---- theme

// Follows the global default until this browser picks one in Settings (or
// with Alt/Option+Shift+T); applyPrefs sets it, the select is settings.js's.
export function initTheme() {
  const root = document.documentElement;
  const osDark = matchMedia('(prefers-color-scheme: dark)');
  const isDark = () => (root.dataset.theme ? root.dataset.theme === 'dark' : osDark.matches);

  // Alt/Option+Shift+T: browsers keep Mod+Shift+T for reopening tabs.
  // Matched by e.code because Option changes e.key on macOS.
  if (/Mac|iPhone|iPad/.test(navigator.platform)) {
    $('cfg-theme-hint').replaceChildren(el('code', null, '⌥⇧t'), ' toggles');
  }
  document.addEventListener('keydown', (e) => {
    if (e.code !== 'KeyT' || !e.shiftKey || !e.altKey || e.ctrlKey || e.metaKey || e.repeat) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    setPref('theme', isDark() ? 'light' : 'dark');
  }, true);
}
