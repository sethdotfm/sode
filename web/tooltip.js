// Hover tooltips: the floating tip for ellipsis-cut text, and the fit of patchbay's CSS tips. core.js and /log load it
// for its side effects.

// Cut-off text shows in full on hover; so does data-hint where CSS sets --sode-hint: on. Elements with their own title keep it.
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

// A CSS tip in a scroller (the .sode__sets rows, the Grid popup's staged list, the editor's Pages list) is clipped or
// makes it scroll. So the floating tip shows these tips, and CSS hides its own. So it does for a tip that fitTip finds
// has no room in the box that clips it (data-sode-tip-float, base.css).
const ROWS = ':is(.sode__sets, .sode__staged-list, .sode-pages__list) [data-pb-tip]';
const rowTip = (n) => n instanceof Element && n.closest(`${ROWS}, [data-pb-tip][data-sode-tip-float]`);

function cutOff(n) {
  const t = rowTip(n);
  if (t) return t.dataset.pbTip.trim() ? { n: t, text: t.dataset.pbTip } : null;
  for (; n instanceof HTMLElement; n = n.parentElement) {
    const cs = getComputedStyle(n);
    if (n.dataset.hint && cs.getPropertyValue('--sode-hint').trim() === 'on') return { n, text: n.dataset.hint };
    if (cs.textOverflow !== 'ellipsis' || cs.overflowX === 'visible') continue;
    // A pixel's slack: Chrome rounds an input's scrollWidth up past text that fits.
    if (n.title || n.scrollWidth <= n.clientWidth + 1) return null;
    const text = (n instanceof HTMLInputElement ? n.value : n.textContent).replace(/\s+/g, ' ').trim();
    return text ? { n, text } : null;
  }
  return null;
}

// The floating tip is fixed on the page, so only the window can cut it off.
function placeCutTip(n) {
  const gap = 4,
    w = cutTip.offsetWidth,
    h = cutTip.offsetHeight,
    r = n.getBoundingClientRect();
  cutTip.style.left = `${Math.max(gap, Math.min(r.left, innerWidth - w - gap))}px`;
  cutTip.style.top = `${Math.max(gap, r.bottom + gap + h <= innerHeight ? r.bottom + gap : r.top - gap - h)}px`;
}

// Patchbay's CSS tip sits under its element, centred (pb-tip--start and pb-tip--end line it up with an edge). Near the
// window's edge, or the edge of a box that clips (an open popup, a scroller), it would be cut off. So before it shows,
// this measures it in a copy and moves it. --sode-tip-x shifts it sideways (base.css). data-sode-tip-flip puts it on
// the other side when that side has room and its own has not. With room on neither side, the floating tip shows it.
const measure = document.createElement('div');
measure.className = 'pb-tip sode__tip-measure';
measure.setAttribute('aria-hidden', 'true');
document.body.append(measure);

function bounds(n) {
  let b = { left: 0, top: 0, right: innerWidth, bottom: innerHeight };
  for (let p = n.parentElement; p && p !== document.body; p = p.parentElement) {
    const cs = getComputedStyle(p);
    if (cs.overflowX === 'visible' && cs.overflowY === 'visible') continue;
    const r = p.getBoundingClientRect();
    b = { left: Math.max(b.left, r.left), top: Math.max(b.top, r.top), right: Math.min(b.right, r.right), bottom: Math.min(b.bottom, r.bottom) };
  }
  return b;
}

function fitTip(t) {
  if (!t.dataset.pbTip?.trim() || t.matches(ROWS)) return;
  measure.textContent = t.dataset.pbTip;
  const gap = 4,
    w = measure.offsetWidth,
    h = measure.offsetHeight,
    r = t.getBoundingClientRect(),
    b = bounds(t);
  const left = t.classList.contains('pb-tip--start') ? r.left : t.classList.contains('pb-tip--end') ? r.right - w : r.left + (r.width - w) / 2;
  const x = Math.max(b.left + gap - left, Math.min(0, b.right - gap - (left + w)));
  const up = t.classList.contains('pb-tip--top');
  const room = up ? r.top - b.top : b.bottom - r.bottom;
  const other = up ? b.bottom - r.bottom : r.top - b.top;
  const fits = (n) => n >= h + gap * 2;
  t.style.setProperty('--sode-tip-x', `${Math.round(x)}px`);
  t.toggleAttribute('data-sode-tip-flip', !fits(room) && fits(other));
  t.toggleAttribute('data-sode-tip-float', w + gap * 2 > b.right - b.left || (!fits(room) && !fits(other)));
}

const touchOnly = globalThis.matchMedia?.('(hover: none)') ?? { matches: false }; // (no matchMedia in the Node tests)

document.addEventListener('mouseover', (e) => {
  if (touchOnly.matches) return;
  const t = e.target instanceof Element && e.target.closest('[data-pb-tip]');
  if (t && !t.contains(e.relatedTarget)) fitTip(t);
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
  if (e.target instanceof Element && e.target.matches('[data-pb-tip]')) fitTip(e.target);
  const t = rowTip(e.target);
  if (touchOnly.matches || !t || !t.dataset.pbTip.trim() || !t.matches(':focus-visible')) return;
  cutFor = t;
  cutTip.textContent = t.dataset.pbTip;
  cutTip.hidden = false;
  placeCutTip(t);
});
document.addEventListener('focusout', hideCutTip);
for (const t of ['scroll', 'pointerdown', 'keydown']) document.addEventListener(t, hideCutTip, true);
