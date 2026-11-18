// Which part of a Grid panel is drawn, one axis at a time (gridpanelview.js draws it). Pure, all in px.
// An axis is { n, pins, size, head, offset, view, over }. It has n items of size px after a name bar head px long.
// The first pins items are pinned at the edge. It scrolls by offset in a view px long. over is extra items each end.
// Items are display order indices (gridAxes), never router numbers.

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

// Items drawn past each end of the view, so a fast scroll shows fewer blank items.
export const OVER = 4;

// The scrolling items to draw as [lo, hi). Pinned items are drawn apart, so lo is never under pins.
export function axisSpan({ n, pins, size, head, offset, view, over = 0 }) {
  const lo = Math.floor(offset / size) + pins - over;
  const hi = Math.ceil((offset + view - head) / size) + over;
  return { lo: clamp(lo, pins, n), hi: clamp(hi, pins, n) };
}

// The most items axisSpan gives at any offset: those past the pinned band, a part item more, over each side.
export function spanMax({ n, pins, size, head, view, over = 0 }) {
  return clamp(Math.ceil((view - head) / size) - pins + 1 + 2 * over, 0, n - pins);
}

// The item under px, a point measured from the view's start; -1 on the name bar or past the last item.
export function indexAt(px, { n, pins, size, head, offset }) {
  if (px < head) return -1;
  const i = px - head < pins * size ? Math.floor((px - head) / size) : Math.floor((px - head + offset) / size);
  return i < n ? i : -1;
}

// The offset that shows item i whole, clear of the name bar and pinned band, moving as little as it can.
export function scrollFor(i, { pins, size, head, offset, view }) {
  if (i < pins) return offset;
  const start = head + i * size;
  if (start < offset + head + pins * size) return (i - pins) * size;
  if (start + size > offset + view) return start + size - view;
  return offset;
}

// The offset that puts item i in the middle of the part past the name bar and pinned band. The scroll range limits
// it. A pinned item always shows, so its offset stays.
export function centreFor(i, { n, pins, size, head, offset, view }) {
  if (i < pins) return offset;
  const mid = head + i * size + size / 2 - (head + pins * size + view) / 2;
  return clamp(Math.round(mid), 0, Math.max(0, head + n * size - view));
}

// gridAxes puts pinned items first, so they are the leading run of order that the edge pins.
export function pinsOf(edge, order) {
  let k = 0;
  while (k < order.length && edge?.items?.[order[k]]?.pinned === true) k++;
  return k;
}

// How many of an edge's pins draw as pinned: at most half the room past the name bar. Thus any number of pins leaves
// room to scroll. The rest scroll with the other items; they come first in order.
export const pinsShown = ({ pins, size, head, view }) => Math.min(pins, Math.max(0, Math.floor((view - head) / size / 2)));

// As indexAt, but null past the last item or before the view, so a caller can tell that from the name bar (-1).
export function slotAt(px, a) {
  if (px < 0) return null;
  if (px < a.head) return -1;
  const i = indexAt(px, a);
  return i < 0 ? null : i;
}

// The whole scrolling items one screen shows, at least one: a PageUp or PageDown moves this many.
export const pageOf = ({ pins, size, head, view }) => Math.max(1, Math.floor((view - head - pins * size) / size));

// The start of item i in content px; -1 is the name bar. The bar and pinned items are sticky, so they follow offset.
export function placeAt(i, { pins, size, head, offset }) {
  if (i < 0) return offset;
  return i < pins ? offset + head + i * size : head + i * size;
}

// The px across that a wheel scrolls when it turns over the horizontal scroll bar, else 0. A trackball or a mouse wheel
// gives deltaY, so over that bar it moves the grid across. bar is the bar in client px, with line and page as the px of
// one line and one page (deltaMode 1 and 2). A sideways wheel and a pinch (ctrlKey) keep the browser's own scroll.
export function wheelAcross(e, bar) {
  if (e.ctrlKey || Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return 0;
  if (e.clientX < bar.left || e.clientX >= bar.right || e.clientY < bar.top || e.clientY >= bar.bottom) return 0;
  return e.deltaY * (e.deltaMode === 1 ? bar.line : e.deltaMode === 2 ? bar.page : 1);
}

// A band is an edge's pinned band: { pins, slots, at }. It shows slots items, from pin at. An edge with more pins than
// slots scrolls its band on its own, and the rest of the grid scrolls as before. The window maths
// above works in slots: the band's slots, then the items past the pins. A band with pins === slots maps item to slot
// one to one. -1 (the name bar) is the same in both.
export const itemOf = (v, b) => (v < b.slots ? (v < 0 ? v : v + b.at) : v - b.slots + b.pins);

// The slot that shows item k, or null for a pin the band does not show now.
export function slotOf(k, b) {
  if (k < 0 || k >= b.pins) return k < 0 ? k : k - b.pins + b.slots;
  return k >= b.at && k < b.at + b.slots ? k - b.at : null;
}

// The band start that shows pin k, moving as little as it can. An item past the pins keeps the start.
export function bandFor(k, b) {
  if (k < 0 || k >= b.pins) return b.at;
  return clamp(k < b.at ? k : k >= b.at + b.slots ? k - b.slots + 1 : b.at, 0, b.pins - b.slots);
}

// The band start that puts pin k in the band's middle, as centreFor does for the rest.
export const bandCentre = (k, b) => (k < 0 || k >= b.pins ? b.at : clamp(k - Math.floor((b.slots - 1) / 2), 0, b.pins - b.slots));

// A wheel over a band that scrolls: { kind, px } for the band to move, else null and the grid scrolls. Over the
// Destination band (pinned rows) the vertical wheel moves it; over the Source band (pinned columns) the sideways one.
// box is the scroller's client box in client px ({ left, top, right, bottom }). It also has the name bar sizes w and h
// and the item size. line and page are the px of one line and one page (deltaMode 1 and 2).
export function wheelBand(e, box, bands) {
  if (e.ctrlKey) return null;
  const { clientX: x, clientY: y } = e;
  if (x < box.left || x >= box.right || y < box.top || y >= box.bottom) return null;
  const scale = (d) => d * (e.deltaMode === 1 ? box.line : e.deltaMode === 2 ? box.page : 1);
  const over = (b, at, start) => b.pins > b.slots && at >= start && at < start + b.slots * box.size;
  const down = Math.abs(e.deltaY) > Math.abs(e.deltaX);
  if (down && over(bands.dsts, y, box.top + box.h)) return { kind: 'dsts', px: scale(e.deltaY) };
  if (!down && e.deltaX && over(bands.srcs, x, box.left + box.w)) return { kind: 'srcs', px: scale(e.deltaX) };
  return null;
}
