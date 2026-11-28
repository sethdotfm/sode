// The Lock bar: the lock button with its drop-down (Lock, Sign out, Step in… or Step out) and "Signed in as <name>" as plain
// text. On Home, a guest gets Sign in. Settings > My user has the same actions (changepin.js); lock.js owns the state.

import { el, icon, on } from './core.js';
import { keyLabel } from './keys.js';
import { buildChangePin } from './changepin.js';

// Long enough to cross the gap from the button to the drop-down without it closing; short enough to feel direct.
const OPEN_MS = 120;
const CLOSE_MS = 250;

// api: { lock, signIn, signOut, stepIn, stepOut, home, stepped, stepOffered }. home: the main page, the only one with
// the chip's Sign in and Step in; a panel page has its own Sign in (#panels-signin). stepped and stepOffered are read
// at each open, since the drop-down is built once.
export function buildChip(api) {
  // Its only caller: it registers the changePin bus that Settings > My user uses.
  buildChangePin();
  const name = el('span', 'sode-lock__who-name');
  // One menu button: the lock icon and the chevron open the drop-down, and Lock is its first item. A lock icon that
  // locked on its own click looked like the drop-down's button. So a person did not see that Lock is a separate action.
  // Ctrl+L stays the fast way to lock.
  const lockIco = el('span', 'sode-lock__more-ico', icon('lock-open'));
  const lockBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm pb-tip--start sode-lock__more', lockIco, icon('chevron-down'));
  lockBtn.type = 'button';
  lockBtn.id = 'lock-chip-more';
  lockBtn.dataset.pbTip = 'Lock menu';
  lockBtn.setAttribute('aria-label', 'Lock menu');
  lockBtn.setAttribute('aria-haspopup', 'menu');
  lockBtn.setAttribute('aria-expanded', 'false');
  lockBtn.setAttribute('aria-controls', 'lock-chip-menu');
  // Status with words, cross-faded with the button (is-hand) so the icon does not snap against the lock screen's border.
  const ico = el('span', 'sode-lock__who-ico', icon('lock'));
  const slot = el('div', 'sode-lock__who-slot', lockBtn, ico);
  const signInBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode-lock__who-signin', 'Sign in');
  signInBtn.type = 'button';
  signInBtn.id = 'lock-chip-signin';
  signInBtn.hidden = true;
  signInBtn.addEventListener('click', () => api.signIn());

  const menu = el('ul', { class: 'pb-menu sode-lock__menu', id: 'lock-chip-menu', role: 'menu', 'aria-label': 'Lock menu', hidden: true });
  const item = (id, run) => {
    const ico2 = el('span', 'sode-lock__menu-ico');
    const label = el('span', 'sode-lock__menu-label');
    const hint = el('span', 'pb-menu__hint');
    const b = el('button', { class: 'pb-menu__item', type: 'button', role: 'menuitem', id, tabIndex: -1 }, ico2, label, hint);
    b.addEventListener('click', () => {
      close(true);
      run();
    });
    menu.append(el('li', { role: 'none' }, b));
    return { b, ico: ico2, label, hint };
  };
  const lock = item('lock-chip-lock', () => api.lock());
  lock.b.setAttribute('aria-keyshortcuts', 'Control+L');
  lock.ico.append(icon('lock'));
  lock.label.append('Lock');
  lock.hint.textContent = keyLabel('ctrl+l');
  // The click asks no Confirm, as Settings' Sign out: the drop-down is already a second, deliberate step. The note says
  // what Ctrl+Shift+L's Confirm says.
  const out = item('lock-chip-signout', () => api.signOut());
  out.ico.append(icon('log-out'));
  out.label.append('Sign out', el('span', 'sode-lock__menu-note', 'Signs out every tab that shares this sign-in'));
  out.hint.textContent = keyLabel('ctrl+shift+l');
  const step = item('lock-chip-stepin', () => (api.stepped() ? api.stepOut() : api.stepIn()));
  const group = el('div', 'sode-lock__who-lock', slot, menu);

  const root = el('div', 'sode-lock__who', el('div', 'sode-lock__who-bar', group, name, signInBtn));
  root.hidden = true;
  // The bar floats and takes no room. A page header keeps clear of its width (bar.css .sode-bar, and panels.css on a
  // panel page). px, not rem: a measured box, and the observer redoes it when the root font size changes the bar.
  new ResizeObserver(() => {
    document.documentElement.style.setProperty('--sode-who-w', `${Math.ceil(root.getBoundingClientRect().width)}px`);
  }).observe(root);

  const items = () => [...menu.querySelectorAll('.pb-menu__item')].filter((b) => !b.closest('li').hidden);
  const isOpen = () => !menu.hidden;
  let timer = 0;
  // Set when Esc, a pick or our own refocus closed the drop-down: the focus that follows must not open it again.
  // Cleared when the focus or the mouse leaves the group.
  let quiet = false;
  let user = false;
  // Hover opened it: the click a person makes a moment later on the button must not shut it again.
  let byHover = false;
  // The focus came by Tab. Other focus on the button (a click, or a dialog giving it back) must not open it.
  let tabbed = false;
  document.addEventListener('keydown', (e) => (tabbed = e.key === 'Tab'), true);
  document.addEventListener('pointerdown', () => (tabbed = false), true);
  const later = (fn, ms) => {
    clearTimeout(timer);
    timer = setTimeout(fn, ms);
  };
  function open(focusFirst) {
    clearTimeout(timer);
    if (lockBtn.inert) return;
    const stepped = api.stepped();
    // Stepped in, the server's userSignOut only steps out, so Sign out would repeat Step out.
    out.b.parentElement.hidden = stepped || !user;
    step.b.parentElement.hidden = !user || !api.home || !(stepped || api.stepOffered());
    step.ico.replaceChildren(icon(stepped ? 'user-round-minus' : 'user-round-check'));
    step.label.textContent = stepped ? 'Step out' : 'Step in…';
    step.hint.textContent = stepped ? keyLabel('ctrl+shift+l') : '';
    if (!isOpen()) {
      menu.hidden = false;
      lockBtn.setAttribute('aria-expanded', 'true');
      root.classList.add('is-menu');
      document.addEventListener('pointerdown', outside, true);
    }
    if (focusFirst) items()[0]?.focus();
  }
  function close(refocus) {
    clearTimeout(timer);
    byHover = false;
    if (!isOpen()) return;
    const inside = group.contains(document.activeElement);
    menu.hidden = true;
    lockBtn.setAttribute('aria-expanded', 'false');
    root.classList.remove('is-menu');
    document.removeEventListener('pointerdown', outside, true);
    if (refocus && inside && !lockBtn.inert) {
      quiet = true;
      lockBtn.focus({ preventScroll: true });
    }
  }
  function outside(e) {
    if (!group.contains(e.target)) close(false);
  }
  // Mouse only: a touch tap also sends pointerenter, and its click opens the drop-down.
  group.addEventListener('pointerenter', (e) => {
    if (e.pointerType !== 'mouse') return;
    if (isOpen()) clearTimeout(timer);
    else if (!quiet)
      later(() => {
        open(false);
        byHover = isOpen();
      }, OPEN_MS);
  });
  group.addEventListener('pointerleave', (e) => {
    if (e.pointerType !== 'mouse') return;
    quiet = false;
    // Keyboard focus inside keeps it open; the focusout below closes it.
    if (group.contains(document.activeElement) && document.activeElement.matches(':focus-visible')) clearTimeout(timer);
    else later(() => close(false), CLOSE_MS);
  });
  group.addEventListener('focusin', (e) => {
    if (!quiet && tabbed && e.target === lockBtn) open(false);
  });
  group.addEventListener('focusout', (e) => {
    if (group.contains(e.relatedTarget)) return;
    quiet = false;
    if (!group.matches(':hover')) close(false);
  });
  lockBtn.addEventListener('click', (e) => {
    quiet = false;
    if (isOpen() && byHover && e.pointerType === 'mouse') byHover = false;
    else if (isOpen()) close(false);
    // detail 0: Enter or Space, which moves the focus into the drop-down as a menu button does.
    else open(e.detail === 0);
  });
  // Stopped here so Home's keys and keyboard mode do not act on the drop-down's keys.
  group.addEventListener('keydown', (e) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const list = items();
    const i = list.indexOf(document.activeElement);
    if (e.key === 'Escape') {
      if (!isOpen()) return;
      close(false);
      quiet = true;
      if (i >= 0) lockBtn.focus({ preventScroll: true });
    } else if (i < 0 && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      quiet = false;
      open(true);
      if (e.key === 'ArrowUp') items().at(-1)?.focus();
    } else if (i >= 0 && e.key === 'Tab') {
      return close(false);
    } else {
      const to = i < 0 ? undefined : { ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: list.length - 1 }[e.key];
      if (to === undefined) return;
      list[(to + list.length) % list.length]?.focus();
    }
    e.preventDefault();
    e.stopPropagation();
  });
  on('locking', () => close(false));

  // pageLocked: the button and its drop-down do nothing, so they go. signIn: chipSignInShown in lockrules.js.
  function update(L, chip, pageLocked, whoName = L.user, signIn = false) {
    root.hidden = !chip;
    root.classList.toggle('is-hand', !!pageLocked);
    // out of the tab order and unclickable while faded
    lockBtn.inert = !!pageLocked;
    lockBtn.setAttribute('aria-hidden', String(!!pageLocked));
    ico.setAttribute('aria-hidden', String(!pageLocked));
    // A guest's drop-down has only Lock: Sign out and Step in need a user.
    user = !!L.user;
    const text = whoName ? `Signed in as ${whoName}` : '';
    // A new user (Step in, Step out) makes the open drop-down's items stale.
    if (!chip || pageLocked || name.textContent !== text) close(false);
    if (name.textContent !== text) name.replaceChildren(...(whoName ? ['Signed in as ', el('strong', 'sode-lock__who-user', whoName)] : []));
    signInBtn.hidden = !api.home || !signIn;
  }
  // Swaps the icon when the overlay's border starts to move.
  function setLocked(locked) {
    lockIco.replaceChildren(icon(locked ? 'lock' : 'lock-open'));
  }
  return { root, update, setLocked, closeMenu: () => close(false) };
}
