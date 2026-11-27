// Each layout draws its own copy ([data-broken]).
import { el, emit, icon, off, on } from './core.js';
import { btn } from './formbits.js';
import { profileName } from './profile.js';
import { layout } from './layout.js';
import { newBannerState, useItem, dismissItem, bannerItems, leftOutLines } from './brokenrules.js';
import { pageItems, presetKey, setKey } from './brokenview.js';

const state = newBannerState();
let role = '';

on('access', (a) => {
  role = a.role || '';
  render();
});

const KIND = { preset: 'Preset', filterSet: 'Filter set' };

const canEdit = (it) => (it.scope === 'global' ? role === 'administrator' || role === 'trusted' : it.scope === 'user' ? !!profileName() : true);

const swapOf = (it) => {
  const m = it.broken.find((x) => (x.kind === 'source' || x.kind === 'destination') && x.ref.n >= 0);
  return m ? { side: m.kind, from: m.ref.n, router: m.ref.router || '' } : null;
};

function itemNode(it) {
  const acts = [];
  // Routers, where a Swap is saved, is not on the phone.
  const sw = role === 'administrator' && layout() !== 'phone' && swapOf(it);
  if (sw) acts.push(btn('pb-btn--sm pb-btn--outline', 'Swap…', () => emit('swapAt', sw)));
  if (canEdit(it)) acts.push(btn('pb-btn--sm pb-btn--outline', 'Edit', () => emit(it.kind === 'preset' ? 'editPresetAt' : 'editFilterSetAt', it)));
  const x = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon('x'));
  x.type = 'button';
  x.setAttribute('aria-label', `Dismiss ${it.name}`);
  x.dataset.pbTip = 'Dismiss';
  x.addEventListener('click', () => {
    dismissItem(state, it.key);
    render();
  });
  const what = it.kind === 'preset' ? 'Left out of the take' : 'Matches nothing';
  return el(
    'div',
    'sode__broken-item',
    el(
      'div',
      'sode__banner-body',
      el('div', 'pb-alert__title', icon('unlink'), ` Broken: ${KIND[it.kind]} ${it.name}`),
      el('div', null, `${what}:`),
      el('ul', 'sode__broken-lines', ...leftOutLines(it.broken, off()).map((l) => el('li', null, l))),
      acts.length ? el('div', 'sode__broken-acts', ...acts) : null,
    ),
    x,
  );
}

function render() {
  const items = bannerItems(state, pageItems());
  for (const box of document.querySelectorAll('[data-broken]')) {
    box.hidden = !items.length;
    box.replaceChildren(...items.map(itemNode));
  }
}

on('itemUsed', (u) => {
  const key = u.kind === 'preset' ? presetKey(u) : setKey(u);
  const it = pageItems().get(key);
  if (it) useItem(state, it);
  render();
});
for (const ev of ['hello', 'presets', 'filters', 'names', 'profile']) on(ev, render);
