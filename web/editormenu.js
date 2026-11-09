// The smart menu: a small list beside the selection (right, else left, else below). It knows nothing about
// cells. The page gives it items, so Find, fill orders and later actions are added by handing it more.
// An item is { label, icon, run, letter?, disabled?, hint?, sub?: [{ label, run, current? }] }; letter is the key that
// activates it while focus is on the menu, and is shown beside it. Esc and the arrows are the menu's own.
import { el, icon } from './core.js';
import { placeMenu, menuTarget } from './editorrules.js';

const GAP = 8;

export function createSmartMenu(host, { onEscape }) {
  const root = el('ul', 'pb-menu sode-ed__menu');
  root.setAttribute('role', 'menu');
  root.hidden = true;
  document.body.append(root);

  // Opening a flyout means going into it; anything else runs.
  function activate(it) {
    if (!it.sub) return it.run();
    // The flyout shows on focus-within, so the item takes focus before its first entry can.
    const b = root.querySelector(`[data-letter="${it.letter}"]`);
    b?.focus();
    b?.parentElement.querySelector(':scope > .pb-menu__sub button')?.focus();
  }
  let current = [];

  const buttons = () => [...root.querySelectorAll(':scope > li > .pb-menu__item')];

  root.addEventListener('keydown', (e) => {
    const list = buttons();
    const i = list.indexOf(document.activeElement);
    if (e.key === 'Escape') {
      e.preventDefault();
      onEscape();
    } else if (e.key === (root.dataset.side === 'left' ? 'ArrowLeft' : 'ArrowRight') && i >= 0) {
      // The arrow toward the flyout goes into it.
      const into = list[i].parentElement.querySelector(':scope > .pb-menu__sub input, :scope > .pb-menu__sub button');
      if (into) {
        e.preventDefault();
        into.focus();
      }
    } else if (i >= 0 && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      // Only with an item focused: inside Find's box or a flyout, letters are typing.
      const it = menuTarget(current, e.key.toLowerCase());
      if (it) {
        e.preventDefault();
        activate(it);
      }
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (i < 0) return; // inside a flyout: it handles its own arrows
      e.preventDefault();
      list[(i + (e.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length]?.focus();
    }
  });

  function item(it, side) {
    const li = el('li');
    const b = el('button', 'pb-menu__item');
    b.type = 'button';
    b.setAttribute('role', 'menuitem');
    b.disabled = !!it.disabled;
    b.append(icon(it.icon), el('span', '', it.label));
    if (it.letter) {
      b.dataset.letter = it.letter;
      b.append(el('span', 'pb-menu__hint sode-ed__keys', it.letter.toUpperCase()));
    }
    if (it.panel) {
      // A flyout the page owns and keeps between rebuilds (Find's box and list), so it carries its own classes.
      b.setAttribute('aria-haspopup', 'true');
      b.append(el('span', 'pb-menu__hint', icon(side === 'left' ? 'chevron-left' : 'chevron-right')));
      it.panel.classList.toggle('pb-menu__sub--left', side === 'left');
      b.addEventListener('click', it.run);
      li.append(b, it.panel);
    } else if (it.sub) {
      b.setAttribute('aria-haspopup', 'true');
      b.append(el('span', 'pb-menu__hint', icon(side === 'left' ? 'chevron-left' : 'chevron-right')));
      const sub = el('ul', `pb-menu pb-menu__sub${side === 'left' ? ' pb-menu__sub--left' : ''}`);
      sub.setAttribute('role', 'menu');
      for (const s of it.sub) {
        const sb = el('button', 'pb-menu__item', s.icon ? icon(s.icon) : null, el('span', '', s.label));
        sb.type = 'button';
        sb.setAttribute('role', 'menuitem');
        if (s.current) sb.setAttribute('aria-current', 'true');
        sb.addEventListener('click', s.run);
        sub.append(el('li', '', sb));
      }
      li.append(b, sub);
    } else {
      b.addEventListener('click', it.run);
      li.append(b);
    }
    return li;
  }

  return {
    root,
    // box is the selection's screen box from the grid view; null hides the menu.
    show(box, items) {
      if (!box || !items.length) return void this.hide();
      const focused = root.contains(document.activeElement) ? [...root.querySelectorAll('.pb-menu__item')].findIndex((n) => n === document.activeElement) : -1;
      current = items;
      root.hidden = false;
      root.replaceChildren();
      const vp = { w: document.documentElement.clientWidth, h: document.documentElement.clientHeight };
      const size = { w: root.offsetWidth || 200, h: root.offsetHeight || items.length * 36 };
      let at = placeMenu(box, size, vp, GAP);
      root.replaceChildren(...items.map((it) => item(it, at.side)));
      size.w = root.offsetWidth;
      size.h = root.offsetHeight;
      at = placeMenu(box, size, vp, GAP);
      root.dataset.side = at.side;
      root.style.left = `${at.left}px`;
      root.style.top = `${at.top}px`;
      if (focused >= 0) root.querySelectorAll('.pb-menu__item')[focused]?.focus();
    },
    hide() {
      current = [];
      root.hidden = true;
    },
    focus(label) {
      buttons()
        .find((b) => !b.disabled && (!label || b.querySelector('span')?.textContent === label))
        ?.focus();
    },
  };
}
