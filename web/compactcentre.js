// Compact view centres the patch row (the three groups, Source to Clear) on the viewport (docs/agents/web-layout.md).
import { centreNudge } from './compactrules.js';

const root = document.documentElement;

// The CSS centring rule is the one switch: while it makes .sode a grid, the nudge applies.
function update() {
  const sode = document.querySelector('.sode');
  const view = document.getElementById('patch-view');
  const row = document.querySelector('.sode__boxes');
  const src = document.getElementById('src');
  if (!sode || !view || !row || !src) return;
  if (getComputedStyle(sode).display !== 'grid') {
    for (const v of ['--sode-centre-x', '--sode-centre-y']) root.style.removeProperty(v);
    return;
  }
  const cs = getComputedStyle(view);
  const ox = parseFloat(cs.left) || 0;
  const oy = parseFloat(cs.top) || 0;
  const shift = (r) => ({ left: r.left - ox, right: r.right - ox, top: r.top - oy, bottom: r.bottom - oy });
  // The groups fill the row from edge to edge; the text boxes give the height to centre.
  const r = row.getBoundingClientRect();
  const a = src.getBoundingClientRect();
  const box = shift({ left: r.left, right: r.right, top: a.top, bottom: a.bottom });
  const n = centreNudge(box, shift(view.getBoundingClientRect()), innerWidth, innerHeight);
  root.style.setProperty('--sode-centre-x', `${n.x}px`);
  root.style.setProperty('--sode-centre-y', `${n.y}px`);
}

let queued = false;
const soon = () => {
  if (queued) return;
  queued = true;
  requestAnimationFrame(() => {
    queued = false;
    update();
  });
};

export function compactCentre() {
  addEventListener('resize', soon);
  new MutationObserver(soon).observe(root, { attributes: true, attributeFilter: ['data-compact', 'data-layout', 'data-sode-pick', 'data-view'] });
  // Anything that moves the boxes: each part of the page and of the block, and each box. A banner that shows moves the
  // centred block, and Take's label waits on the font.
  const ro = new ResizeObserver(soon);
  for (const n of document.querySelectorAll('.sode > *, #patch-view > *, .sode__boxes .sode__box')) ro.observe(n);
  document.fonts?.ready.then(soon);
  soon();
}
