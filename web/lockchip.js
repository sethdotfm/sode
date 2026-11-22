// The Lock bar: the lock button and "Signed in as <name>" with its menu. The menu has Sign out (Step out while stepped
// in), Step in… and Change PIN. On Home, a guest gets Sign in. Settings > My user has the same actions (changepin.js);
// lock.js owns the state.

import { el, icon, on } from './core.js';
import { keyLabel } from './keys.js';
import { buildChangePin } from './changepin.js';

// api: { lock, signIn, signOut, stepIn, changePin, home, stepped, stepOffered }. home: the main page, the only one with
// the chip's Sign in and Step in; a panel page has its own Sign in (#panels-signin). stepped and stepOffered are read
// at each open, since the menu is built once.
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
  const signInBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode-lock__who-signin', 'Sign in');
  signInBtn.type = 'button';
  signInBtn.id = 'lock-chip-signin';
  signInBtn.hidden = true;
  signInBtn.addEventListener('click', () => api.signIn());

  // The name stays the status text under the Lock; only the button look and the chevron fade (lock.css is-hand).
  const nameBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode-lock__who-btn', name, el('span', 'sode-lock__who-chev', icon('chevron-down')));
  nameBtn.type = 'button';
  nameBtn.id = 'lock-chip-name';
  nameBtn.setAttribute('aria-haspopup', 'menu');
  nameBtn.setAttribute('aria-expanded', 'false');
  nameBtn.setAttribute('aria-controls', 'lock-chip-menu');
  const menu = el('ul', { class: 'pb-menu sode-lock__menu', id: 'lock-chip-menu', role: 'menu', 'aria-labelledby': 'lock-chip-name', hidden: true });
  const item = (id, ico2, run) => {
    const label = el('span');
    const hint = el('span', 'pb-menu__hint');
    const b = el('button', { class: 'pb-menu__item', type: 'button', role: 'menuitem', id, tabIndex: -1 }, icon(ico2), label, hint);
    b.addEventListener('click', () => {
      close(false);
      run();
    });
    menu.append(el('li', { role: 'none' }, b));
    return { b, label, hint };
  };
  const out = item('lock-chip-signout', 'door-open', () => api.signOut());
  out.hint.textContent = keyLabel('ctrl+shift+l');
  const step = item('lock-chip-stepin', 'user', () => api.stepIn());
  step.label.textContent = 'Step in…';
  const pin = item('lock-chip-pin', 'key-round', () => api.changePin());
  pin.label.textContent = 'Change PIN';
  const who = el('div', 'sode-lock__who-menu', nameBtn, menu);

  const root = el('div', 'sode-lock__who', el('div', 'sode-lock__who-bar', slot, who, signInBtn));
  root.hidden = true;
  // The bar floats and takes no room. A page header keeps clear of its width (bar.css .sode-bar, and panels.css on a
  // panel page). px, not rem: a measured box, and the observer redoes it when the root font size changes the bar.
  new ResizeObserver(() => {
    document.documentElement.style.setProperty('--sode-who-w', `${Math.ceil(root.getBoundingClientRect().width)}px`);
  }).observe(root);

  const items = () => [...menu.querySelectorAll('.pb-menu__item')].filter((b) => !b.closest('li').hidden);
  const isOpen = () => !menu.hidden;
  function open(focusFirst) {
    const stepped = api.stepped();
    out.label.textContent = stepped ? 'Step out' : 'Sign out';
    step.b.parentElement.hidden = !api.home || !api.stepOffered();
    menu.hidden = false;
    nameBtn.setAttribute('aria-expanded', 'true');
    document.addEventListener('pointerdown', outside, true);
    if (focusFirst) items()[0]?.focus();
  }
  function close(refocus) {
    if (!isOpen()) return;
    menu.hidden = true;
    nameBtn.setAttribute('aria-expanded', 'false');
    document.removeEventListener('pointerdown', outside, true);
    if (refocus) nameBtn.focus({ preventScroll: true });
  }
  function outside(e) {
    if (!who.contains(e.target)) close(false);
  }
  // detail 0: Enter or Space, which moves the focus into the menu as a menu button does.
  nameBtn.addEventListener('click', (e) => (isOpen() ? close(false) : open(e.detail === 0)));
  nameBtn.addEventListener('keydown', (e) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    e.stopPropagation();
    open(true);
    if (e.key === 'ArrowUp') items().at(-1)?.focus();
  });
  // Stopped here so Home's keys and keyboard mode do not act on the menu's keys.
  menu.addEventListener('keydown', (e) => {
    const list = items();
    const i = list.indexOf(document.activeElement);
    const to = { ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: list.length - 1 }[e.key];
    if (e.key === 'Escape') close(true);
    else if (e.key === 'Tab') return close(false);
    else if (to !== undefined) list[(to + list.length) % list.length]?.focus();
    else return;
    e.preventDefault();
    e.stopPropagation();
  });
  on('locking', () => close(false));

  // pageLocked: the lock button and the name's menu do nothing, so they go. signIn: chipSignInShown in lockrules.js.
  function update(L, chip, pageLocked, whoName = L.user, signIn = false) {
    root.hidden = !chip;
    root.classList.toggle('is-hand', !!pageLocked);
    lockBtn.inert = !!pageLocked; // out of the tab order and unclickable while faded
    lockBtn.setAttribute('aria-hidden', String(!!pageLocked));
    ico.setAttribute('aria-hidden', String(!pageLocked));
    // Under the Lock the name is status only: a span's role, with no menu.
    nameBtn.inert = !!pageLocked;
    nameBtn.hidden = !whoName;
    name.textContent = whoName ? `Signed in as ${whoName}` : '';
    if (!chip || pageLocked || !L.user) close(false);
    signInBtn.hidden = !api.home || !signIn;
  }
  // Swaps the icon when the overlay's border starts to move.
  function setLocked(locked) {
    lockBtn.replaceChildren(icon(locked ? 'lock' : 'lock-open'));
  }
  return { root, update, setLocked };
}
