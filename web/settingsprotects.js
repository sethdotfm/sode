// Settings > Routers > Protects (Administrator): every Protect and Router lock the router reports, with the device
// that set it, and Sode's own with the Sode user. Free is offered only on Sode's own. Free Sode's Protects frees
// every one with Sode's device number; it is Critical, so the server asks the PIN again.

import { $, el, S, on, dstItem, labelNode } from './core.js';
import { levelText } from './levelview.js';
import { holdLabel } from './protectrules.js';
import { protectPlan, sendProtect, clearSodeProtects, holdsAt, holdBadge } from './protects.js';
import { confirmBox } from './confirmbox.js';
import { btn } from './formbits.js';

const list = $('protects-list');
const confirm = confirmBox($('protects-confirm'));
$('protects-confirm').append(confirm.node);
const status = el('p', 'sode__hint');
status.setAttribute('role', 'status');
$('protects-confirm').after(status);

function row(n, h) {
  const dst = dstItem(n);
  const clear =
    h.sode &&
    btn('pb-btn--outline pb-btn--sm', 'Free', async () => {
      if (await confirm.ask(`Free the Protect on ${dst.label}, every level? ${holdLabel(h)}.`, 'Free')) sendProtect(protectPlan(dst, false));
    });
  if (clear) clear.setAttribute('aria-label', `Free the Protect on ${dst.label}`);
  const li = el(
    'li',
    'sode__protects-row',
    el('strong', null, labelNode(dst)),
    el('span', 'sode__hint', `Levels ${levelText(h.levels)}`),
    holdBadge(h),
    clear || null,
  );
  li.dataset.dst = String(n);
  return li;
}

function render() {
  if (!S.cfg) return;
  const dsts = [...new Set([...S.prot.keys(), ...S.protBy.keys()])].sort((a, b) => a - b);
  const rows = [];
  for (const n of dsts) {
    const p = holdsAt(n);
    for (const h of p.holds) rows.push(row(n, h));
    // Sode's record names levels the router does not report: Free Sode's Protects sends the Free Protect for them too.
    const rec = S.protBy.get(n);
    const lost = rec ? rec.levels.filter((u) => !p.levels.includes(u)) : [];
    if (lost.length)
      rows.push(row(n, { ...(p.sode || { sode: true, owner: 'sode', by: rec.by, client: rec.kind === 'client' }), device: rec.device, kind: 1, levels: lost }));
  }
  list.replaceChildren(...(rows.length ? rows : [el('li', 'sode__hint', 'No Protects or Router locks.')]));
}

let frame = 0;
const later = () => {
  frame ||= requestAnimationFrame(() => {
    frame = 0;
    render();
  });
};
for (const ev of ['hello', 'tally', 'names']) on(ev, later);

$('protects-clear-all').addEventListener('click', async () => {
  const ok = await confirm.ask("Free every Protect with Sode's device number? Destinations other users hold go free.", "Free Sode's Protects");
  if (!ok) return;
  status.textContent = 'Freeing…';
  clearSodeProtects((done, text) => {
    status.textContent = done ? `Sode's Protects freed: ${text}.` : `Not freed: ${text}.`;
  });
});
