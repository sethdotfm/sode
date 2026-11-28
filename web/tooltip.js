// Hover tooltips: every data-pb-tip, and the full text of ellipsis-cut text. core.js and /log load it for its side
// effects.

// Patchbay draws a data-pb-tip as a CSS ::after under its element. A box that clips (a modal, a scroller) cuts that
// tip off, and in a scroller it makes the box scroll. So base.css removes the ::after (content: none). This module shows
// the tip in a fixed element on document.body, where only the window can cut it. placeTip keeps it inside the window and off its control's field. The ::after rules still give the tip's look and place: tipLook reads them from the computed style of the
// ::after. Thus pb-tip--top, pb-tip--start and pb-tip--end keep working. So do a page's own rules (a wrapped tip, a
// tip that opens below) and rules that hide a tip (display: none).
const tip = document.createElement('div');
tip.className = 'pb-tip sode__tip';
tip.hidden = true;
tip.setAttribute('aria-hidden', 'true');
document.body.append(tip);
let tipFor = null,
  frame = 0,
  last = '';

// The tip's place in the window: under its element (over it when up), centred or lined up with one edge (align
// 'start' or 'end'). It goes on the first side with room that keeps it off avoid (see fieldBox). Else it goes on the
// first side with room. With room on neither, it goes to the side with more room. It always stays a gap inside the
// window. r is the element's box; w and h the tip's size.
export function placeTip({ r, w, h, vw, vh, align = 'center', up = false, gap = 4, avoid = null }) {
  const x = align === 'start' ? r.left : align === 'end' ? r.right - w : r.left + (r.width - w) / 2;
  const left = Math.max(gap, Math.min(x, vw - gap - w));
  const below = r.bottom + gap,
    above = r.top - gap - h;
  const sides = up ? [above, below] : [below, above];
  const fits = (top) => top >= gap && top + h <= vh - gap;
  const clear = (top) => !avoid || left + w <= avoid.left || left >= avoid.right || top + h <= avoid.top || top >= avoid.bottom;
  const top = sides.find((t) => fits(t) && clear(t)) ?? sides.find(fits) ?? (r.top > vh - r.bottom ? above : below);
  return { left, top: Math.max(gap, Math.min(top, vh - gap - h)) };
}

// The box that a tip must not cover: the parts of the element's field that do not hold the element. A field is a label
// and its box, a main page box under its head, or a wizard section under its heading. Null when there is no field.
const FIELD = '.sode__field, .sode__box, .sode-setup__section';
function fieldBox(t) {
  const f = t.closest(FIELD);
  if (!f) return null;
  const rs = [...f.children]
    .filter((c) => !c.contains(t))
    .map((c) => c.getBoundingClientRect())
    .filter((q) => q.width && q.height);
  if (!rs.length) return null;
  return {
    left: Math.min(...rs.map((q) => q.left)),
    top: Math.min(...rs.map((q) => q.top)),
    right: Math.max(...rs.map((q) => q.right)),
    bottom: Math.max(...rs.map((q) => q.bottom)),
  };
}

// What the ::after rules say for this tip: null when a rule hides it, else its side, edge and wrap.
function tipLook(t) {
  const cs = getComputedStyle(t, '::after');
  if (cs.display === 'none') return null;
  return {
    up: cs.bottom !== 'auto' && cs.top === 'auto',
    align: cs.right !== 'auto' ? 'end' : cs.left.endsWith('%') ? 'center' : 'start',
    whiteSpace: cs.whiteSpace,
    width: cs.width,
  };
}

const gapPx = () => parseFloat(getComputedStyle(document.documentElement).fontSize) / 4; // --pb-space-2

function paintTip(text) {
  const look = tipLook(tipFor);
  if (!look || !text.trim()) return void (tip.hidden = true);
  tip.textContent = text;
  tip.style.width = look.width === 'auto' ? '' : look.width;
  tip.style.whiteSpace = look.whiteSpace;
  tip.hidden = false;
  const gap = gapPx();
  // A one-line tip wider than its max-width (about the window, .pb-tip) wraps, so it stays whole.
  if (tip.scrollWidth > tip.clientWidth) tip.style.whiteSpace = 'normal';
  const { left, top } = placeTip({
    r: tipFor.getBoundingClientRect(),
    w: tip.offsetWidth,
    h: tip.offsetHeight,
    vw: innerWidth,
    vh: innerHeight,
    gap,
    avoid: fieldBox(tipFor),
    ...look,
  });
  tip.style.left = `${left}px`;
  tip.style.top = `${top}px`;
}

// Closes the tip now. A control calls it when it opens a menu over the place of its own tip (the Lock menu).
export function hideTip() {
  tip.hidden = true;
  tipFor = null;
  cancelAnimationFrame(frame);
}

// The CSS tip showed while its element was under the pointer or had keyboard focus, and moved and changed with it. So
// while the tip shows, each frame checks those, the text (a data-pb-tip that changes) and the element's place.
function follow() {
  if (!tipFor) return;
  if (!tipFor.isConnected || touchOnly.matches || !tipFor.matches(':hover, :focus-visible')) return hideTip();
  const r = tipFor.getBoundingClientRect();
  const now = `${tipFor.dataset.pbTip}|${r.left}|${r.top}|${r.width}|${r.height}|${innerWidth}|${innerHeight}`;
  if (now !== last) {
    last = now;
    paintTip(tipFor.dataset.pbTip ?? '');
  }
  frame = requestAnimationFrame(follow);
}

function showTip(t) {
  if (t === tipFor) return;
  cancelAnimationFrame(frame);
  tipFor = t;
  last = '';
  follow();
}

// Cut-off text shows in full on hover; so does data-hint where CSS sets --sode-hint: on. Elements with their own title
// keep it. A data-pb-tip that holds the whole text already says it, so that text does not show two times.
const cutTip = document.createElement('div');
cutTip.className = 'pb-tip';
cutTip.hidden = true;
cutTip.setAttribute('aria-hidden', 'true');
document.body.append(cutTip);
let cutFor = null;
const hideCutTip = () => {
  cutTip.hidden = true;
  cutFor = null;
};

function cutOff(n) {
  for (; n instanceof HTMLElement; n = n.parentElement) {
    const cs = getComputedStyle(n);
    if (n.dataset.hint && cs.getPropertyValue('--sode-hint').trim() === 'on') return { n, text: n.dataset.hint };
    if (cs.textOverflow !== 'ellipsis' || cs.overflowX === 'visible') continue;
    // A pixel's slack: Chrome rounds an input's scrollWidth up past text that fits.
    if (n.title || n.scrollWidth <= n.clientWidth + 1) return null;
    const text = (n instanceof HTMLInputElement ? n.value : n.textContent).replace(/\s+/g, ' ').trim();
    if (!text || n.closest('[data-pb-tip]')?.dataset.pbTip.includes(text)) return null;
    return { n, text };
  }
  return null;
}

// Under its text, or over it without room there. When the data-pb-tip tip shows on that side, it goes past that tip.
function placeCutTip(n) {
  const gap = 4,
    w = cutTip.offsetWidth,
    h = cutTip.offsetHeight,
    r = n.getBoundingClientRect(),
    o = tip.hidden ? null : tip.getBoundingClientRect();
  let below = r.bottom + gap,
    above = r.top - gap - h;
  if (o && o.top >= r.bottom - 1) below = o.bottom + gap;
  if (o && o.bottom <= r.top + 1) above = o.top - gap - h;
  cutTip.style.left = `${Math.max(gap, Math.min(r.left, innerWidth - w - gap))}px`;
  cutTip.style.top = `${Math.max(gap, below + h <= innerHeight ? below : above)}px`;
}

// The gates of patchbay's CSS tip stay. A touch screen gets no tip, since a tap leaves :hover on the button. A focus
// tip shows only for keyboard focus.
const touchOnly = globalThis.matchMedia?.('(hover: none)') ?? { matches: false }; // (no matchMedia in the Node tests)

document.addEventListener('mouseover', (e) => {
  if (touchOnly.matches) return;
  const t = e.target instanceof Element && e.target.closest('[data-pb-tip]');
  if (t) showTip(t);
  else if (tipFor && !tipFor.matches(':focus-visible')) hideTip();
  const c = cutOff(e.target);
  if (!c) return hideCutTip();
  if (c.n === cutFor) return;
  cutFor = c.n;
  cutTip.textContent = c.text;
  cutTip.hidden = false;
  placeCutTip(c.n);
});
document.addEventListener('mouseout', (e) => {
  if (!e.relatedTarget) hideCutTip();
});
document.addEventListener('focusin', (e) => {
  if (!touchOnly.matches && e.target instanceof Element && e.target.matches('[data-pb-tip]:focus-visible')) showTip(e.target);
});
document.addEventListener('focusout', hideCutTip);
for (const t of ['scroll', 'pointerdown', 'keydown']) document.addEventListener(t, hideCutTip, true);
