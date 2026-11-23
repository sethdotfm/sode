// A Palette row's ⋯ menu (palette.js): Pin or Unpin (Alt+P) and Bookmark or Remove bookmark (Alt+B). Each item shows an
// icon, its text and its key. The keys work on the selected row without the menu. The focus stays in the search box,
// so the menu's buttons take no focus, and Esc closes the menu before it clears the search.

import { el, icon } from './core.js';
import { keyLabel } from './keys.js';

// host: the element the menu sits in (position: relative). act(what, row): what is 'pin' or 'bookmark'.
export function rowMenu(host, act) {
  const menu = el('ul', { class: 'pb-menu sode-palette__menu', id: 'palette-menu', role: 'menu', 'aria-label': 'Row actions', hidden: true });
  host.append(menu);
  menu.addEventListener('mousedown', (e) => e.preventDefault());
  let row = null;
  let from = null;

  const item = (what, ic, text, key) => {
    const b = el(
      'button',
      { class: 'pb-menu__item', type: 'button', role: 'menuitem', tabIndex: -1, 'data-act': what },
      icon(ic),
      el('span', null, text),
      el('span', 'pb-menu__hint', keyLabel(key)),
    );
    b.addEventListener('click', () => {
      const r = row;
      close();
      act(what, r);
    });
    return el('li', { role: 'none' }, b);
  };

  function outside(e) {
    if (!menu.contains(e.target) && e.target !== from && !from?.contains(e.target)) close();
  }

  // state: { pinned, bookmarked, canBookmark }. button: the ⋯ button, which the menu hangs under.
  function open(button, r, { pinned, bookmarked, canBookmark }) {
    if (isOpen() && from === button) return close();
    close();
    row = r;
    from = button;
    menu.replaceChildren(
      item('pin', pinned ? 'pin-off' : 'pin', pinned ? 'Unpin' : 'Pin', 'alt+p'),
      ...(canBookmark ? [item('bookmark', bookmarked ? 'bookmark-minus' : 'bookmark-plus', bookmarked ? 'Remove bookmark' : 'Bookmark', 'alt+b')] : []),
    );
    menu.hidden = false;
    button.setAttribute('aria-expanded', 'true');
    // Under the button, right edges in line; above it when the host has no room below.
    const h = host.getBoundingClientRect();
    const b = button.getBoundingClientRect();
    const top = b.bottom - h.top;
    const up = top + menu.offsetHeight > host.clientHeight && b.top - h.top >= menu.offsetHeight;
    menu.style.top = `${up ? b.top - h.top - menu.offsetHeight : top}px`;
    menu.style.right = `${h.right - b.right}px`;
    document.addEventListener('pointerdown', outside, true);
  }

  function close() {
    if (!isOpen()) return false;
    menu.hidden = true;
    from?.setAttribute('aria-expanded', 'false');
    from = row = null;
    document.removeEventListener('pointerdown', outside, true);
    return true;
  }

  const isOpen = () => !menu.hidden;
  return { open, close, isOpen };
}
