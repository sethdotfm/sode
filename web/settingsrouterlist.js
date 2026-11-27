// Settings > Routers: the router list beside the detail view (settingsrouterdetail.js holds the routers and the
// selection). Each row has two lines of fixed height: the prefix, the Router title and the marks, then the address and
// the link state. A count line above the list gives the link states. With one router the list hides and the detail
// view fills the section. From FIND_FROM routers a find box and "Only routers with a problem" show. The list has a
// bounded height and a big scroll bar (.sode__rlist in SCROLLERS). Up and Down on the list move the selection. A drag
// of a row, or Alt+Up and Alt+Down, change the order; the first router is the default router. A filtered list does
// not reorder, since a row would land next to rows that it does not show.

import { $, el, on } from './core.js';
import {
  routerItems,
  selectedRouter,
  selectRouter,
  addRouter,
  linkOfRouter,
  errorsOf,
  routerEdited,
  stateNode,
  moveRouter,
  moveRouterTo,
} from './settingsrouterdetail.js';
import { readValues, countLine, findMatch, hasProblem, rowMarks, FIND_FROM } from './routerdetailrules.js';

const list = $('routers-list');
const find = $('routers-find');
const problems = $('routers-problems');

let canDrag = false;

function row(x, many) {
  const v = readValues(x.text);
  const s = x.isNew ? null : linkOfRouter(x);
  const marks = rowMarks({ first: x.first, many, isNew: x.isNew, removed: x.removed, edited: routerEdited(x) });
  const title = el('span', 'sode__rrow-title', v.title || `Router ${v.prefix}`);
  const line1 = el('span', 'sode__rrow-line', title, ...marks.map((m) => el('span', `${m.cls} sode__rrow-mark`, m.text)));
  const state = x.isNew ? el('span', 'sode__hint', 'Not saved') : s ? stateNode(s) : el('span');
  const line2 = el('span', 'sode__rrow-line sode__rrow-sub', el('span', 'sode__rrow-addr', v.address || 'No address'), state);
  const on = x.uid === selectedRouter();
  const n = el(
    'div',
    { class: 'sode__rrow', role: 'option', id: `router-row-${x.uid}`, tabIndex: on ? 0 : -1 },
    el('span', 'sode__rrow-prefix', v.prefix),
    el('span', 'sode__rrow-text', line1, line2),
  );
  n.dataset.router = x.uid;
  n.setAttribute('aria-selected', String(on));
  n.draggable = canDrag && !x.isNew;
  if (x.removed) n.dataset.removed = '';
  return n;
}

function shown(all) {
  const q = find.hidden ? '' : find.value;
  const only = !problems.closest('[hidden]') && problems.checked;
  return all.filter((x) => {
    const v = readValues(x.text);
    if (!findMatch(v, q)) return false;
    return !only || hasProblem(x.isNew ? null : linkOfRouter(x)?.key, Object.keys(errorsOf(x)).length);
  });
}

function draw() {
  const all = routerItems();
  const many = all.length > 1;
  $('routers-panes').classList.toggle('sode__rt--one', !many);
  $('routers-list-wrap').hidden = !many;
  $('routers-count').textContent = countLine(all.filter((x) => !x.isNew).map((x) => linkOfRouter(x)?.key || null));
  const findable = all.length >= FIND_FROM;
  find.hidden = !findable;
  $('routers-problems-wrap').hidden = !findable;
  if (!many) return void list.replaceChildren();
  const had = list.contains(document.activeElement) ? document.activeElement.dataset?.router : '';
  const rows = shown(all);
  canDrag = rows.length === all.length;
  if (!rows.length) {
    const clear = el('button', { class: 'pb-btn pb-btn--outline pb-btn--sm', type: 'button' }, 'Show all routers');
    clear.addEventListener('click', () => {
      find.value = '';
      problems.checked = false;
      draw();
    });
    list.replaceChildren(el('div', 'sode__rlist-empty', el('span', null, 'No router matches.'), clear));
  } else list.replaceChildren(...rows.map((x) => row(x, many)));
  if (had) list.querySelector(`[data-router="${CSS.escape(had)}"]`)?.focus({ preventScroll: true });
  list.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' });
}

function pick(uid, focus) {
  selectRouter(uid);
  if (focus) list.querySelector(`[data-router="${CSS.escape(uid)}"]`)?.focus({ preventScroll: true });
}

list.addEventListener('click', (e) => {
  const r = e.target.closest('.sode__rrow');
  if (r) pick(r.dataset.router, true);
});
list.addEventListener('keydown', (e) => {
  const r = e.target.closest('.sode__rrow');
  if (!r || e.ctrlKey || e.metaKey) return;
  if (e.altKey) {
    const step = { ArrowUp: -1, ArrowDown: 1 }[e.key];
    if (!step) return;
    e.preventDefault();
    const uid = r.dataset.router;
    if (canDrag) moveRouter(uid, step);
    list.querySelector(`[data-router="${CSS.escape(uid)}"]`)?.focus({ preventScroll: true });
    return;
  }
  const rows = [...list.querySelectorAll('.sode__rrow')];
  const i = rows.indexOf(r);
  const to = { ArrowUp: i - 1, ArrowDown: i + 1, Home: 0, End: rows.length - 1 }[e.key];
  if (to !== undefined) {
    e.preventDefault();
    if (rows[to]) pick(rows[to].dataset.router, true);
  } else if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault();
    pick(r.dataset.router, false);
    $('cfg-rt-title').focus();
  }
});

// A drag of a row puts it before or after the row under the pointer.
let dragUid = '';
const clearDrop = () => {
  for (const x of list.querySelectorAll('[data-drop]')) delete x.dataset.drop;
};
list.addEventListener('dragstart', (e) => {
  dragUid = e.target.closest?.('.sode__rrow')?.dataset.router || '';
  if (!dragUid) return;
  e.dataTransfer.effectAllowed = 'move';
  e.dataTransfer.setData('text/plain', dragUid);
});
list.addEventListener('dragover', (e) => {
  const r = e.target.closest('.sode__rrow');
  if (!r || !dragUid || !r.draggable) return;
  e.preventDefault();
  clearDrop();
  const box = r.getBoundingClientRect();
  r.dataset.drop = e.clientY < box.top + box.height / 2 ? 'before' : 'after';
});
list.addEventListener('drop', (e) => {
  const r = e.target.closest('.sode__rrow');
  if (!r || !dragUid) return;
  e.preventDefault();
  const after = r.dataset.drop === 'after';
  clearDrop();
  const uid = dragUid;
  dragUid = '';
  moveRouterTo(uid, r.dataset.router, after);
});
list.addEventListener('dragend', () => {
  dragUid = '';
  clearDrop();
});

// The find box and the filter are not settings: the form must not see their events (it would drop the exit warning).
for (const f of [find, problems])
  for (const ev of ['input', 'change'])
    f.addEventListener(ev, (e) => {
      e.stopPropagation();
      draw();
    });
find.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && find.value) {
    e.preventDefault();
    e.stopPropagation();
    find.value = '';
    draw();
  }
});
$('router-add').addEventListener('click', () => {
  find.value = '';
  problems.checked = false;
  addRouter();
});

on('routerRows', draw);
