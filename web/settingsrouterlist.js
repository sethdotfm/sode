// Settings > Routers: a row for each router after the first, with its Prefix, Title, Address and Matrix, plus Add
// router and Remove router. A save source of the one Save (savesources.js), keys of routerlistrules.js. The first
// router's row comes from settingsrouters.js; with two or more routers it gets a Prefix field from here. Remove
// router asks the server what it breaks before the router is staged; the save is Critical.

import { $, S, el, emit, on, request } from './core.js';
import { addSaveSource } from './savesources.js';
import { diffRows, showValue } from './savereviewrules.js';
import {
  ATTR_LABELS,
  LIST_ATTRS,
  LIST_KEY,
  addKey,
  freePrefix,
  keyAttr,
  keyRouter,
  listKey,
  readRouter,
  removeKey,
  routerListProblem,
  usesNote,
} from './routerlistrules.js';

let admin = false;
let base = {}; // {key: value} as loaded or saved
let vers = {};
let blocks = []; // The row of each router after the first.
let adds = []; // The row of each router to add.
const removed = new Map(); // A staged removal: router UID to the items it breaks.
let first = null; // The first router's Prefix field, with two or more routers.
let firstRow = null;
let addSeq = 0;

const idOf = (id, attr) => `cfg-rt-${id}-${attr}`;

function field(id, text, input, wrap = input) {
  const label = el('label', null, text);
  label.htmlFor = id;
  input.id = id;
  return el('div', 'sode__field', label, wrap);
}

function textInput(max, placeholder = '') {
  const f = el('input', 'pb-input');
  f.maxLength = max;
  f.spellcheck = false;
  f.placeholder = placeholder;
  return f;
}

function numberInput(min, max, placeholder = '') {
  const f = el('input', 'pb-input');
  f.type = 'number';
  f.min = String(min);
  f.max = String(max);
  f.placeholder = placeholder;
  return f;
}

function button(text, cls, fn) {
  const b = el('button', `pb-btn pb-btn--sm ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
}

// fields: the Prefix, Title, Address and Matrix fields of one row, and Levels for a new router.
function fields(id, withLevels) {
  const inputs = {
    prefix: textInput(2),
    title: textInput(32, 'Router title'),
    address: textInput(255, '10.0.0.50:8910'),
    matrix: numberInput(0, 255),
  };
  inputs.prefix.classList.add('sode__routers-prefix');
  const kids = [
    field(idOf(id, 'prefix'), 'Prefix', inputs.prefix),
    field(idOf(id, 'title'), 'Title', inputs.title),
    field(idOf(id, 'address'), 'Address', inputs.address),
    field(idOf(id, 'matrix'), 'Matrix', inputs.matrix, el('span', 'pb-number-wrap', inputs.matrix)),
  ];
  if (withLevels) {
    inputs.levels = numberInput(1, 256, '17');
    kids.push(field(idOf(id, 'levels'), 'Levels', inputs.levels, el('span', 'pb-number-wrap', inputs.levels)));
  }
  return { inputs, node: el('div', 'sode__fields', ...kids) };
}

const values = (x) => readRouter(Object.fromEntries(Object.entries(x.inputs).map(([k, f]) => [k, f.value])));

// block gives the row of router r. A row that is there already keeps its node, so an open note stays.
function block(r) {
  let b = blocks.find((x) => x.uid === r.uid);
  if (!b) {
    const { inputs, node } = fields(r.uid, false);
    b = { uid: r.uid, inputs, note: el('div', 'sode__routers-note'), title: el('span', 'sode__label') };
    b.remove = button('Remove…', 'pb-btn--outline', () => askRemove(b));
    b.node = el('li', 'sode__routers-item', el('div', 'sode__routers-head', b.title, b.remove), node, b.note);
    b.node.dataset.router = r.uid;
  }
  b.prefix = r.prefix;
  b.title.textContent = `Router ${r.prefix}`;
  b.inputs.prefix.value = r.prefix;
  b.inputs.title.value = r.title || '';
  b.inputs.address.value = r.address || '';
  b.inputs.matrix.value = String(r.matrix ?? 0);
  return b;
}

function addRow() {
  const x = { n: ++addSeq };
  const { inputs, node } = fields(`new${x.n}`, true);
  x.inputs = inputs;
  const used = [...S.routers.map((r) => r.prefix), ...blocks.map((b) => values(b).prefix), ...adds.map((a) => values(a).prefix)];
  if (first) used.push(readRouter({ prefix: first.input.value }).prefix);
  inputs.prefix.value = freePrefix(used);
  inputs.matrix.value = '0';
  const cancel = button('Cancel', 'pb-btn--ghost', () => {
    adds = adds.filter((a) => a !== x);
    paint();
    emit('saveSourceChanged');
  });
  x.node = el('li', 'sode__routers-item sode__routers-item--new', el('div', 'sode__routers-head', el('span', 'sode__label', 'New router'), cancel), node);
  adds.push(x);
  paint();
  emit('saveSourceChanged');
  inputs.address.focus();
}

// The note under a row: what Remove router breaks, and the buttons that stage or drop it.
function paintNote(b, items, staged) {
  const { head, lines } = usesNote(b.prefix, items);
  const list = lines.length ? [el('ul', 'sode__routers-uses', ...lines.map((t) => el('li', null, t)))] : [];
  const keep = button(staged ? `Keep router ${b.prefix}` : 'Keep', 'pb-btn--ghost', () => {
    removed.delete(b.uid);
    b.note.replaceChildren();
    paint();
    emit('saveSourceChanged');
  });
  const go = button(`Remove router ${b.prefix}`, 'pb-btn--outline', () => {
    removed.set(b.uid, items);
    paintNote(b, items, true);
    paint();
    emit('saveSourceChanged');
  });
  const when = staged ? [el('p', null, `Router ${b.prefix} goes when you save. The save asks for your PIN again.`)] : [];
  b.note.replaceChildren(
    el('div', 'pb-note pb-note--warn', ...when, el('p', null, head), ...list, el('div', 'sode__routers-acts', ...(staged ? [keep] : [go, keep]))),
  );
}

async function askRemove(b) {
  b.note.replaceChildren(el('p', 'sode__hint', `Looking for what names router ${b.prefix}…`));
  const m = await request('routerUsesResult', { t: 'routerUses', router: b.uid });
  if (!m.ok) {
    b.note.replaceChildren(el('div', 'pb-note pb-note--warn', m.err || 'The list did not load.'));
    return;
  }
  paintNote(b, m.items || [], false);
}

function now() {
  const out = {};
  if (first) out[listKey(first.uid, 'prefix')] = readRouter({ prefix: first.input.value }).prefix;
  for (const b of blocks) {
    const gone = removed.has(b.uid);
    const v = values(b);
    // A staged removal sends no edits of that router.
    for (const a of LIST_ATTRS) out[listKey(b.uid, a)] = gone ? base[listKey(b.uid, a)] : v[a];
    out[removeKey(b.uid)] = gone;
  }
  for (const x of adds) out[addKey(x.n)] = values(x);
  return out;
}
const edited = () => adds.length > 0 || diffRows(base, now()).length > 0;

// paint puts the first router's row and every other row in the list. The inputs keep their text.
function paint() {
  if (!firstRow) return;
  const many = S.routers.length + adds.length > 1;
  if (first) {
    first.wrap.hidden = !many;
    firstRow.append(first.wrap);
  }
  for (const b of blocks) {
    const gone = removed.has(b.uid);
    b.node.classList.toggle('sode__routers-item--removed', gone);
    for (const f of Object.values(b.inputs)) f.disabled = gone;
    b.remove.hidden = !admin || gone;
  }
  for (const x of adds) x.node.querySelector('.sode__routers-head .pb-btn').hidden = !admin;
  $('routers-list').replaceChildren(firstRow, ...blocks.map((b) => b.node), ...adds.map((x) => x.node));
  $('router-add').hidden = !admin;
}

// A hello moves only a baseline with no edits on it, so another person's save shows as a conflict.
function load(force = false) {
  if (!force && blocks.length + adds.length && edited()) return;
  const r0 = S.routers[0];
  first?.wrap.remove();
  first = null;
  base = {};
  vers = {};
  if (r0 && S.routers.length > 1) {
    const input = textInput(2);
    input.classList.add('sode__routers-prefix');
    input.value = r0.prefix;
    first = { uid: r0.uid, prefix: r0.prefix, input, wrap: field(idOf(r0.uid, 'prefix'), 'Prefix', input) };
    first.wrap.classList.add('sode__routers-first-prefix');
    const k = listKey(r0.uid, 'prefix');
    base[k] = r0.prefix;
    vers[k] = S.versions?.[k];
  }
  blocks = S.routers.slice(1).map(block);
  for (const b of blocks) {
    const v = values(b);
    for (const a of LIST_ATTRS) base[listKey(b.uid, a)] = v[a];
    base[removeKey(b.uid)] = false;
    for (const a of [...LIST_ATTRS, 'remove']) vers[listKey(b.uid, a)] = S.versions?.[listKey(b.uid, a)];
  }
  for (const b of blocks) if (removed.has(b.uid)) b.note.replaceChildren();
  removed.clear();
  paint();
}

// showRouterList: settingsrouters.js hands over the first router's row each time it draws it.
export function showRouterList(row) {
  firstRow = row;
  if (!blocks.length && !first) load(true);
  else paint();
}

on('hello', () => load());
on('access', (a) => {
  admin = a.role === 'administrator';
  paint();
});
$('router-add').addEventListener('click', addRow);

const blockOf = (key) => blocks.find((b) => b.uid === keyRouter(key));
const prefixOf = (key) => (first && keyRouter(key) === first.uid ? first.prefix : blockOf(key)?.prefix || '');

addSaveSource({
  owns: (key) => LIST_KEY.test(key),
  base: () => base,
  now,
  version: (key) => vers[key],
  label(r) {
    const attr = keyAttr(r.key);
    if (attr === 'add') return `New router ${now()[r.key]?.prefix || ''}`.trim();
    if (attr === 'remove') return `Remove router ${prefixOf(r.key)}`;
    return `Router ${prefixOf(r.key)}: ${ATTR_LABELS[attr]}`;
  },
  show(r, v) {
    const attr = keyAttr(r.key);
    if (attr === 'add') return v ? `${v.title || `Router ${v.prefix}`} at ${v.address || 'no address'}, matrix ${v.matrix}` : 'Blank';
    if (attr === 'remove') return v ? usesNote(prefixOf(r.key), removed.get(keyRouter(r.key)) || []).head : 'Kept';
    return showValue(v);
  },
  node(key) {
    const attr = keyAttr(key);
    if (attr === 'add') return adds.find((x) => addKey(x.n) === key)?.inputs.prefix;
    if (attr === 'remove') return blockOf(key)?.remove;
    return $(idOf(keyRouter(key), attr));
  },
  problem() {
    const rows = [];
    const r0 = S.routers[0];
    if (r0) rows.push({ name: `Router ${r0.prefix}`, prefix: first ? readRouter({ prefix: first.input.value }).prefix : r0.prefix, title: '', matrix: 0 });
    for (const b of blocks) if (!removed.has(b.uid)) rows.push({ name: `Router ${b.prefix}`, ...values(b) });
    for (const x of adds) rows.push({ name: 'The new router', ...values(x) });
    return routerListProblem(rows);
  },
  saved(key, value, r) {
    const attr = keyAttr(key);
    if (attr === 'add') adds = adds.filter((x) => addKey(x.n) !== key);
    else {
      base[key] = value;
      if (r.version) vers[key] = r.version;
    }
    // The hello with the new list came before this answer, so the rows follow it once the other fields are saved.
    if (attr === 'add' || attr === 'remove') queueMicrotask(() => load());
  },
  conflict(key, r) {
    vers[key] = r.version;
    base[key] = r.value;
  },
  reset() {
    adds = [];
    load(true);
  },
});
