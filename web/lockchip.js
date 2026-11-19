// Change my PIN and Sign out live in Settings > My user (changepin.js); lock.js owns the state.

import { el, icon } from './core.js';
import { buildChangePin } from './changepin.js';

export function buildChip(api) {
  buildChangePin();
  const name = el('span', 'sode-lock__who-name');
  const lockBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm pb-tip--start', icon('lock-open'));
  lockBtn.type = 'button';
  lockBtn.dataset.pbTip = 'Lock (Ctrl+L)';
  lockBtn.setAttribute('aria-label', 'Lock');
  lockBtn.addEventListener('click', () => api.lock());
  // Status with words, cross-faded with the button (is-hand) so the icon does not snap against the lock screen's border.
  const ico = el('span', 'sode-lock__who-ico', icon('lock'));
  const slot = el('div', 'sode-lock__who-slot', lockBtn, ico);
  const root = el('div', 'sode-lock__who', el('div', 'sode-lock__who-bar', slot, name));
  root.hidden = true;
  // The bar floats and takes no room; the panel pages keep their header clear of its width (panels.css). The main
  // page has no such header, so it skips the observer. px, not rem: a measured box, and the observer redoes it when
  // the root font size changes the bar.
  if (document.querySelector('.sode-panels__head'))
    new ResizeObserver(() => {
      document.documentElement.style.setProperty('--sode-who-w', `${Math.ceil(root.getBoundingClientRect().width)}px`);
    }).observe(root);

  // pageLocked: the lock button does nothing, so it goes.
  function update(L, chip, pageLocked, who = L.user) {
    root.hidden = !chip;
    root.classList.toggle('is-hand', !!pageLocked);
    lockBtn.inert = !!pageLocked; // out of the tab order and unclickable while faded
    lockBtn.setAttribute('aria-hidden', String(!!pageLocked));
    ico.setAttribute('aria-hidden', String(!pageLocked));
    name.textContent = who ? `Signed in as ${who}` : '';
  }
  // Swaps the icon when the overlay's border starts to move.
  function setLocked(on) {
    lockBtn.replaceChildren(icon(on ? 'lock' : 'lock-open'));
  }
  return { root, update, setLocked };
}
