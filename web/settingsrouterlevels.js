// Settings > Routers > Router levels: one block for each router after the first, with its Levels, Level offset, Level
// numbering and Default levels. A save source of the one Save (savesources.js), keys router/<uid>/<attribute>. The
// first router's level fields stay in its own block. Add router and Remove router are in settingsrouterlist.js.

import { $, S, el, on, routerData } from './core.js';
import { addSaveSource } from './savesources.js';
import { diffRows, showValue } from './savereviewrules.js';
import { routerDisplay } from './routerlistrules.js';
import {
  routerKey,
  routerLevelsOf,
  defaultLevelsText,
  readRouterLevels,
  routerFields,
  routerLevelsProblem,
  NUMBERING,
  numberingText,
} from './routerlevelrules.js';

const list = $('router-levels-list');
const note = $('router-levels-note');
const ATTRS = { levels: 'Levels', levelWireOffset: 'Level offset', levelNumbering: 'Level numbering', defaultLevels: 'Default levels' };

let base = {}; // {key: value} as loaded or saved
let vers = {};
let blocks = []; // [{uid, prefix, inputs: {levels, levelWireOffset, levelNumbering, defaultText}}]

const idOf = (uid, attr) => `cfg-rl-${uid}-${attr}`;

// input: the control the label names; wrap: what the field shows (the input in its wrapper).
function field(id, text, input, wrap = input) {
  const label = el('label', null, text);
  label.htmlFor = id;
  input.id = id;
  return el('div', 'sode__field', label, wrap);
}

function numberInput(min, max) {
  const f = el('input', 'pb-input');
  f.type = 'number';
  if (min != null) f.min = String(min);
  if (max != null) f.max = String(max);
  return f;
}

function block(r) {
  const levels = numberInput(1, 256);
  const offset = numberInput();
  const numbering = el('select', 'pb-select', ...NUMBERING.map(([v, t]) => Object.assign(el('option', null, t), { value: v })));
  const def = el('input', 'pb-input');
  def.spellcheck = false;
  const inputs = { levels, levelWireOffset: offset, levelNumbering: numbering, defaultText: def };
  const title = r.title ? `Router ${r.prefix}: ${r.title}` : `Router ${r.prefix}`;
  const node = el(
    'div',
    'sode__router-levels-item',
    el('div', 'sode__label', title),
    el(
      'div',
      'sode__fields',
      field(idOf(r.uid, 'levels'), 'Levels', levels, el('span', 'pb-number-wrap', levels)),
      field(idOf(r.uid, 'levelWireOffset'), 'Level offset', offset, el('span', 'pb-number-wrap', offset)),
      field(idOf(r.uid, 'levelNumbering'), 'Level numbering', numbering, el('span', 'pb-select-wrap', numbering)),
      field(idOf(r.uid, 'defaultLevels'), 'Default levels', def),
    ),
  );
  node.dataset.router = r.uid;
  return { uid: r.uid, prefix: r.prefix, name: routerDisplay(r), inputs, node };
}

const values = (b) =>
  readRouterLevels({
    levels: b.inputs.levels.value,
    levelWireOffset: b.inputs.levelWireOffset.value,
    levelNumbering: b.inputs.levelNumbering.value,
    defaultText: b.inputs.defaultText.value,
  });

function fill(b, v) {
  b.inputs.levels.value = String(v.levels);
  b.inputs.levelWireOffset.value = String(v.levelWireOffset);
  b.inputs.levelNumbering.value = v.levelNumbering;
  b.inputs.defaultText.value = defaultLevelsText(v);
}

function now() {
  const out = {};
  for (const b of blocks) Object.assign(out, routerFields(b.uid, values(b)));
  return out;
}
const edited = () => diffRows(base, now()).length > 0;

// A hello moves only a baseline with no edits on it, so another person's save shows as a conflict.
function load() {
  if (blocks.length && edited()) return;
  const more = S.routers.slice(1);
  blocks = more.map(block);
  base = {};
  vers = {};
  for (const b of blocks) {
    const v = routerLevelsOf({ ...routerData(b.uid), levelNumbering: routerData(b.uid).numbering });
    fill(b, v);
    Object.assign(base, routerFields(b.uid, v));
    for (const attr of Object.keys(ATTRS)) vers[routerKey(b.uid, attr)] = S.versions?.[routerKey(b.uid, attr)];
  }
  list.replaceChildren(...blocks.map((b) => b.node));
  const first = routerData('').prefix || 'A';
  note.textContent = more.length ? `The levels of router ${first} are in its own block below.` : 'Sode has one router. Its levels are in its own block below.';
}

on('hello', load);

const blockOf = (key) => blocks.find((b) => key.startsWith(`router/${b.uid}/`));
const attrOf = (key) => key.slice(key.lastIndexOf('/') + 1);

addSaveSource({
  owns: (key) => /^router\/[^/]+\/(levels|levelWireOffset|levelNumbering|defaultLevels)$/.test(key),
  base: () => base,
  now,
  version: (key) => vers[key],
  label: (r) => `${blockOf(r.key)?.name || 'Router'}: ${ATTRS[attrOf(r.key)]}`,
  show: (r, v) => (attrOf(r.key) === 'levelNumbering' ? numberingText(v) : showValue(v)),
  node: (key) => $(idOf(blockOf(key)?.uid, attrOf(key))),
  problem() {
    for (const b of blocks) {
      const p = routerLevelsProblem(b.name, values(b));
      if (p) return p;
    }
    return '';
  },
  saved(key, value, r) {
    base[key] = value;
    if (r.version) vers[key] = r.version;
  },
  conflict(key, r) {
    vers[key] = r.version;
    base[key] = r.value;
  },
  reset() {
    blocks = [];
    load();
  },
});
