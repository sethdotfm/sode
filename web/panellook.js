// artB is a glyph beside an icon, or a corner badge on an image. An unset hideText follows the item's Default artwork.

const SCALE_MIN = 50;
const SCALE_MAX = 200;
export const THEMES = ['auto', 'light', 'dark'];
// a- is the panel's own image, d- a Default artwork image (internal/app/artdefaults.go).
const ART_ID = /^[ad]-[0-9a-f]{8}$/;
const ICON = /^icon:([a-z0-9-]+)$/;
// An Icon library icon, as iconIDRe in internal/app/iconlib.go. Drawn as an image, never as a sprite icon.
const LIB = /^lib:([0-9a-f]{8})$/;
// Icon B: a number 0 to 16 or a letter A to Z. Text, not a sprite icon: Lucide has no digits.
export const ART_B_CHOICES = [...Array.from({ length: 17 }, (_, i) => String(i)), ...Array.from({ length: 26 }, (_, i) => String.fromCharCode(65 + i))];

// Percent to a multiplier; a non-number counts as 100.
function scaleOf(percent) {
  if (typeof percent !== 'number' || !Number.isFinite(percent)) return 1;
  return Math.min(SCALE_MAX, Math.max(SCALE_MIN, percent)) / 100;
}

export function lookOf(settings) {
  const s = settings || {};
  return {
    titleBar: s.titleBar !== false,
    scale: scaleOf(s.fontSize),
    theme: THEMES.includes(s.theme) ? s.theme : 'auto',
    wake: s.wake === true,
    fullscreen: s.fullscreen === true,
    landscape: s.landscape === true,
  };
}

// Only under artwork, so a cell never ends up with neither.
export const textHidden = (cell) => cell?.hideText === true && (!!artOf(cell) || !!artBOf(cell));

export const artBOf = (cell) => (typeof cell?.artB === 'string' && ART_B_CHOICES.includes(cell.artB) ? cell.artB : '');

// A corner badge over an image that fills the cell; beside a sprite icon Icon B keeps its 60% of the row. artNode (panelview.js) asks this.
export const artBBadge = (cell) => !!artBOf(cell) && !!artOf(cell) && !artOf(cell).icon;

export const cellScale = (cell) => scaleOf(cell?.fontSize);

// null when the cell has no usable artwork. Anything but an icon name or an artwork id is dropped, so a stored URL never loads.
export function artOf(cell) {
  const art = cell?.art;
  if (typeof art !== 'string') return null;
  const icon = ICON.exec(art);
  if (icon) return { icon: icon[1] };
  if (LIB.test(art)) return { id: art, lib: true };
  return ART_ID.test(art) ? { id: art } : null;
}

export function artUrlPath(panelId, artId) {
  const lib = LIB.exec(artId);
  if (lib) return `api/v1/icon-library/${lib[1]}`;
  return artId.startsWith('d-') ? `api/v1/default-artwork/images/${artId}` : `api/v1/panels/${panelId}/artwork/${artId}`;
}

// The item a cell shows, keyed as Default artwork is (cellItem and ArtDefault.key in artdefaults.go); '' for none.
// A Source or Destination keys by router and number. One with no router has the old key, kind/n, which is the first
// router's.
export function itemKey(cell) {
  if ((cell?.kind === 'source' || cell?.kind === 'destination') && Number.isInteger(cell.n)) {
    return cell.router ? `${cell.kind}/${cell.router}/${cell.n}` : `${cell.kind}/${cell.n}`;
  }
  if (cell?.kind === 'preset' && cell.id) return `preset/${cell.id}`;
  if (cell?.kind === 'preset' && cell.name) return `preset/${cell.scope}/${cell.name}`;
  return '';
}

// first is the first router's UID. Its defaults are also kept under the old key, for an item with no router. Examples
// are a Grid name on an edge with Follow the router on, and an old cell. With first not known yet, the first default of each number
// gets the old key. A preset's default is also kept under the preset's name, for a Preset cell that has no id yet.
export function defaultsBy(list, first = '') {
  const out = new Map();
  for (const d of Array.isArray(list) ? list : []) {
    out.set(itemKey(d), d);
    if (d.kind === 'preset' && d.id && d.name) out.set(itemKey({ ...d, id: '' }), d);
    if (d.kind === 'preset' || !Number.isInteger(d.n)) continue;
    const old = itemKey({ kind: d.kind, n: d.n });
    if (d.router && (d.router === first || (!first && !out.has(old)))) out.set(old, d);
    if (!d.router && first) out.set(itemKey({ ...d, router: first }), d);
  }
  return out;
}

export const defaultFor = (cell, defaults) => defaults?.get(itemKey(cell)) || null;

// A copy: the stored cell never holds a default, so a save, a Push or a template carries only what the cell sets.
export function withDefault(cell, defaults) {
  const d = defaultFor(cell, defaults);
  if (!d) return cell;
  const out = { ...cell };
  if (!artOf(cell) && !artBOf(cell)) {
    out.art = d.art;
    out.artB = d.artB;
  }
  if (typeof cell.hideText !== 'boolean') out.hideText = d.hideText === true;
  return out;
}

// A Source or Destination default was set while the number had another router name. Unknown names never warn.
export const nameMismatch = (d, routerName) => !!d && d.kind !== 'preset' && !!d.name && !!routerName && d.name !== routerName;
