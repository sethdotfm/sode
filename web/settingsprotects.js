// Settings > Routers > Protects (Administrator): every Protect and Router lock that each router reports. Each row
// shows the device that set it, and Sode's own show the Sode user. With two or more routers, each destination shows
// its router's prefix. Free is offered only on Sode's own. Free Sode's Protects frees every one with Sode's device
// number of its router, on each router that is up. It is Critical, so the server asks the PIN again.

import { $, el, S, on, dstItem, labelNode } from './core.js';
import { levelText } from './levelview.js';
import { holdLabel } from './protectrules.js';
import { protectPlan, sendProtect, clearSodeProtects, everyHold, holdBadge } from './protects.js';
import { confirmBox } from './confirmbox.js';
import { btn } from './formbits.js';

const list = $('protects-list');
const confirm = confirmBox($('protects-confirm'));
$('protects-confirm').append(confirm.node);
const status = el('p', 'sode__hint');
status.setAttribute('role', 'status');
$('protects-confirm').after(status);

// uid: the router (a routerData UID; blank is the first).
function row(n, h, uid) {
  const dst = dstItem(n, '', uid);
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
    el('span', 'sode__hint', `Levels ${levelText(h.levels, uid)}`),
    holdBadge(h),
    clear || null,
  );
  li.dataset.dst = String(n);
  if (uid) li.dataset.router = uid;
  return li;
}

function render() {
  if (!S.cfg) return;
  const rows = everyHold().map(({ n, hold, uid }) => row(n, hold, uid));
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
  const ok = await confirm.ask(
    S.routers.length > 1
      ? "Free every Protect with Sode's device number on each router that is connected? Destinations other users hold go free."
      : "Free every Protect with Sode's device number? Destinations other users hold go free.",
    "Free Sode's Protects",
  );
  if (!ok) return;
  status.textContent = 'Freeing…';
  clearSodeProtects((done, text) => {
    status.textContent = done ? `Sode's Protects freed: ${text}.` : `Not freed: ${text}.`;
  });
});
