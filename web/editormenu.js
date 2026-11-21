// The smart menu: a small list beside the selection (right, else left, else below). It knows nothing about cells: the page gives it items.
// An item is { label, icon, run, chord?, keys?, disabled?, sub?: [{ label, run, current?, disabled?, hint? }] }; chord is the letter that,
// with Ctrl (on a Mac too), runs it, and is shown beside it. keys is the text of another key that runs it, shown the same
// way. { divider: true } is a rule between items. Esc and the arrows are the menu's own.
// area() is the box the menu and its flyouts stay inside (editorrules.js menuArea); the viewport when not given. place
// is placeMenu's stand-in (the Live Editor's Smart menu sits above a Source bar name).
import { el, icon } from './core.js';
import { mac } from './keys.js';
import { placeMenu, subSide, flyoutShift, menuChord, chordText, EDGE_GAP as GAP } from './editorrules.js';

const viewport = () => ({ left: 0, top: 0, right: document.documentElement.clientWidth, bottom: document.documentElement.clientHeight });

export function createSmartMenu(host, { onEscape, area = viewport, place = placeMenu }) {
  const root = el('ul', 'pb-menu sode-ed__menu');
  root.setAttribute('role', 'menu');
  root.hidden = true;
  document.body.append(root);

  let current = [];
  let bounds = viewport();

  // Ctrl+letter: true when it ran an item. The grid's keydown calls this too.
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

  // Opens right; placeSubs turns a flyout left once the menu is placed and measured.
  function item(it) {
    if (it.divider) {
      const hr = el('li', 'pb-menu__divider');
      hr.setAttribute('role', 'separator');
      return hr;
    }
    const li = el('li');
    const b = el('button', 'pb-menu__item');
    b.type = 'button';
    b.setAttribute('role', 'menuitem');
    b.disabled = !!it.disabled;
    b.append(icon(it.icon), el('span', '', it.label));
    if (it.chord) b.append(el('span', 'pb-menu__hint sode-ed__keys', chordText(it.chord, mac)));
    else if (it.keys) b.append(el('span', 'pb-menu__hint sode-ed__keys', it.keys));
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
        sb.disabled = !!s.disabled;
        // A disabled button gets no pointer events, so a tip would never show: the reason is text beside it.
        if (s.hint) sb.append(el('span', 'pb-menu__hint', s.hint));
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

  // Every flyout opens away from the selection, or the other way when the widest has no room there. One side for all
  // of them, so the chevrons all point the same way. Widths are read with each shown but invisible for a moment, since
  // a closed one has none.
  function placeSubs(at) {
    const lis = [...root.children].filter((li) => li._sub);
    if (!lis.length) return;
    const w = Math.max(
      ...lis.map(({ _sub: sub }) => {
        sub.style.display = 'block';
        sub.style.visibility = 'hidden';
        const width = sub.offsetWidth;
        sub.style.display = '';
        sub.style.visibility = '';
        return width;
      }),
    );
    const side = subSide(at, root.offsetWidth, w, bounds, GAP);
    for (const li of lis) {
      li.dataset.subSide = side;
      li._sub.classList.toggle('pb-menu__sub--left', side === 'left');
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
      root.style.visibility = '';
      bounds = area();
      root.replaceChildren(...items.map(item));
      const at = place(box, { w: root.offsetWidth, h: root.offsetHeight }, bounds, GAP);
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
    // Places the shown menu at a new box without a rebuild, so focus and an open flyout stay. A null box hides it until
    // the next move or show.
    move(box) {
      if (root.hidden) return;
      root.style.visibility = box ? '' : 'hidden';
      if (!box) return;
      bounds = area();
      const at = place(box, { w: root.offsetWidth, h: root.offsetHeight }, bounds, GAP);
      root.dataset.side = at.side;
      root.style.left = `${at.left}px`;
      root.style.top = `${at.top}px`;
      placeSubs(at);
    },
    focus(label) {
      buttons()
        .find((b) => !b.disabled && (!label || b.querySelector('span')?.textContent === label))
        ?.focus();
    },
  };
}
