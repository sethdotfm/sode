// What a panel's look settings and a cell's artwork mean. Pure. Stored names are in docs/persisted-names.md.
// Settings: titleBar (false hides the title bar), fontSize (percent), theme ('auto' follows the browser, 'light' or 'dark'), wake, fullscreen, landscape.
// Cell: art ('icon:<vendored icon name>' or an artwork id), artB (a glyph drawn beside it), fontSize (percent) and hideText.

const SCALE_MIN = 50;
const SCALE_MAX = 200;
export const THEMES = ['auto', 'light', 'dark'];
const ART_ID = /^a-[0-9a-f]{8}$/;
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

// '' when the cell has no Icon B, holds something that is not one of the choices, or has an uploaded image as Icon A
// (the image stands alone; artB stays stored, so a vendored icon brings it back).
export const artBOf = (cell) => (typeof cell?.artB === 'string' && ART_B_CHOICES.includes(cell.artB) && !artOf(cell)?.id ? cell.artB : '');

export const cellScale = (cell) => scaleOf(cell?.fontSize);

// null when the cell has no usable artwork. Anything but an icon name or an artwork id is dropped, so a stored URL never loads.
export function artOf(cell) {
  const art = cell?.art;
  if (typeof art !== 'string') return null;
  const icon = ICON.exec(art);
  if (icon) return { icon: icon[1] };
  return ART_ID.test(art) ? { id: art } : null;
}

export const artUrlPath = (panelId, artId) => `api/v1/panels/${panelId}/artwork/${artId}`;
