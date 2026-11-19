// The Swap dialog (GLOSSARY.md, Swap). An Administrator opens it from Settings > Routers and stages one Swap into
// the one Save (savesources.js). A user opens it from My user, and it saves their own items at once. Rules in
// swaprules.js.
import { el, S, emit, off, on, onMsg, srcItem, dstItem, wsSend, sentence, LINK_DOWN } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { addSaveSource } from './savesources.js';
import { groupByOwner, parseShown, swapLabel, swapShow, swapValue } from './swaprules.js';

const KINDS = { preset: 'Preset', filterSet: 'Filter set', panel: 'Panel' };

let ui = null;
let mine = false;
let items = []; // the last list the server sent
let ticked = new Set();
let asked = null; // {side, from, to} of that list
let seq = 0;
let waiting = 0;
let staged = null; // {value, items}: the Swap that the next Save sends

addSaveSource({
  owns: (key) => key === 'swap',
  base: () => ({}),
  now: () => (staged ? { swap: staged.value } : {}),
  version: () => '',
  label: (r) => swapLabel(r.now || staged?.value || r.was, off()),
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

function button(id, label, cls, fn) {
  const b = el('button', `pb-btn ${cls}`, label);
  b.type = 'button';
  b.id = id;
  b.addEventListener('click', fn);
  return b;
}

function numberBox(id, label) {
  const input = el('input', 'pb-input');
  input.type = 'number';
  input.id = id;
  input.min = String(off());
  const name = el('span', 'sode__hint');
  const field = el('div', 'sode__field', Object.assign(el('label', null, label), { htmlFor: id }), el('span', 'pb-number-wrap', input), name);
  return { input, name, field };
}

const nameAt = (side, n) => (n == null ? '' : (side === 'source' ? srcItem(n) : dstItem(n)).name || 'No name');

function fields() {
  return { side: ui.side.value, from: parseShown(ui.from.input.value, off()), to: parseShown(ui.to.input.value, off()) };
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
  if (!asked || f.side !== asked.side || f.from !== asked.from || f.to == null || f.to === f.from) return null;
  return swapValue({ router: S.routerUid, side: f.side, from: f.from, to: f.to }, items, ticked, ui.broken.checked);
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
  if (!wsSend({ t: 'swapList', id: waiting, swap: { router: S.routerUid, side: f.side, from: f.from, mine } })) fail(LINK_DOWN);
}

onMsg('swapListResult', (m) => {
  if (!ui || m.id !== waiting) return;
  waiting = 0;
  if (!m.ok) return fail(m.err || 'The list did not load.');
  const f = fields();
  asked = { side: f.side, from: f.from };
  items = m.items || [];
  ticked = new Set(items.map((it) => it.id));
  paintList();
});

onMsg('swapMineResult', (m) => {
  if (!ui || m.id !== waiting) return;
  waiting = 0;
  ui.go.disabled = false;
  if (!m.ok) return fail(m.err || 'The Swap did not save.');
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
}

function build() {
  if (ui) return;
  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal sode__filters sode__swap');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'swap-title');
  dialog.tabIndex = -1;
  const title = Object.assign(el('span', 'pb-modal__title', 'Swap'), { id: 'swap-title' });
  const side = el('select', 'pb-select');
  side.id = 'swap-side';
  side.append(Object.assign(el('option', null, 'Destination'), { value: 'destination' }), Object.assign(el('option', null, 'Source'), { value: 'source' }));
  const sideField = el('div', 'sode__field', Object.assign(el('label', null, 'Swap a'), { htmlFor: 'swap-side' }), el('span', 'pb-select-wrap', side));
  const from = numberBox('swap-from', 'Old number');
  const to = numberBox('swap-to', 'New number');
  const broken = el('input', 'pb-check__input');
  broken.type = 'checkbox';
  broken.id = 'swap-broken';
  const list = el('div', 'sode__swap-list');
  list.setAttribute('aria-label', 'Items that use the old number');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const go = button('swap-go', 'Done', 'pb-btn--ok', done);
  dialog.append(
    el('div', 'pb-modal__header', title),
    el(
      'div',
      'pb-modal__body',
      el('div', 'sode__fields', sideField, from.field, to.field),
      button('swap-find', 'Find items', 'pb-btn--outline', find),
      el('label', 'pb-check', broken, el('span', 'pb-check__label', 'Only Broken')),
      list,
      err,
    ),
    el('div', 'pb-modal__footer', button('swap-cancel', 'Cancel', 'pb-btn--outline', close), go),
  );
  root.append(dialog);
  document.body.append(root);
  const dlg = trapDialog(root, dialog, close);
  ui = { dlg, title, side, from, to, broken, list, err, go };
  side.addEventListener('change', () => {
    asked = null;
    items = [];
    paintList();
  });
  from.input.addEventListener('input', paintNames);
  to.input.addEventListener('input', paintNames);
  from.input.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    find();
  });
  broken.addEventListener('change', paintList);
}

// opts.mine: the user's own items from My user, saved at once. Else everyone's, staged into the one Save.
function open(opts = {}) {
  build();
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
  ui.dlg.show();
  if (opts.side && opts.from != null) {
    find();
    ui.to.input.focus();
  } else ui.from.input.focus();
}

on('openSwap', open);
