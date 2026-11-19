// Compact view centres the Source, Destination and Levels boxes on the viewport (docs/agents/web-layout.md).
import { centreNudge } from './compactrules.js';

const root = document.documentElement;

// The CSS centring rule is the one switch: while it makes .sode a grid, the nudge applies.
function update() {
  const sode = document.querySelector('.sode');
  const view = document.getElementById('patch-view');
  const src = document.getElementById('src');
  const lvl = document.getElementById('lvl');
  if (!sode || !view || !src || !lvl) return;
  if (getComputedStyle(sode).display !== 'grid') {
    for (const v of ['--sode-centre-x', '--sode-centre-y', '--sode-take-w']) root.style.removeProperty(v);
    return;
  }
  // Take's column, mirrored as padding on the left of the row (desktop.css), so the boxes are the row's middle.
  const take = document.getElementById('patch').closest('.sode__box');
  if (take) root.style.setProperty('--sode-take-w', `${take.getBoundingClientRect().width}px`);
  const cs = getComputedStyle(view);
  const ox = parseFloat(cs.left) || 0;
  const oy = parseFloat(cs.top) || 0;
  const shift = (r) => ({ left: r.left - ox, right: r.right - ox, top: r.top - oy, bottom: r.bottom - oy });
  const a = src.getBoundingClientRect();
  // A one-level router hides Levels (levelview.js): the span then ends at the last box before Take.
  const before = [...document.querySelectorAll('.sode__boxes > .sode__box')].filter((n) => n !== take && n.offsetParent);
  const b = lvl.offsetParent ? lvl.getBoundingClientRect() : { right: Math.max(...before.map((n) => n.getBoundingClientRect().right)) };
  const box = shift({ left: a.left, right: b.right, top: a.top, bottom: a.bottom });
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
  // Anything that moves the boxes inside the block: its size, the row's, Take's (its label waits on the font) and each box's.
  const ro = new ResizeObserver(soon);
  for (const n of document.querySelectorAll('#patch-view, .sode__boxes, .sode__boxes > .sode__box, .sode__mark')) ro.observe(n);
  document.fonts?.ready.then(soon);
  soon();
}
