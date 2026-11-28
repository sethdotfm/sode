// Settings > Routers: the router list above the detail view (settingsrouterdetail.js holds the routers and the
// selection). It is a row of chips that wraps. A chip has the prefix, the Router title and the marks, then the address
// and the link state. The CSS shows the link state only when the router is not connected (data-link). A count line
// above gives the link states. With one router the list hides and the detail view fills the section. From FIND_FROM
// routers a find box and "Only routers with a problem" show at the end of the row. Left, Right, Up and Down move the
// selection. A drag of a chip, or Alt and an arrow, change the order; the first router is the default router. A
// filtered list does not reorder, since a router would land next to routers that it does not show.

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
import { rowStep, orderStep } from './routerlistrules.js';

const list = $('routers-list');
const find = $('routers-find');
const problems = $('routers-problems');

let canDrag = false;

function row(x, many) {
  const v = readValues(x.text);
  const s = x.isNew ? null : linkOfRouter(x);
  const marks = rowMarks({ first: x.first, many, isNew: x.isNew, removed: x.removed, edited: routerEdited(x) });
  // A long title is cut with an ellipsis; its title attribute shows it whole.
  const name = v.title || `Router ${v.prefix}`;
  const title = el('span', { class: 'sode__rrow-title', title: name }, name);
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
  n.dataset.link = x.isNew ? 'new' : s?.key || '';
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
  const saved = all.filter((x) => !x.isNew);
  $('routers-count').textContent = countLine(
    saved.map((x) => linkOfRouter(x)?.key || null),
    all.length - saved.length,
  );
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
    const step = orderStep(e.key);
    if (!step) return;
    e.preventDefault();
    const uid = r.dataset.router;
    if (canDrag) moveRouter(uid, step);
    list.querySelector(`[data-router="${CSS.escape(uid)}"]`)?.focus({ preventScroll: true });
    return;
  }
  const rows = [...list.querySelectorAll('.sode__rrow')];
  const to = rowStep(e.key, rows.indexOf(r), rows.length);
  if (to >= 0) {
    e.preventDefault();
    pick(rows[to].dataset.router, true);
  } else if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault();
    pick(r.dataset.router, false);
    $('cfg-rt-title').focus();
  }
});

// A drag of a router puts it before or after the router under the pointer.
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
  r.dataset.drop = e.clientX < box.left + box.width / 2 ? 'before' : 'after';
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
