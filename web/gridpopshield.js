// The Grid popup's shield: Protect or Free Protect on a destination name (GLOSSARY.md Protect). One small button sits
// at the outer edge of the name under the pointer, or of the cursor's name after a key. So it covers no crosspoint,
// and the grid's own name nodes stay as they are. On a picked name it acts on every picked destination with the same offer
// (shieldTargets). A protected name always shows its owner mark (gridpanelnodes.js). protects.js decides what is
// offered and makes the plan; confirm(plan) opens the main page's Confirm.
import { el, icon, on, dstItem } from './core.js';
import { protectOfferOf, protectManyPlan } from './protects.js';
import { shieldTargets } from './protectrules.js';

const NAME = '.sode-gridpanel__dst:not([hidden])';

// One name, or many picked names, in the button's tip and label.
function shieldTip(set, dsts) {
  if (dsts.length > 1)
    return set ? `Protect the ${dsts.length} picked destinations on every level` : `Free the Protect on the ${dsts.length} picked destinations`;
  const label = dstItem(dsts[0]).label;
  return set ? `Protect ${label} on every level` : `Free the Protect on ${label}`;
}

export function gridShield(box, grid, confirm) {
  const b = el('button', { class: 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode-gridpop__shield', type: 'button', hidden: true });
  b.addEventListener('mousedown', (e) => e.preventDefault());
  let targets = []; // the destination numbers the button acts on
  b.addEventListener('click', () =>
    confirm(
      protectManyPlan(
        targets.map((n) => dstItem(n)),
        b.dataset.protect === 'set',
      ),
    ),
  );
  box.append(b);
  let at = null; // the name node the button is on
  let picked = []; // the picked destination numbers (the body's picks.dsts)

  function place(node) {
    at = node;
    const n = node ? Number(node.dataset.dst) : NaN;
    const t = Number.isNaN(n) ? null : shieldTargets(n, picked, protectOfferOf);
    if (!t) {
      b.hidden = true;
      targets = [];
      return;
    }
    const set = t.offer === 'set';
    targets = t.dsts;
    const tip = shieldTip(set, targets);
    if (b.dataset.protect !== t.offer) b.replaceChildren(icon(set ? 'shield' : 'shield-off'));
    Object.assign(b.dataset, { dst: String(n), protect: t.offer, count: String(targets.length), pbTip: tip });
    b.setAttribute('aria-label', tip);
    const r = node.getBoundingClientRect();
    const o = box.getBoundingClientRect();
    b.hidden = false;
    // The name's outer edge: the side away from the crosspoints.
    b.style.left = `${r.left - o.left}px`;
    b.style.top = `${r.top - o.top + (r.height - b.offsetHeight) / 2}px`;
  }

  grid.addEventListener('pointermove', (e) => {
    if (e.pointerType === 'touch') return;
    const node = e.target.closest?.(NAME) || null;
    if (node !== at) place(node);
  });
  // A scroll moves name nodes to other items.
  grid.addEventListener('scroll', () => place(null), true);
  // The shield sits on the name, so the pointer moves onto it (or its icon) and back without a change.
  grid.addEventListener('pointerleave', (e) => {
    if (!b.contains(e.relatedTarget)) place(null);
  });
  b.addEventListener('pointerleave', (e) => {
    if (!grid.contains(e.relatedTarget)) place(null);
  });
  const offTally = on('tally', () => at && place(at.isConnected ? at : null));
  return {
    // The body's picks changed (gridpanelrun.js ui.onPicks): the shield on a picked name now covers them all.
    picks(p) {
      picked = p.dsts;
      if (at) place(at.isConnected ? at : null);
    },
    // After a key: the name the cursor outlines (gridpanelcursor.js data-cue).
    cursor: () => place(grid.querySelector(`${NAME}[data-cue]`)),
    hide: () => place(null),
    stop: offTally,
  };
}
