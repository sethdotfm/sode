// A Grid panel's Sources or Destinations list in the editor: the panel's items in drawn order, pinned first. Each row
// has a grip, Pin, Move and Remove. Add from the router is under it (editorrouterlist.js). The bar's buttons act on every
// selected row; the Cell tab sets their artwork through art(), and the bar's Artwork calls onArt to show it. While
// Follow the router is on (the Panel tab's switch), only Pin and artwork change it. Rules are editorgridrules.js; each
// edit is one ed.setFields of the edge.
import { el, icon } from './core.js';
import { axisSpan, OVER } from './gridpanelwindow.js';
import { btn } from './formbits.js';
import { createOrderedList } from './orderedlist.js';
import { createRouterList } from './editorrouterlist.js';
import { isGridPanel } from './gridpaneledges.js';
import { myRights, onMyRights } from './editorrights.js';
import { artNode } from './panelview.js';
import { plural } from './gridclip.js';
import { defaultFor } from './panellook.js';
import {
  edgeRows,
  clearEdge,
  selectRows,
  moveRows,
  stepRows,
  pinItems,
  removeItems,
  itemsArt,
  itemsHideText,
  drawnOrder,
  edgeListKey,
} from './editorgridrules.js';

const FIRST = 30; // rows drawn before the list has a height
const NONE = () => ({ selected: new Set(), anchor: null });
const MOVE_KEYS = { top: 'Alt+Home', up: 'Alt+Up, Ctrl+Up or Cmd+Shift+Up', down: 'Alt+Down, Ctrl+Down or Cmd+Shift+Down', end: 'Alt+End' };

// key is the panel field and the /names list ('sources' or 'destinations'). rights is the Rights key, what the item
// word, side the edge of the grid it is drawn on. onPick(list) runs when the selection changes.
export function createEdgeList(ed, { key, rights, what, side, onPick, onArt }) {
  const edge = () => ed.fields()[key];
  const router = () => ed.routerNames()?.[key] || [];
  const set = (next) => next !== edge() && ed.setFields({ [key]: next });
  const Whats = `${what[0].toUpperCase()}${what.slice(1)}s`;

  const count = el('span', 'sode-edge__count');
  count.id = `ed-${key}-state`;
  // The pinned count is also the switch that shows only the pinned rows.
  let pinOnly = false;
  const pinsBtn = btn('pb-btn--ghost pb-btn--sm sode-edge__pins', '', () => {
    pinOnly = !pinOnly;
    list.scrollTop = 0;
    paint(true);
  });
  pinsBtn.id = `ed-${key}-pins`;
  const FOLLOWING = `Follow the router is on for ${what}s, so the router sets the order. Pin and artwork still apply. Switch it off in the Panel tab.`;
  const following = el('p', 'sode__hint');
  following.id = `ed-${key}-following`;
  const clear = btn('pb-btn--outline pb-btn--sm', 'Clear', () => set(clearEdge(edge(), myRights(), rights)));
  clear.id = `ed-${key}-clear`;
  clear.dataset.pbTip = `Takes every ${what} off the panel.`;
  const outside = el('p', 'sode__hint');
  outside.id = `ed-${key}-outside`;
  const list = el('ol', 'sode-pages__list sode-edge__list');
  list.id = `ed-${key}-list`;
  list.tabIndex = 0;
  list.setAttribute('aria-label', `${Whats} on the panel, ${side}`);
  // A grid, not a listbox: each row holds buttons, and an option cannot. Rows carry aria-selected, and the
  // keyboard cursor is the active descendant, as editorrouterlist.js does for its listbox.
  list.setAttribute('role', 'grid');
  list.setAttribute('aria-multiselectable', 'true');
  const empty = el(
    'div',
    'pb-empty sode-edge__empty',
    el('p', 'pb-empty__title', `No ${what}s yet`),
    el('p', 'pb-empty__msg', 'Add them from the router, or switch on Follow the router in the Panel tab.'),
  );
  empty.id = `ed-${key}-empty`;
  const routerList = createRouterList({
    key,
    what,
    kind: rights,
    edge,
    router,
    rights: myRights,
    off: () => ed.routerNames()?.numberOffset || 0,
    readOnly: () => ed.readOnly(),
    fixed: () => fixed && FOLLOWING,
    set,
  });

  // The selection: router numbers, so it stays through moves and edits. cur is the keyboard's row.
  let sel = NONE();
  let cur = null;
  const many = () => sel.selected;

  const selNote = el('span', 'sode-edge__sel');
  selNote.id = `ed-${key}-sel`;
  selNote.setAttribute('role', 'status');
  selNote.dataset.pbTip = 'Click selects a row, Shift-click a range, Cmd- or Ctrl-click one more. In the list, Ctrl+A selects all and Esc none.';
  const barBtn = (id, name, tip, fn) => {
    const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', fn);
    b.append(icon(name));
    b.id = `ed-${key}-sel-${id}`;
    b.setAttribute('aria-label', tip.replace(/ \(.*\)$/, ''));
    b.dataset.pbTip = tip;
    tipOf.set(b, tip);
    return b;
  };
  const tipOf = new WeakMap();
  const pinSel = (on) => set(pinItems(edge(), router(), myRights(), rights, many(), on));
  const removeSel = () => {
    set(removeItems(edge(), many()));
    setSel(NONE());
  };
  const selArt = barBtn('art', 'image', 'Artwork: Icon A, Icon B and Hide text', () => onArt?.(api));
  const selPin = barBtn('pin', 'pin', 'Pin, in list order', () => pinSel(true));
  const selUnpin = barBtn('unpin', 'pin-off', 'Unpin', () => pinSel(false));
  const moves = [
    barBtn('top', 'arrow-up-to-line', `Move to the top (${MOVE_KEYS.top})`, () => moveSel('top')),
    barBtn('up', 'arrow-up', `Move up (${MOVE_KEYS.up})`, () => moveSel('up')),
    barBtn('down', 'arrow-down', `Move down (${MOVE_KEYS.down})`, () => moveSel('down')),
    barBtn('end', 'arrow-down-to-line', `Move to the end (${MOVE_KEYS.end})`, () => moveSel('end')),
  ];
  const selRemove = barBtn('remove', 'trash-2', 'Remove from the panel', removeSel);
  const acts = el(
    'div',
    'sode-edge__acts',
    selNote,
    el('span', 'sode-panels__spacer'),
    selArt,
    el('span', 'sode-edge__sep'),
    selPin,
    selUnpin,
    el('span', 'sode-edge__sep'),
    ...moves,
    el('span', 'sode-edge__sep'),
    selRemove,
  );
  acts.id = `ed-${key}-acts`;
  acts.setAttribute('role', 'toolbar');
  acts.setAttribute('aria-label', `Selected ${what}s`);
  const head = el('div', 'sode-edge__head', el('h2', 'sode-edge__h', Whats, count), pinsBtn, el('span', 'sode-panels__spacer'), clear);
  const on = el('section', 'sode-edge__part sode-edge__on', acts, following, outside, list, empty);
  on.setAttribute('aria-label', `${Whats} on the panel`);
  const node = el('section', 'sode-edge', head, on, routerList.node);
  node.id = `ed-${key}-col`;

  let shown = [];
  let at = new Map(); // router number -> index in shown
  // Pin and Remove on a row in a selection of several act on the whole selection, as Move does.
  const inMany = (n) => many().has(n) && many().size > 1;
  const rows = createOrderedList({
    rowClass: 'sode-pages__row sode-edge__row',
    toolsClass: 'sode-pages__tools',
    move: (from, to) => {
      const n = shown[from].n;
      if (inMany(n)) return moveSel(to < from ? 'up' : 'down');
      set(stepRows(edge(), router(), myRights(), rights, new Set([n]), to < from ? 'up' : 'down'));
    },
    // Dragging a selected row drags the selection; any other row goes alone.
    drop: (from, over, after) => {
      const ns = dragSet(from);
      const before = after ? (shown[over + 1]?.n ?? null) : shown[over].n;
      set(moveRows(edge(), router(), myRights(), rights, ns, before));
    },
    grip: (i) => {
      const ns = dragSet(i);
      return ns.size > 1 ? plural(ns.size, what) : shown[i].name || `${what} ${shown[i].n + shownOff}`;
    },
    dragging: (i) => {
      dragged = i < 0 ? null : dragSet(i);
      for (const li of made.values()) markRow(li);
    },
    remove: (i) => {
      const n = shown[i].n;
      set(removeItems(edge(), inMany(n) ? many() : new Set([n])));
      if (inMany(n)) setSel(NONE());
    },
    removeTip: 'Remove',
  });
  let dragged = null; // router numbers being dragged
  const dragSet = (i) => (many().has(shown[i].n) ? many() : new Set([shown[i].n]));

  function setSel(next, keepCur) {
    sel = next;
    if (!keepCur) cur = sel.anchor;
    drawSel();
    onPick?.(api);
  }
  function moveSel(dir) {
    if (!many().size || fixed) return;
    set(stepRows(edge(), router(), myRights(), rights, many(), dir));
    reveal(cur ?? [...many()][0]);
  }

  // The selection bar and the marks on the rows drawn. A selection change draws only this.
  function drawSel() {
    const ro = ed.readOnly();
    const n = many().size;
    selNote.textContent = `${n} selected`;
    for (const b of [selArt, selPin, selUnpin]) b.disabled = ro || !n;
    // A disabled button can show no tip in some browsers, so the note above the list says it too.
    for (const b of [selRemove, ...moves]) {
      b.disabled = ro || !n || fixed;
      b.dataset.pbTip = fixed ? FOLLOWING : tipOf.get(b);
    }
    for (const li of made.values()) markRow(li);
  }
  function markRow(li) {
    const n = Number(li.dataset.id);
    li.toggleAttribute('data-selected', many().has(n));
    li.setAttribute('aria-selected', String(many().has(n)));
    li.toggleAttribute('data-dragged', !!dragged?.has(n));
    li.classList.toggle('is-cur', n === cur);
    if (n === cur) list.setAttribute('aria-activedescendant', li.id);
    else if (list.getAttribute('aria-activedescendant') === li.id) list.removeAttribute('aria-activedescendant');
  }

  // Every Panel tab keystroke repaints the sidebar, and an edge can hold thousands of rows, so a repaint with the
  // same inputs draws nothing.
  let drawn = [];
  function paint(force) {
    const ro = ed.readOnly();
    // A Buttons panel never shows the list, so it holds no rows.
    if (!isGridPanel(ed.fields())) {
      drawn = [];
      shown = [];
      at = new Map();
      made = new Map();
      sel = NONE();
      return list.replaceChildren();
    }
    // Before the router list paints, since it reads fixed.
    fixed = !Array.isArray(edge()?.members);
    routerList.paint();
    const now = [edge(), router(), myRights(), ro, ed.routerNames(), ed.panel()?.id, ed.art(), pinOnly];
    if (!force && now.every((x, i) => x === drawn[i])) return;
    // Another panel starts with nothing selected.
    if (now[5] !== drawn[5]) sel = NONE();
    drawn = now;
    const off = ed.routerNames()?.numberOffset || 0;
    const v = edgeRows(edge(), router(), myRights(), rights);
    const all = drawnOrder(v.rows);
    const pinned = all.filter((r) => r.pinned).length;
    if (!pinned) pinOnly = false;
    shown = pinOnly ? all.slice(0, pinned) : all;
    at = new Map(shown.map((r, i) => [r.n, i]));
    // Rows that left the list leave the selection.
    if ([...sel.selected].some((x) => !at.has(x))) {
      sel = { selected: new Set([...sel.selected].filter((x) => at.has(x))), anchor: at.has(sel.anchor) ? sel.anchor : null };
      queueMicrotask(() => onPick?.(api));
    }
    if (!at.has(cur)) cur = sel.anchor;
    const n = v.rows.length;
    count.textContent = String(n);
    count.dataset.pbTip = `${plural(n, what)}, ${v.pins} pinned`;
    pinsBtn.hidden = !pinned;
    pinsBtn.textContent = `${pinned} pinned`;
    pinsBtn.setAttribute('aria-pressed', String(pinOnly));
    pinsBtn.dataset.pbTip = pinOnly ? `Shows every ${what} again` : `Shows only the pinned ${what}s`;
    outside.textContent = v.outside ? `${plural(v.outside, what)} outside your Rights, kept.` : '';
    outside.hidden = !v.outside;
    following.textContent = FOLLOWING;
    following.hidden = !fixed || !n;
    acts.hidden = list.hidden = !n;
    empty.hidden = !!n;
    clear.disabled = ro || fixed || !n;
    clear.dataset.pbTip = fixed ? FOLLOWING : `Takes every ${what} off the panel.`;
    shownOff = off;
    shownRo = ro;
    made = new Map();
    drawRows();
    drawSel();
  }

  // Only the rows in view have nodes, as in the router list. An edge can hold thousands, and each row has about 20
  // elements and 10 listeners. Rows are one height, so margins stand in for the rows not drawn.
  let shownOff = 0;
  let shownRo = false;
  let fixed = false; // Follow the router is on.
  let made = new Map(); // row index -> its li, for the rows of the last paint
  let pitch = 0; // px from the top of one row to the next
  let span = { lo: 0, hi: 0 };
  function makeRow(r, i) {
    const off = shownOff;
    const own = edge()?.items?.[r.n];
    const art = own?.art || own?.artB ? artNode({ art: own.art, artB: own.artB }, ed.art()) : null;
    const body = [
      el('span', 'sode-pages__num', String(r.n + off)),
      ...(art ? [el('span', 'sode-edge__art', art)] : []),
      el('span', 'sode-pages__name', r.name || `${what} ${r.n + off}`),
    ];
    if (r.gone) body.push(el('span', 'sode__hint', 'not on router'));
    // A Move across the line between pinned and unpinned rows does nothing (moveRows), so it is off.
    const li = rows.row({
      i,
      n: shown.length,
      id: String(r.n),
      readOnly: shownRo,
      fixed: fixed && FOLLOWING,
      upOff: !r.pinned && !!shown[i - 1]?.pinned,
      downOff: r.pinned && shown[i + 1] && !shown[i + 1].pinned,
      body: [el('div', 'sode-pages__head', ...body)],
      actions: [
        {
          icon: r.pinned ? 'pin-off' : 'pin',
          tip: r.pinned ? 'Unpin' : 'Pin to the edge',
          act: 'pin',
          run: () => set(pinItems(edge(), router(), myRights(), rights, inMany(r.n) ? many() : new Set([r.n]), !r.pinned)),
        },
      ],
    });
    li.querySelector('[data-act="pin"]').setAttribute('aria-pressed', String(r.pinned));
    li.id = `ed-${key}-row-${r.n}`;
    li.setAttribute('role', 'row');
    // The rows are windowed, so each says where it is in the whole list.
    li.setAttribute('aria-rowindex', String(i + 1));
    for (const c of li.children) if (!c.hasAttribute('aria-hidden')) c.setAttribute('role', 'gridcell');
    markRow(li);
    return li;
  }
  function drawRows() {
    const n = shown.length;
    list.setAttribute('aria-rowcount', String(n));
    // A hidden list has no height to measure, so it draws the first rows until it shows.
    if (!pitch && n && list.firstElementChild?.offsetHeight) {
      pitch = list.firstElementChild.offsetHeight + (parseFloat(getComputedStyle(list).rowGap) || 0);
    }
    const view = list.clientHeight;
    const next = pitch && view ? axisSpan({ n, pins: 0, size: pitch, head: 0, offset: list.scrollTop, view, over: OVER }) : { lo: 0, hi: Math.min(n, FIRST) };
    if (next.lo === span.lo && next.hi === span.hi && made.size) return;
    span = next;
    // A redraw replaces the buttons of a row it makes again, so focus goes back to the same act on the same item.
    // The item is found by its router number, since a move can put it at another index.
    const had = list.contains(document.activeElement) && document.activeElement !== list ? document.activeElement : null;
    const back = had && { act: had.dataset.act, id: had.closest('li')?.dataset.id };
    const keep = new Map();
    for (let i = next.lo; i < next.hi; i++) keep.set(i, made.get(i) || makeRow(shown[i], i));
    made = keep;
    // Margins, not the list's padding: with border-box sizing, padding past max-height makes the list taller.
    for (const li of keep.values()) li.style.margin = '';
    const rowsIn = [...keep.values()];
    if (pitch && rowsIn.length) {
      rowsIn[0].style.marginTop = `${next.lo * pitch}px`;
      rowsIn[rowsIn.length - 1].style.marginBottom = `${(n - next.hi) * pitch}px`;
    }
    list.replaceChildren(...rowsIn);
    const active = list.getAttribute('aria-activedescendant');
    if (active && !document.getElementById(active)) list.removeAttribute('aria-activedescendant');
    if (back && !list.contains(document.activeElement)) list.querySelector(`li[data-id="${back.id}"] [data-act="${back.act}"]`)?.focus();
  }

  // Scrolls the least that shows the item's row.
  function reveal(n) {
    const i = at.get(n);
    if (i === undefined || !pitch) return;
    const top = i * pitch;
    if (top < list.scrollTop) list.scrollTop = top;
    else if (top + pitch > list.scrollTop + list.clientHeight) list.scrollTop = top + pitch - list.clientHeight;
    drawRows();
  }

  // Shift-click would also mark the page's text.
  list.addEventListener('mousedown', (e) => e.shiftKey && !e.target.closest('button') && e.preventDefault());
  list.addEventListener('click', (e) => {
    const li = e.target.closest('li[data-id]');
    if (!li || e.target.closest('button')) return;
    const n = Number(li.dataset.id);
    cur = n;
    setSel(selectRows(sel, shown, n, { shift: e.shiftKey, toggle: e.metaKey || e.ctrlKey }), true);
    list.focus({ preventScroll: true });
  });
  list.addEventListener('keydown', (e) => {
    if (e.target !== list || !shown.length) return;
    const k = edgeListKey(e);
    if (!k) return;
    e.preventDefault();
    if (k.all) return setSel({ selected: new Set(shown.map((r) => r.n)), anchor: sel.anchor ?? shown[0].n }, true);
    if (k.none) return setSel(NONE(), true);
    if (k.move) return ed.readOnly() || fixed || moveSel(k.move);
    const i = at.get(cur) ?? -1;
    if (k.toggle) {
      if (i < 0) return;
      return setSel(selectRows(sel, shown, cur, { toggle: true }), true);
    }
    const page = Math.max(1, Math.floor(list.clientHeight / (pitch || 1)) - 1);
    const to = { up: i - 1, down: i + 1, home: 0, end: shown.length - 1, pageUp: i - page, pageDown: i + page }[k.go];
    const j = i < 0 ? 0 : Math.max(0, Math.min(shown.length - 1, to));
    const n = shown[j].n;
    const from = at.get(sel.anchor);
    cur = n;
    if (k.extend && from !== undefined) {
      const selected = new Set();
      for (let x = Math.min(from, j); x <= Math.max(from, j); x++) selected.add(shown[x].n);
      setSel({ selected, anchor: sel.anchor }, true);
    } else setSel({ selected: new Set([n]), anchor: n }, true);
    reveal(n);
  });
  list.addEventListener('scroll', drawRows, { passive: true });
  // Shown, resized or a new font size: the rows that fit, and their height, can change.
  new ResizeObserver(() => {
    pitch = 0;
    drawRows();
    if (pitch) drawRows();
  }).observe(list);
  onMyRights(() => paint());

  const api = {
    node,
    what,
    update: () => paint(),
    // The selected rows in the order the panel page draws them, so an Icon B run goes down the page.
    picked: () => drawnOrder(shown.filter((r) => many().has(r.n))),
    item: (n) => edge()?.items?.[n] || {},
    // One artwork choice (arttoolrules.js artRun) on the selected rows. Returns the rows past the end of an Icon B run.
    art(choice) {
      const out = itemsArt(
        edge(),
        api.picked().map((r) => r.n),
        choice,
      );
      set(out.edge);
      return out.left;
    },
    // Hide text on the selected rows, as the Cell tab sets it on cells.
    hideText(on) {
      const ns = api.picked().map((r) => r.n);
      set(itemsHideText(edge(), ns, on, (n) => !!defaultFor({ kind: what, n }, ed.defaults())));
    },
    clearPicks: () => many().size && setSel(NONE()),
    // The Live Editor's grid picks (editorlive.js): the same selection as a click here. Rows not in the list are left out.
    select(ns) {
      const keep = ns.filter((n) => at.has(n));
      setSel({ selected: new Set(keep), anchor: keep.at(-1) ?? null });
      if (keep.length) reveal(keep.at(-1));
    },
    // The selection bar. The Live Editor can show it over the grid while names are picked there.
    acts,
  };
  return api;
}
