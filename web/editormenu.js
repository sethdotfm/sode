// The smart menu: a small list beside the selection (right, else left, else below). It knows nothing about
// cells. The page gives it items, so Find, fill orders and later actions are added by handing it more.
// An item is { label, icon, run, disabled?, hint?, sub?: [{ label, run, current? }] }.
import { el, icon } from './core.js';
import { placeMenu } from './editorrules.js';

const GAP = 8;

export function createSmartMenu(host, { onEscape }) {
  const root = el('ul', 'pb-menu sode-ed__menu');
  root.setAttribute('role', 'menu');
  root.hidden = true;
  document.body.append(root);

  const buttons = () => [...root.querySelectorAll(':scope > li > .pb-menu__item')];

  root.addEventListener('keydown', (e) => {
    const list = buttons();
    const i = list.indexOf(document.activeElement);
    if (e.key === 'Escape') {
      e.preventDefault();
      onEscape();
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
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
    if (it.sub) {
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
      root.hidden = true;
    },
    focus() {
      buttons()
        .find((b) => !b.disabled)
        ?.focus();
    },
  };
}
