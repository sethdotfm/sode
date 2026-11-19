// Settings > Routers: the Routers list and each router block's sub-heading. One router today; the list and the block
// stay with one (no flat view), so a second router only adds rows and blocks.

import { $, S, el, emit, on } from './core.js';

let admin = false;
on('access', (a) => {
  admin = a.role === 'administrator';
  for (const id of ['router-1-swap', 'router-1-broken']) {
    const b = $(id);
    if (b) b.hidden = !admin;
  }
});

export const routerTitle = () => S.cfg?.routerName?.trim() || 'Router';

// onPick(id): a row's button goes to that router's block.
export function fillRouters(onPick) {
  if (!S.cfg) return;
  const title = routerTitle();
  $('router-1-title').textContent = title;
  const go = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', title);
  go.type = 'button';
  go.addEventListener('click', () => onPick('1'));
  // Swap stages into the one Save, so it is offered where the Routers section is editable (Administrator).
  const swap = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Swap…');
  swap.type = 'button';
  swap.id = 'router-1-swap';
  swap.setAttribute('aria-haspopup', 'dialog');
  swap.hidden = !admin;
  swap.addEventListener('click', () => emit('openSwap', { mine: false }));
  // The Broken and Name moved count (brokencount.js), Administrator only since it covers everyone's items.
  const broken = el('div', 'sode__broken-count');
  broken.id = 'router-1-broken';
  broken.hidden = !admin;
  $('routers-list').replaceChildren(el('li', 'sode__routers-row', go, el('span', 'sode__hint sode__code', S.cfg.router || 'No address'), swap, broken));
}
