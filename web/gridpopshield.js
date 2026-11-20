// The Grid popup's shield: Protect or Clear Protect on a destination name (GLOSSARY.md Protect). One button sits over
// the name under the pointer, or the cursor's name after a key. So the grid's own name nodes stay as they are. A
// protected name always shows its Protected badge (gridpanelnodes.js). protects.js decides what is offered and makes
// the plan; confirm(plan) opens the main page's Confirm.
import { el, icon, on, dstItem } from './core.js';
import { protectOfferOf, protectPlan } from './protects.js';

const NAME = '.sode-gridpanel__dst:not([hidden])';

export function gridShield(box, grid, confirm) {
  const b = el('button', { class: 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode-gridpop__shield', type: 'button', hidden: true });
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    const n = Number(b.dataset.dst);
    confirm(protectPlan(dstItem(n), b.dataset.protect === 'set'));
  });
  box.append(b);
  let at = null; // the name node the button is over

  function place(node) {
    at = node;
    const n = node ? Number(node.dataset.dst) : NaN;
    const offer = Number.isNaN(n) ? '' : protectOfferOf(n);
    if (!offer) {
      b.hidden = true;
      return;
    }
    const set = offer === 'set';
    const label = dstItem(n).label;
    const tip = set ? `Protect ${label} on every level` : `Clear the Protect on ${label}`;
    if (b.dataset.protect !== offer) b.replaceChildren(icon(set ? 'shield' : 'shield-off'));
    Object.assign(b.dataset, { dst: String(n), protect: offer, pbTip: tip });
    b.setAttribute('aria-label', tip);
    const r = node.getBoundingClientRect();
    const o = box.getBoundingClientRect();
    b.hidden = false;
    b.style.left = `${r.right - o.left - b.offsetWidth}px`;
    b.style.top = `${r.top - o.top + (r.height - b.offsetHeight) / 2}px`;
  }

  grid.addEventListener('pointermove', (e) => {
    if (e.pointerType === 'touch') return;
    const node = e.target.closest?.(NAME) || null;
    if (node !== at) place(node);
  });
  // A scroll moves name nodes to other items.
  grid.addEventListener('scroll', () => place(null), true);
  grid.addEventListener('pointerleave', (e) => {
    if (e.relatedTarget !== b) place(null);
  });
  const offTally = on('tally', () => at && place(at.isConnected ? at : null));
  return {
    // After a key: the name the cursor outlines (gridpanelcursor.js data-cue).
    cursor: () => place(grid.querySelector(`${NAME}[data-cue]`)),
    hide: () => place(null),
    stop: offTally,
  };
}
