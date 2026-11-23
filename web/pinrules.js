// Palette pin rules (GLOSSARY.md, Palette pin): pure, so Node tests load them. A pin is a Palette row key
// (palettecatalogue.js), never a web address. Pins show first, in their saved order, when the Palette opens with nothing
// typed. They are apart from Bookmarks. The server checks the same list (internal/config/palette.go
// PalettePinsProblem); web/test/pin_cases.json holds the cases that both read.

import { rowByKey } from './palettecatalogue.js';
import { rowKeyOK, moveBookmark } from './bookmarkrules.js';
import { keyLabel } from './keys.js';

// Keep in step with MaxPalettePins in internal/config/palette.go.
export const PINS_MAX = 24;

// Home is first in the empty Palette on every page but Home, so it is never a pin. Keep in step with palette.go.
export const HOME_KEY = 'page:home';

// Why Sode refuses this pin list, or ''. The same words as the server.
export function pinsProblem(list) {
  if (list.length > PINS_MAX) return `a list holds at most ${PINS_MAX} Palette pins`;
  const seen = new Set();
  for (const k of list) {
    if (k === HOME_KEY) return 'Home is always first in the Palette, so it is not pinned';
    if (!rowKeyOK(k)) return 'a Palette pin names a Palette row that Sode does not know';
    if (seen.has(k)) return 'a Palette row is pinned only one time';
    seen.add(k);
  }
  return '';
}

// A stored list as the page uses it: bad keys and repeats out, at most PINS_MAX.
export function cleanPins(v) {
  if (!Array.isArray(v)) return [];
  const out = [];
  for (const k of v) {
    const key = typeof k === 'string' ? k.trim() : '';
    if (rowKeyOK(key) && key !== HOME_KEY && !out.includes(key)) out.push(key);
  }
  return out.slice(0, PINS_MAX);
}

// The list with key at the end, or { err } when it is full.
export function addPin(list, key) {
  if (list.includes(key)) return { list };
  const next = [...list, key];
  const why = pinsProblem(next);
  return why ? { err: why } : { list: next };
}

export const removePin = (list, key) => list.filter((k) => k !== key);

// Alt+P and the row menu: one key pins and unpins. { list, pinned } or { err }.
export function togglePin(list, key) {
  if (list.includes(key)) return { list: removePin(list, key), pinned: false };
  const r = addPin(list, key);
  return r.err ? r : { list: r.list, pinned: true };
}

// A pin whose row is gone (a deleted preset, a panel that went): a grey row that says Missing, so it can be unpinned.
const NOUN = { action: 'action', preset: 'preset', page: 'page', panel: 'panel', filterSet: 'filter set', setting: 'setting' };
export function missingRow(key) {
  const noun = NOUN[key.split(':')[0]] || 'row';
  return {
    key,
    kind: 'missing',
    cat: '',
    label: `${noun[0].toUpperCase()}${noun.slice(1)}`,
    hint: '',
    icon: 'circle-off',
    words: [],
    home: false,
    missing: true,
    target: null,
    access: { ok: false, lock: false, why: `This ${noun} is not in Sode now. Press ${keyLabel('alt+p')} to unpin it.`, offer: [] },
  };
}

// The pins as rows, in their order. ready: the settings and panels are read. Until then a pin with no row is left
// out, not Missing.
export function pinRows(list, rows, { ready = true } = {}) {
  const out = [];
  for (const key of list) {
    const row = rowByKey(rows, key);
    if (row) out.push(row);
    else if (ready) out.push(missingRow(key));
  }
  return out;
}

// Alt+arrows ({ by }) or a drag ({ before }), as the Bookmarks editor moves its rows. null when nothing moves.
export const movePin = (list, i, where) => moveBookmark(list, i, where);
