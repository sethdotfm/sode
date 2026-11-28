// The Swap dialog (GLOSSARY.md, Swap). An Administrator opens it from Settings > Routers and stages one Swap into
// the one Save (savesources.js). A user opens it from their own Profile dialog, and it saves their own items at once. Rules in
// swaprules.js. With two or more routers the Router select picks the router whose numbers swap; each number is shown
// in that router's own numbering.
import { S, el, emit, on, onMsg, srcItem, dstItem, wsSend, sentence, LINK_DOWN, routerData } from './core.js';
import { routerDisplay } from './routerlistrules.js';
import { btn } from './formbits.js';
import { modal } from './modal.js';
import { addSaveSource } from './savesources.js';
import { firstTicked, groupByOwner, parseShown, swapLabel, swapShow, swapValue } from './swaprules.js';

const KINDS = { preset: 'Preset', filterSet: 'Filter set', panel: 'Panel', tieLine: 'Tie line' };

let ui = null;
let mine = false;
let items = []; // the last list the server sent
let ticked = new Set();
let asked = null; // {side, from, to} of that list
let seq = 0;
let back = null;
let waiting = 0;
let staged = null; // {value, items}: the Swap that the next Save sends

// The router in the dialog (a routerData UID: blank is the first router) and its number offset.
const picked = () => (ui?.router.value && ui.router.value !== routerData('').uid ? ui.router.value : '');
const off = (uid = picked()) => routerData(uid).off;
// A staged value's router: its shown name with two or more routers.
const nameOf = (uid) => (S.routers.length > 1 ? routerDisplay(S.routers.find((r) => r.uid === uid) || routerData(uid)) : '');
const offOf = (uid) => routerData(uid === routerData('').uid ? '' : uid).off;

addSaveSource({
  owns: (key) => key === 'swap',
  base: () => ({}),
  now: () => (staged ? { swap: staged.value } : {}),
  version: () => '',
  label: (r) => {
    const v = r.now || staged?.value || r.was;
    return swapLabel(v, offOf(v.router), nameOf(v.router));
  },
  show: (_r, v) => swapShow(v, staged?.items || []),
  problem: () => '',
  saved: () => {
    staged = null;
  },
  conflict: () => {},
  reset: () => {
    staged = null;
  },
});

function numberBox(id, label) {
  const input = el('input', 'pb-input');
  input.type = 'number';
  input.id = id;
  input.min = String(off());
  const name = el('span', 'sode__hint');
  const field = el('div', 'sode__field', Object.assign(el('label', null, label), { htmlFor: id }), el('span', 'pb-number-wrap', input), name);
  return { input, name, field };
}

const nameAt = (side, n) => (n == null ? '' : (side === 'source' ? srcItem(n, '', picked()) : dstItem(n, '', picked())).name || 'No name');

function fields() {
  return { router: routerData(picked()).uid, side: ui.side.value, from: parseShown(ui.from.input.value, off()), to: parseShown(ui.to.input.value, off()) };
}

function paintNames() {
  const f = fields();
  ui.from.name.textContent = nameAt(f.side, f.from);
  ui.to.name.textContent = nameAt(f.side, f.to);
  const v = current();
  ui.go.disabled = !v;
  const n = v?.items.length || 0;
  ui.go.textContent = mine ? 'Swap and save' : `Stage ${n} ${n === 1 ? 'item' : 'items'}`;
}

function fail(text) {
  ui.err.textContent = text ? sentence(text) : '';
  ui.err.hidden = !text;
}

// The value for the list on screen, or null while the numbers differ from the ones it was found for.
function current() {
  const f = fields();
  if (!asked || f.router !== asked.router || f.side !== asked.side || f.from !== asked.from || f.to == null || f.to === f.from) return null;
  return swapValue({ router: f.router, side: f.side, from: f.from, to: f.to }, items, ticked, ui.broken.checked);
}

function paintList() {
  const groups = groupByOwner(items, ui.broken.checked);
  const kids = [];
  for (const g of groups) {
    kids.push(el('h4', 'sode__swap-owner', g.owner));
    const ul = el('ul', 'sode__swap-items');
    for (const it of g.items) {
      const box = el('input', 'pb-check__input');
      box.type = 'checkbox';
      box.checked = ticked.has(it.id);
      box.addEventListener('change', () => {
        if (box.checked) ticked.add(it.id);
        else ticked.delete(it.id);
        paintNames();
      });
      const label = el('label', 'pb-check', box, el('span', 'pb-check__label', `${KINDS[it.kind] || it.kind}: ${it.name}`));
      const li = el('li', null, label);
      if (it.broken) li.append(el('span', 'pb-badge pb-badge--warn', 'Broken'));
      if (it.kind === 'tieLine') li.append(el('span', 'sode__hint', 'A tie line is a cable. Tick it only if the cable moved.'));
      ul.append(li);
    }
    kids.push(ul);
  }
  if (asked && !groups.length) kids.push(el('p', 'sode__hint', ui.broken.checked ? 'No Broken items' : 'No items'));
  ui.list.replaceChildren(...kids);
  paintNames();
}

function find() {
  const f = fields();
  if (f.from == null) return fail('Enter the old number.');
  fail('');
  waiting = ++seq;
  if (!wsSend({ t: 'swapList', id: waiting, swap: { router: f.router, side: f.side, from: f.from, mine } })) fail(LINK_DOWN);
}

onMsg('swapListResult', (m) => {
  if (!ui || m.id !== waiting) return;
  waiting = 0;
  if (!m.ok) return fail(m.err || 'List not loaded.');
  const f = fields();
  asked = { router: f.router, side: f.side, from: f.from };
  items = m.items || [];
  ticked = new Set(firstTicked(items));
  paintList();
});

on('wsclose', () => {
  if (!ui || !waiting || !mine) return;
  waiting = 0;
  ui.go.disabled = false;
  fail(LINK_DOWN);
});

onMsg('swapMineResult', (m) => {
  if (!ui || m.id !== waiting) return;
  waiting = 0;
  ui.go.disabled = false;
  if (!m.ok) return fail(m.err || 'Swap not saved.');
  emit('swapDone', m.count);
  close();
});

function done() {
  const v = current();
  if (!v) return;
  if (mine) {
    waiting = ++seq;
    ui.go.disabled = true;
    if (!wsSend({ t: 'swapMine', id: waiting, swap: v })) {
      ui.go.disabled = false;
      fail(LINK_DOWN);
    }
    return;
  }
  staged = { value: v, items: items.slice() };
  emit('saveSourceChanged');
  close();
}

function close() {
  ui?.dlg.hide();
  waiting = 0;
  if (back?.isConnected) back.focus();
  back = null;
}

function build() {
  if (ui) return;
  const router = el('select', { class: 'pb-select', id: 'swap-router' });
  const routerField = el('div', 'sode__field', Object.assign(el('label', null, 'Router'), { htmlFor: 'swap-router' }), el('span', 'pb-select-wrap', router));
  const side = el('select', { class: 'pb-select', id: 'swap-side' });
  side.append(Object.assign(el('option', null, 'Destination'), { value: 'destination' }), Object.assign(el('option', null, 'Source'), { value: 'source' }));
  const sideField = el('div', 'sode__field', Object.assign(el('label', null, 'Swap'), { htmlFor: 'swap-side' }), el('span', 'pb-select-wrap', side));
  const from = numberBox('swap-from', 'Old number');
  const to = numberBox('swap-to', 'New number');
  const broken = el('input', { class: 'pb-check__input', type: 'checkbox', id: 'swap-broken' });
  const list = el('div', { class: 'sode__swap-list', 'aria-label': 'Items that use the old number' });
  const err = el('div', { class: 'pb-note pb-note--error', role: 'alert', hidden: true });
  const go = btn('sode-btn--primary', '', done, 'swap-go');
  const dlg = modal({
    titleId: 'swap-title',
    title: 'Swap',
    cls: 'sode__filters sode__swap',
    body: [
      el('div', 'sode__fields', routerField, sideField, from.field, to.field),
      el(
        'div',
        'sode__ptools sode__swap-find',
        btn('pb-btn--outline', 'Find items', find, 'swap-find'),
        el('label', 'pb-check', broken, el('span', 'pb-check__label', 'Only Broken')),
      ),
      list,
      err,
    ],
    foot: [btn('pb-btn--outline', 'Cancel', close, 'swap-cancel'), go],
    onEsc: close,
  });
  ui = { dlg, title: dlg.title, router, routerField, side, from, to, broken, list, err, go };
  const restart = () => {
    asked = null;
    items = [];
    for (const b of [ui.from, ui.to]) b.input.min = String(off());
    paintList();
  };
  side.addEventListener('change', restart);
  router.addEventListener('change', restart);
  from.input.addEventListener('input', paintNames);
  to.input.addEventListener('input', paintNames);
  from.input.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    find();
  });
  broken.addEventListener('change', paintList);
}

// The Router select: every router of the hello, shown only with two or more. want is a router UID; blank or unknown is
// the first router.
function fillRouters(want) {
  const list = S.routers.length ? S.routers : [{ uid: routerData('').uid, prefix: routerData('').prefix, title: routerData('').title }];
  ui.router.replaceChildren(...list.map((r) => Object.assign(el('option', null, routerDisplay(r)), { value: r.uid })));
  ui.router.value = list.some((r) => r.uid === want) ? want : list[0].uid;
  ui.routerField.hidden = list.length < 2;
}

// opts.mine: the user's own items from their Profile dialog, saved at once. Else everyone's, staged into the one Save.
// opts.router: the router to start on (a router UID), else the first router.
function open(opts = {}) {
  build();
  fillRouters(opts.router);
  mine = !!opts.mine;
  ui.title.textContent = mine ? 'Swap in my items' : 'Swap source or destination';
  asked = null;
  items = [];
  ticked = new Set();
  ui.broken.checked = false;
  for (const b of [ui.from, ui.to]) {
    b.input.value = '';
    b.input.min = String(off());
  }
  fail('');
  // opts.side and opts.from (internal): a Broken item's reference, from the Broken banner or count list.
  if (opts.side && opts.from != null) {
    ui.side.value = opts.side;
    ui.from.input.value = String(opts.from + off());
  }
  paintList();
  back ||= document.activeElement;
  ui.dlg.show();
  if (opts.side && opts.from != null) {
    find();
    ui.to.input.focus();
  } else ui.from.input.focus();
}

on('openSwap', open);
