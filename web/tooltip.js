// Hover tooltip for ellipsis-cut text; core.js and /log load it for its side effects.

// Cut-off text shows in full on hover; so does data-hint where CSS sets --sode-hint: on. Elements with their own title keep it.
const cutTip = document.createElement('div');
cutTip.className = 'pb-tip';
cutTip.hidden = true;
cutTip.setAttribute('aria-hidden', 'true');
document.body.append(cutTip);
let cutFor = null;
const hideCutTip = () => { cutTip.hidden = true; cutFor = null; };

// Tips in a scrolling row (.sode__sets) would be clipped, so the floating tip shows them (the CSS one is hidden there).
const rowTip = (n) => n instanceof Element && n.closest('.sode__sets [data-pb-tip]');

function cutOff(n) {
  const t = rowTip(n);
  if (t) return { n: t, text: t.dataset.pbTip };
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

function placeCutTip(n) {
  const gap = 4, w = cutTip.offsetWidth, h = cutTip.offsetHeight, r = n.getBoundingClientRect();
  cutTip.style.left = `${Math.max(gap, Math.min(r.left, innerWidth - w - gap))}px`;
  cutTip.style.top = `${r.bottom + gap + h <= innerHeight ? r.bottom + gap : r.top - gap - h}px`;
}

const touchOnly = globalThis.matchMedia?.('(hover: none)') ?? { matches: false }; // (no matchMedia in the Node tests)

document.addEventListener('mouseover', (e) => {
  if (touchOnly.matches) return;
  const c = cutOff(e.target);
  if (!c) return hideCutTip();
  if (c.n === cutFor) return;
  cutFor = c.n;
  cutTip.textContent = c.text;
  cutTip.hidden = false;
  placeCutTip(c.n);
});
document.addEventListener('mouseout', (e) => { if (!e.relatedTarget) hideCutTip(); });
document.addEventListener('focusin', (e) => {
  const t = rowTip(e.target);
  if (touchOnly.matches || !t || !t.matches(':focus-visible')) return;
  cutFor = t;
  cutTip.textContent = t.dataset.pbTip;
  cutTip.hidden = false;
  placeCutTip(t);
});
document.addEventListener('focusout', hideCutTip);
for (const t of ['scroll', 'pointerdown', 'keydown']) document.addEventListener(t, hideCutTip, true);
