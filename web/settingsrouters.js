// Settings > Routers: the Routers list and each router block's sub-heading. One router today; the list and the block
// stay with one (no flat view), so a second router only adds rows and blocks.

import { $, S, el, emit } from './core.js';

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
  swap.addEventListener('click', () => emit('openSwap', { mine: false }));
  $('routers-list').replaceChildren(el('li', 'sode__routers-row', go, el('span', 'sode__hint sode__code', S.cfg.router || 'No address'), swap));
}
