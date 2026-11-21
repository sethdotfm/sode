// Change my PIN and Sign out live in Settings > My user (changepin.js); lock.js owns the state.

import { el, icon } from './core.js';
import { keyLabel } from './keys.js';
import { buildChangePin } from './changepin.js';

export function buildChip(api) {
  buildChangePin();
  const name = el('span', 'sode-lock__who-name');
  const lockBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm pb-tip--start', icon('lock-open'));
  lockBtn.type = 'button';
  lockBtn.dataset.pbTip = `Lock (${keyLabel('ctrl+l')})`;
  lockBtn.setAttribute('aria-label', 'Lock');
  lockBtn.addEventListener('click', () => api.lock());
  // Status with words, cross-faded with the button (is-hand) so the icon does not snap against the lock screen's border.
  const ico = el('span', 'sode-lock__who-ico', icon('lock'));
  const slot = el('div', 'sode-lock__who-slot', lockBtn, ico);
  // Home only: a panel page has its own Sign in (#panels-signin).
  const home = !document.querySelector('.sode-panels__head, .sode-log__head');
  const signInBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode-lock__who-signin', 'Sign in');
  signInBtn.type = 'button';
  signInBtn.id = 'lock-chip-signin';
  signInBtn.hidden = true;
  signInBtn.addEventListener('click', () => api.signIn());
  const root = el('div', 'sode-lock__who', el('div', 'sode-lock__who-bar', slot, name, signInBtn));
  root.hidden = true;
  // The bar floats and takes no room. A page with a header keeps it clear of the bar's width (base.css .sode-bar, and
  // panels.css on a panel page). The main page has no such header, so it skips the observer. px, not rem: a measured box, and the observer redoes it when
  // the root font size changes the bar.
  if (!home)
    new ResizeObserver(() => {
      document.documentElement.style.setProperty('--sode-who-w', `${Math.ceil(root.getBoundingClientRect().width)}px`);
    }).observe(root);

  // pageLocked: the lock button does nothing, so it goes. signIn: chipSignInShown in lockrules.js.
  function update(L, chip, pageLocked, who = L.user, signIn = false) {
    root.hidden = !chip;
    root.classList.toggle('is-hand', !!pageLocked);
    lockBtn.inert = !!pageLocked; // out of the tab order and unclickable while faded
    lockBtn.setAttribute('aria-hidden', String(!!pageLocked));
    ico.setAttribute('aria-hidden', String(!pageLocked));
    name.textContent = who ? `Signed in as ${who}` : '';
    signInBtn.hidden = !home || !signIn;
  }
  // Swaps the icon when the overlay's border starts to move.
  function setLocked(on) {
    lockBtn.replaceChildren(icon(on ? 'lock' : 'lock-open'));
  }
  return { root, update, setLocked };
}
