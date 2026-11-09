// The smart menu: a small list beside the selection (right, else left, else below). It knows nothing about
// cells. The page gives it items, so Find, fill orders and later actions are added by handing it more.
// An item is { label, icon, run, chord?, disabled?, sub?: [{ label, run, current? }] }; chord is the letter that,
// with Ctrl or Cmd, runs it, and is shown beside it. Esc and the arrows are the menu's own.
// area() is the box the menu and its flyouts stay inside (editorrules.js menuArea); the viewport when not given.
import { el, icon } from './core.js';
import { placeMenu, subSide, flyoutShift, menuChord, chordText } from './editorrules.js';

const GAP = 8; // keep in step with EDGE_GAP in editorfind.js
const MAC = /Mac|iPhone|iPad/.test(navigator.platform || '');
const viewport = () => ({ left: 0, top: 0, right: document.documentElement.clientWidth, bottom: document.documentElement.clientHeight });

export function createSmartMenu(host, { onEscape, area = viewport }) {
  const root = el('ul', 'pb-menu sode-ed__menu');
  root.setAttribute('role', 'menu');
  root.hidden = true;
  document.body.append(root);

  let current = [];
  let bounds = viewport();

  // Ctrl or Cmd+letter: true when it ran an item. The grid's keydown calls this too.
  const run = (e) => {
    const it = menuChord(current, e);
    if (it) it.run();
    return !!it;
  };

  const buttons = () => [...root.querySelectorAll(':scope > li > .pb-menu__item')];

  root.addEventListener('keydown', (e) => {
    const list = buttons();
    const i = list.indexOf(document.activeElement);
    if (e.key === 'Escape') {
      e.preventDefault();
      onEscape();
    } else if (i >= 0 && e.key === (list[i].parentElement.dataset.subSide === 'left' ? 'ArrowLeft' : 'ArrowRight')) {
      // The arrow toward the flyout goes into it.
      const into = list[i].parentElement.querySelector(':scope > .pb-menu__sub input, :scope > .pb-menu__sub button');
      if (into) {
        e.preventDefault();
        into.focus();
      }
    } else if (run(e)) {
      e.preventDefault();
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (i < 0) return; // inside a flyout: it handles its own arrows
      e.preventDefault();
      list[(i + (e.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length]?.focus();
    }
  });

  // A flyout the menu built (Type) is pulled back inside the area when it hangs off the bottom; Find fits itself.
  function fitSub(sub) {
    sub.style.translate = '';
    const r = sub.getBoundingClientRect();
    if (!r.width) return;
    const { x, y } = flyoutShift(r, { w: bounds.right, h: bounds.bottom }, GAP);
    if (x || y) sub.style.translate = `${x}px ${y}px`;
  }

  // Builds the item opening right; placeSubs turns a flyout to the left once the menu is placed and measured.
  function item(it) {
    const li = el('li');
    const b = el('button', 'pb-menu__item');
    b.type = 'button';
    b.setAttribute('role', 'menuitem');
    b.disabled = !!it.disabled;
    b.append(icon(it.icon), el('span', '', it.label));
    if (it.chord) b.append(el('span', 'pb-menu__hint sode-ed__keys', chordText(it.chord, MAC)));
    if (it.panel) {
      // A flyout the page owns and keeps between rebuilds (Find's box and list), so it carries its own classes.
      b.setAttribute('aria-haspopup', 'true');
      li._chevron = el('span', 'pb-menu__hint', icon('chevron-right'));
      b.append(li._chevron);
      li._sub = it.panel;
      b.addEventListener('click', it.run);
      li.append(b, it.panel);
    } else if (it.sub) {
      b.setAttribute('aria-haspopup', 'true');
      li._chevron = el('span', 'pb-menu__hint', icon('chevron-right'));
      b.append(li._chevron);
      const sub = el('ul', 'pb-menu pb-menu__sub');
      sub.setAttribute('role', 'menu');
      for (const s of it.sub) {
        const sb = el('button', 'pb-menu__item', s.icon ? icon(s.icon) : null, el('span', '', s.label));
        sb.type = 'button';
        sb.setAttribute('role', 'menuitem');
        if (s.current) sb.setAttribute('aria-current', 'true');
        sb.addEventListener('click', s.run);
        sub.append(el('li', '', sb));
      }
      li._sub = sub;
      li.addEventListener('pointerenter', () => fitSub(sub));
      li.addEventListener('focusin', () => fitSub(sub));
      li.append(b, sub);
    } else {
      b.addEventListener('click', it.run);
      li.append(b);
    }
    return li;
  }

  // A flyout opens away from the selection, or the other way when there is no room there. Its width is read with it
  // shown but invisible for a moment, since a closed one has none.
  function placeSubs(at) {
    for (const li of root.children) {
      const sub = li._sub;
      if (!sub) continue;
      sub.style.display = 'block';
      sub.style.visibility = 'hidden';
      const w = sub.offsetWidth;
      sub.style.display = '';
      sub.style.visibility = '';
      const side = subSide(at, root.offsetWidth, w, bounds, GAP);
      li.dataset.subSide = side;
      sub.classList.toggle('pb-menu__sub--left', side === 'left');
      li._chevron.replaceChildren(icon(side === 'left' ? 'chevron-left' : 'chevron-right'));
    }
  }

  return {
    root,
    run,
    // box is the selection's screen box from the grid view; null hides the menu.
    show(box, items) {
      if (!box || !items.length) return void this.hide();
      const focused = root.contains(document.activeElement) ? [...root.querySelectorAll('.pb-menu__item')].findIndex((n) => n === document.activeElement) : -1;
      current = items;
      root.hidden = false;
      bounds = area();
      root.replaceChildren(...items.map(item));
      const at = placeMenu(box, { w: root.offsetWidth, h: root.offsetHeight }, bounds, GAP);
      root.dataset.side = at.side;
      root.style.left = `${at.left}px`;
      root.style.top = `${at.top}px`;
      placeSubs(at);
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
