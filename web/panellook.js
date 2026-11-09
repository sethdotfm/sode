// What a panel's look settings and a cell's artwork mean. Pure. Stored names are in docs/persisted-names.md.
// Settings: titleBar (false hides the title bar), fontSize (percent), backdrop (an artwork id), wake, haptics.
// Cell: art ('icon:<vendored icon name>' or an artwork id) and fontSize (percent).

const SCALE_MIN = 50;
const SCALE_MAX = 200;
const ART_ID = /^a-[0-9a-f]{8}$/;
const ICON = /^icon:([a-z0-9-]+)$/;

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
    backdrop: ART_ID.test(s.backdrop) ? s.backdrop : '',
    wake: s.wake === true,
    haptics: s.haptics === true,
  };
}

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
