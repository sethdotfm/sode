// The hover tooltip for text cut off with an ellipsis. Standalone: it only
// needs the DOM, so core.js (and the /log page) load it for its side effects.

// Text cut off with an ellipsis shows in full in a tooltip on hover, as
// soon as the pointer arrives; so does an element's data-hint wherever the
// CSS sets --sode-hint: on (its text is hidden there to save room).
// Cut-off elements with a title of their own keep it.
const cutTip = document.createElement('div');
cutTip.className = 'sode__cut-tip';
cutTip.hidden = true;
cutTip.setAttribute('aria-hidden', 'true'); // the full text is already in the DOM
document.body.append(cutTip);
let cutFor = null;
const hideCutTip = () => { cutTip.hidden = true; cutFor = null; };

function cutOff(n) {
  for (; n instanceof HTMLElement; n = n.parentElement) {
    const cs = getComputedStyle(n);
    if (n.dataset.hint && cs.getPropertyValue('--sode-hint').trim() === 'on') return { n, text: n.dataset.hint };
    if (cs.textOverflow !== 'ellipsis' || cs.overflowX === 'visible') continue;
    // A pixel's slack: Chrome rounds an input's scrollWidth up past text
    // that fits.
    if (n.title || n.scrollWidth <= n.clientWidth + 1) return null;
    const text = (n instanceof HTMLInputElement ? n.value : n.textContent).replace(/\s+/g, ' ').trim();
    return text ? { n, text } : null;
  }
  return null;
}

// Under the element (above it if there's no room), kept on screen. The tip
// sits in the zoomed page, so viewport positions are scaled back by the
// zoom it actually renders at.
function placeCutTip(n) {
  cutTip.style.left = cutTip.style.top = '0px';
  const o = cutTip.getBoundingClientRect(), r = n.getBoundingClientRect();
  const k = o.width / cutTip.offsetWidth || 1, gap = 4 * k;
  const x = Math.max(gap, Math.min(r.left, innerWidth - o.width - gap));
  const y = r.bottom + gap + o.height <= innerHeight ? r.bottom + gap : r.top - gap - o.height;
  cutTip.style.left = `${(x - o.left) / k}px`;
  cutTip.style.top = `${(y - o.top) / k}px`;
}

document.addEventListener('mouseover', (e) => {
  const c = cutOff(e.target);
  if (!c) return hideCutTip();
  if (c.n === cutFor) return;
  cutFor = c.n;
  cutTip.textContent = c.text;
  cutTip.hidden = false;
  placeCutTip(c.n);
});
document.addEventListener('mouseout', (e) => { if (!e.relatedTarget) hideCutTip(); });
for (const t of ['scroll', 'pointerdown', 'keydown']) document.addEventListener(t, hideCutTip, true);
