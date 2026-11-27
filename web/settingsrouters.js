// Settings > Routers: the first router's row of the Routers list. settingsrouterlist.js adds the rows of the other
// routers, Add router and Remove router.

import { $, S, el, emit, on } from './core.js';
import { showRouterList } from './settingsrouterlist.js';
import { routerDisplay } from './routerlistrules.js';

let admin = false;
on('access', (a) => {
  admin = a.role === 'administrator';
  for (const id of ['router-1-swap', 'router-1-broken']) {
    const b = $(id);
    if (b) b.hidden = !admin;
  }
});

// The first router's heading: its Router title, or "Router" with none. With two or more routers its prefix comes first,
// as every other router's does ("A Studio A", "Router A").
export const routerTitle = () => (S.routers.length > 1 ? routerDisplay(S.routers[0]) : S.routers[0]?.title?.trim() || 'Router');

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
  swap.addEventListener('click', () => emit('openSwap', { mine: false, router: S.routers[0]?.uid || '' }));
  // The Broken and Name moved count (brokencount.js), Administrator only since it covers everyone's items.
  const broken = el('div', 'sode__broken-count');
  broken.id = 'router-1-broken';
  broken.hidden = !admin;
  // The link state, in words (settingsrouterlist.js paints it).
  const link = el('span', 'sode__routers-link');
  link.dataset.link = S.routers[0]?.uid || '';
  showRouterList(el('li', 'sode__routers-row', go, el('span', 'sode__hint sode__code', S.routers[0]?.address || 'No address'), link, swap, broken));
}
