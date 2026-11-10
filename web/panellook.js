// What a panel's look settings and a cell's artwork mean. Pure. Stored names are in docs/persisted-names.md.
// Settings: titleBar (false hides the title bar), fontSize (percent), theme ('auto' follows the browser, 'light' or 'dark'), wake, fullscreen, landscape.
// Cell: art ('icon:<vendored icon name>' or an artwork id), artB (a glyph beside an icon, a corner badge on an image), fontSize (percent) and
// hideText (true or false; unset takes the item's Default artwork).

const SCALE_MIN = 50;
const SCALE_MAX = 200;
export const THEMES = ['auto', 'light', 'dark'];
// a- is the panel's own image, d- a Default artwork image (internal/app/artdefaults.go).
const ART_ID = /^[ad]-[0-9a-f]{8}$/;
const ICON = /^icon:([a-z0-9-]+)$/;
// Icon B: a digit 0 to 16 or a capital letter, so "[camera] 2" is two picks. Text, not a sprite icon: Lucide has no digits.
export const ART_B_CHOICES = [...Array.from({ length: 17 }, (_, i) => String(i)), ...Array.from({ length: 26 }, (_, i) => String.fromCharCode(65 + i))];

// Percent to a multiplier. Anything that is not a number counts as 100.
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

// Text is only hidden under artwork (Icon A or B), so a cell never ends up with neither.
export const textHidden = (cell) => cell?.hideText === true && (!!artOf(cell) || !!artBOf(cell));

// '' when the cell has no Icon B or holds something that is not one of the choices.
export const artBOf = (cell) => (typeof cell?.artB === 'string' && ART_B_CHOICES.includes(cell.artB) ? cell.artB : '');

// Icon B is a corner badge whenever Icon A draws as an image rather than a vendored sprite icon: the image fills the
// cell and the badge sits over it. Beside a sprite icon Icon B keeps its 60% of the row. artNode (panelview.js) asks this.
export const artBBadge = (cell) => !!artBOf(cell) && !!artOf(cell) && !artOf(cell).icon;

export const cellScale = (cell) => scaleOf(cell?.fontSize);

// null when the cell has no usable artwork. Anything but an icon name or an artwork id is dropped, so a stored URL never loads.
export function artOf(cell) {
  const art = cell?.art;
  if (typeof art !== 'string') return null;
  const icon = ICON.exec(art);
  if (icon) return { icon: icon[1] };
  return ART_ID.test(art) ? { id: art } : null;
}

export const artUrlPath = (panelId, artId) =>
  artId.startsWith('d-') ? `api/v1/default-artwork/images/${artId}` : `api/v1/panels/${panelId}/artwork/${artId}`;

// The item a cell shows, keyed as Default artwork is (cellItem and ArtDefault.key in artdefaults.go); '' for none.
export function itemKey(cell) {
  if ((cell?.kind === 'source' || cell?.kind === 'destination') && Number.isInteger(cell.n)) return `${cell.kind}/${cell.n}`;
  if (cell?.kind === 'preset' && cell.name) return `preset/${cell.scope}/${cell.name}`;
  return '';
}

// A list of defaults (GET /default-artwork, or a panel read's artDefaults) as a Map by item.
export const defaultsBy = (list) => new Map((Array.isArray(list) ? list : []).map((d) => [itemKey(d), d]));

export const defaultFor = (cell, defaults) => defaults?.get(itemKey(cell)) || null;

// The cell as drawn: its own artwork wins whole, else its item's default fills both icons. Hide text comes from the
// default unless the cell sets true or false itself. A copy: the stored cell never holds a default, so a save, a
// Push or a template carries only what the cell sets.
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
