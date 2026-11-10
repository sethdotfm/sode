// What the panel editor's sidebar stores and says. Pure. Stored names are in docs/persisted-names.md.
import { fixedDestsOf, FIXED_MAX } from './panelrules.js';

export { FIXED_MAX };

export const FONT_MIN = 50;
export const FONT_MAX = 200;
export const ART_MAX_BYTES = 2 << 20; // keep in step with panelArtBytes in internal/app/panelstore.go and docs/api.md
export const ART_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/svg+xml'];

// Each panel setting's default. A value equal to it is dropped from the stored settings, as the panel page
// reads a missing key as the default (panelmodel.js createState and panellook.js lookOf).
const DEFAULTS = {
  titleBar: true,
  fontSize: 100,
  theme: 'auto',
  wake: false,
  fullscreen: false,
  landscape: false,
  mode: 'take',
  multi: true,
  guard: 'none',
  resultText: false,
  kiosk: false,
};

export function setSetting(settings, key, value) {
  const out = { ...settings };
  // settings.levels is a UI level subset, where none chosen and every level both mean all.
  const dflt = key === 'levels' ? !value || !value.length : value === DEFAULTS[key];
  if (dflt) delete out[key];
  else out[key] = key === 'levels' ? [...value].sort((a, b) => a - b) : value;
  return out;
}

// Fixed destination (settings.fixedDests). The order is kept: the first one is what the Current Source cell reads.
const withFixed = (settings, list) => {
  const out = { ...settings };
  if (list.length) out.fixedDests = list;
  else delete out.fixedDests;
  return out;
};
export function addFixed(settings, n) {
  const list = fixedDestsOf({ settings });
  return list.includes(n) || list.length >= FIXED_MAX ? settings : withFixed(settings, [...list, n]);
}
export const dropFixed = (settings, n) =>
  withFixed(
    settings,
    fixedDestsOf({ settings }).filter((x) => x !== n),
  );

export function fontPercent(text) {
  const t = String(text).trim();
  if (!/^\d+$/.test(t)) return { error: `Use a whole number from ${FONT_MIN} to ${FONT_MAX}.` };
  const value = Number(t);
  return value < FONT_MIN || value > FONT_MAX ? { error: `Use a whole number from ${FONT_MIN} to ${FONT_MAX}.` } : { value };
}

export function artProblem(file) {
  if (!ART_TYPES.includes(file.type)) return 'Artwork is a JPEG, PNG, GIF or SVG image.';
  if (file.size > ART_MAX_BYTES) return 'Artwork is at most 2 MB.';
  return '';
}

// A cell's levels field is a subset of the panel's. Every level, or none chosen, removes the field.
export function levelsPatch(all, chosen) {
  const pick = all.filter((l) => chosen.includes(l));
  return !pick.length || pick.length === all.length ? undefined : pick;
}

// A level box shows what the panel or cell does: no stored subset means every level, so every box is ticked.
export function levelsShown(all, stored) {
  const pick = Array.isArray(stored) ? all.filter((l) => stored.includes(l)) : [];
  return pick.length ? pick : all;
}

// The patch after ticking or unticking one level. Unticking the last one is refused by the boxes (levelLocked), as
// nothing ticked would store the same as every level.
export function levelsToggle(all, stored, level, on) {
  const now = levelsShown(all, stored);
  return levelsPatch(all, on ? [...now, level] : now.filter((x) => x !== level));
}

export const levelLocked = (all, stored, level) => {
  const now = levelsShown(all, stored);
  return now.length === 1 && now[0] === level;
};

// The cell kind icons first (keep in step with CATEGORY_ICON in findrules.js), then Result and action icons, then pictures.
export const ART_ICONS = [
  'arrow-right-from-line',
  'arrow-right-to-line',
  'bookmark',
  'zap',
  'eraser',
  'eye',
  'panels-top-left',
  'tablet-smartphone',
  'check',
  'equal',
  'x',
  'search',
  'undo-2',
  'redo-2',
  'refresh-cw',
  'rotate-ccw',
  'arrow-left-right',
  'arrow-up-down',
  'arrow-up',
  'arrow-down',
  'arrow-left',
  'arrow-right',
  'plus',
  'minus',
  'combine',
  'split',
  'hand',
  'pointer',
  'video',
  'tv',
  'monitor',
  'camera',
  'clapperboard',
  'film',
  'projector',
  'cast',
  'mic',
  'headphones',
  'volume-2',
  'speaker',
  'radio',
  'music',
  'play',
  'pause',
  'square',
  'circle',
  'star',
  'flag',
  'bell',
  'power',
  'lock',
  'shield-check',
  'triangle-alert',
  'wifi',
  'cable',
  'globe',
  'layers-3',
  'layout-grid',
  'users',
  'user',
  'house',
  'sun',
  'moon',
  'clock',
  'image',
];


const amount = (list) => (!Array.isArray(list) ? 'All' : list.length ? String(list.length) : 'None');

// The Rights table of a client: one [name, value] row per list, then Visibility. No Rights means everything.
export function rightsRows(rights) {
  const r = rights || {};
  return [
    ['Sources', amount(r.sources)],
    ['Destinations', amount(r.dsts)],
    ['Levels', amount(r.levels)],
    ['Presets', amount(r.presets)],
    ['Visibility', r.visibility === 'hidden' ? 'Hidden' : 'Read-only'],
  ];
}

// The picker's icon search: names that contain every typed word, in the order given.
export const artFilter = (names, text) => {
  const words = String(text).toLowerCase().split(/[\s-]+/).filter(Boolean);
  return names.filter((n) => words.every((w) => n.includes(w)));
};
