// What the panel editor's sidebar stores and says. Pure. Stored names are in docs/persisted-names.md.

export const FONT_MIN = 50;
export const FONT_MAX = 200;
export const ART_MAX_BYTES = 1 << 20; // keep in step with panelArtBytes in internal/app/panelstore.go
export const ART_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/svg+xml'];

// Each panel setting's default. A value equal to it is dropped from the stored settings, as the panel page
// reads a missing key as the default (panelmodel.js createState and panellook.js lookOf).
const DEFAULTS = {
  titleBar: true,
  fontSize: 100,
  theme: 'auto',
  wake: false,
  haptics: false,
  fullscreen: false,
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

export function fontPercent(text) {
  const t = String(text).trim();
  if (!/^\d+$/.test(t)) return { error: `Use a whole number from ${FONT_MIN} to ${FONT_MAX}.` };
  const value = Number(t);
  return value < FONT_MIN || value > FONT_MAX ? { error: `Use a whole number from ${FONT_MIN} to ${FONT_MAX}.` } : { value };
}

export function artProblem(file) {
  if (!ART_TYPES.includes(file.type)) return 'Artwork is a JPEG, PNG, GIF or SVG image.';
  if (file.size > ART_MAX_BYTES) return 'Artwork is at most 1 MB.';
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

export const ART_ICONS = [
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
  'bookmark',
  'flag',
  'bell',
  'zap',
  'power',
  'eye',
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

export const MODE_HINT = {
  take: 'A press stages the route. The Take cell sends it.',
  instant: 'A press sends the route at once, with no Take cell.',
};

const count = (list, one, many) => (!Array.isArray(list) ? `all ${many}` : list.length ? `${list.length} ${list.length === 1 ? one : many}` : `no ${many}`);

// Sources and Destinations start the sentence, so their first letter stays upper case.
export function rightsLine(rights) {
  const r = rights || {};
  const parts = [
    count(r.sources, 'Source', 'Sources'),
    count(r.dsts, 'Destination', 'Destinations'),
    count(r.levels, 'level', 'levels'),
    count(r.presets, 'Preset', 'Presets'),
  ];
  const head = parts.join(', ').replace(/^all /, 'All ').replace(/^no /, 'No ');
  const vis = r.visibility === 'hidden' ? 'hidden' : 'read-only';
  return rights && Object.keys(r).length ? `${head}, ${vis} visibility.` : 'All Sources, Destinations, levels and Presets, read-only visibility.';
}

// Watch in the editor: for a saved panel the user may edit, with the state the server last told us.
// `known` outlives a save, because the save's answer carries no `watching`.
export function watchView(p, known) {
  if (!p || !p.canEdit) return null;
  if ('watching' in p) known[p.id] = !!p.watching;
  return { id: p.id, on: !!known[p.id] };
}
