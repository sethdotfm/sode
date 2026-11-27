// Settings > Routers: the rules of the detail view of one router (settingsrouterdetail.js) and of the router list
// (settingsrouterlist.js). Each router has the same settings, as the one Save keys router/<uid>/<attribute>. Pure
// (Node-tested).

import { parseLevelsAs, levelInputOf, levelLabelOf, levelsTextAs } from './levels.js';
import { routerLevelsOf } from './routerlevelrules.js';

// Every setting of a router, in the order of the detail view. Keep in step with ROUTER_SETTINGS in core.js and
// routerAttrs in internal/app/onesavefields.go.
export const DETAIL_ATTRS = Object.freeze([
  'prefix',
  'title',
  'address',
  'matrix',
  'nameRequests',
  'nameLength',
  'levels',
  'levelWireOffset',
  'levelNumbering',
  'defaultLevels',
  'numberOffset',
  'nameRereadSeconds',
  'fireMode',
  'salvoGroup',
  'tallyResyncSeconds',
  'protectDevice',
]);

export const ATTR_LABELS = Object.freeze({
  prefix: 'Prefix',
  title: 'Router title',
  address: 'Address',
  matrix: 'Matrix',
  nameRequests: 'Name requests',
  nameLength: 'Name character limit',
  levels: 'Levels',
  levelWireOffset: 'Level offset',
  levelNumbering: 'Level numbering',
  defaultLevels: 'Default levels',
  numberOffset: 'Source and destination numbers',
  nameRereadSeconds: 'Re-read names every (s)',
  fireMode: 'Multi destination',
  salvoGroup: 'Router salvo group',
  tallyResyncSeconds: 'Re-read routes every (s)',
  protectDevice: "Sode's device number",
});

const INTS = new Set([
  'matrix',
  'nameLength',
  'levels',
  'levelWireOffset',
  'numberOffset',
  'nameRereadSeconds',
  'salvoGroup',
  'tallyResyncSeconds',
  'protectDevice',
]);

// The settings of a router that Add router makes, as input text. Keep in step with the router of config.Defaults
// (internal/config/config.go); Sode's device number starts at 0, so Protect is off until a person sets it.
export const NEW_ROUTER = Object.freeze({
  prefix: '',
  title: '',
  address: '',
  matrix: '0',
  nameRequests: 'std',
  nameLength: '12',
  levels: '17',
  levelWireOffset: '-1',
  levelNumbering: '1',
  defaultLevels: '',
  numberOffset: '1',
  nameRereadSeconds: '60',
  fireMode: 'sequential',
  salvoGroup: '127',
  tallyResyncSeconds: '300',
  protectDevice: '0',
});

// levelLabel: the label of UI level ui in numbering mode ('1', '0' or 'va').
export const levelLabel = (ui, mode) => levelLabelOf([], mode, ui, mode);

// Saved levels (1 numbering, "1-3") in the numbering of the router: "V, 1-2". The box of Default levels and the Save
// review show it. Router.LevelsText in internal/config/config.go writes the same for the Settings saves record. Shared
// cases: web/test/levels_text_cases.json. Blank stays blank, and a text that does not parse stays as it is.
export function levelsText(saved, count, mode) {
  if (!String(saved || '').trim()) return '';
  const list = parseLevelsAs(String(saved), '1', Number.isInteger(count) && count >= 1 ? count : 256);
  return list ? levelsTextAs(list, mode) : saved;
}

// The default levels as the box shows them: in the numbering of the router.
export const defaultLevelsBox = (saved, count, mode) => levelsText(saved, count, mode);

// The default levels of the box as saved: '' for every level, the text in 1 numbering, or null when it does not parse.
export function readDefaultLevels(text, mode, count) {
  const t = String(text || '').trim();
  if (!t) return '';
  const list = Number.isInteger(count) && count >= 1 ? parseLevelsAs(t, mode, count) : null;
  return list ? levelInputOf(list, [], '1', '1') : null;
}

// After a change of Level numbering the box keeps the same levels, written in the new numbering. The level count can
// be mid-edit, so the levels are not limited here: Save checks them.
export function renumberDefault(text, from, to) {
  const list = String(text || '').trim() && parseLevelsAs(text, from, Infinity);
  return list ? levelsTextAs(list, to) : text;
}

// textsOf: the input text of each setting of router r of the hello's routers list (S.routers).
export function textsOf(r) {
  const lv = routerLevelsOf({ levels: r.levels, levelNumbering: r.levelNumbering, defaultLevels: '' });
  const count = lv.levels;
  const mode = r.levelNumbering || '1';
  const out = {};
  for (const a of DETAIL_ATTRS) out[a] = r[a] == null ? NEW_ROUTER[a] : String(r[a]);
  out.levels = String(count);
  out.levelWireOffset = String(r.levelWireOffset ?? lv.levelWireOffset);
  out.levelNumbering = mode;
  out.defaultLevels = defaultLevelsBox(r.defaultLevels, count, mode);
  return out;
}

// readValues: the values that the input text t saves, by attribute. A whole number that does not read is NaN.
export function readValues(t) {
  const out = {};
  for (const a of DETAIL_ATTRS) {
    const v = String(t[a] ?? '').trim();
    if (a === 'prefix') out[a] = v.toUpperCase();
    else if (INTS.has(a)) out[a] = v === '' || !/^-?\d+$/.test(v) ? NaN : Number(v);
    else out[a] = v;
  }
  out.defaultLevels = readDefaultLevels(t.defaultLevels, out.levelNumbering, out.levels);
  return out;
}

// The range of level numbers that a numbering shows for a level count: "1–17", "0–16" or "V, 1–16".
export function numberingRange(levels, mode) {
  const n = Number(levels);
  if (!Number.isInteger(n) || n < 2) return levelLabel(1, mode);
  if (mode === 'va') return n === 2 ? 'V, 1' : `V, 1–${n - 1}`;
  return `${levelLabel(1, mode)}–${levelLabel(n, mode)}`;
}

// The hint under Level offset: "Level 1 = level 0 on the router."
export function offsetNote(offset, mode) {
  const off = Number(offset);
  if (String(offset).trim() === '' || !Number.isInteger(off)) return '';
  return `Level ${levelLabel(1, mode)} = level ${1 + off} on the router.`;
}

// The hint under Default levels, when it has no error.
export function defaultNote(v) {
  if (v.defaultLevels === '') return 'Blank is every level, also after the level count changes.';
  if (!v.defaultLevels || !Number.isInteger(v.levels)) return '';
  const n = parseLevelsAs(v.defaultLevels, '1', v.levels)?.length || 0;
  return `${n} of ${v.levels} levels.`;
}

export const groupNote = (fireMode) => (fireMode === 'native' ? 'Pick a group that only Sode uses.' : 'Used only with Router salvo.');

const whole = (n, lo, hi) => Number.isInteger(n) && n >= lo && n <= hi;
const ADDRESS = /^(\[[^\]\s]+\]|[^\s:[\]]+):(\d{1,5})$/;

// fieldErrors: the error of each setting of values v, by attribute. Only settings with an error are in it. others: the
// prefixes of the other routers that stay after the save. Keep in step with Router.problems in
// internal/config/validate.go.
export function fieldErrors(v, others = []) {
  const e = {};
  if (!/^[A-Z]{1,2}$/.test(v.prefix)) e.prefix = 'Enter one or two letters, A to Z.';
  else if (others.includes(v.prefix)) e.prefix = `Another router has the prefix ${v.prefix}. Give each router its own prefix.`;
  if ([...v.title].length > 32) e.title = 'Enter at most 32 characters.';
  const at = v.address.match(ADDRESS);
  if (v.address && (!at || !whole(Number(at[2]), 1, 65535))) e.address = 'Enter the address as host:port, for example 10.0.0.50:8910.';
  if (!whole(v.matrix, 0, 255)) e.matrix = 'Enter a whole number from 0 to 255.';
  else if (v.nameRequests === 'std' && v.matrix > 15) e.matrix = 'Standard name requests allow matrix 0 to 15. Set Name requests to Extended.';
  else if (v.fireMode === 'native' && v.matrix > 15) e.matrix = 'Router salvo allows matrix 0 to 15. Set Multi destination to One after another.';
  if (![4, 8, 12, 16].includes(v.nameLength)) e.nameLength = 'Pick 4, 8, 12 or 16.';
  if (!whole(v.levels, 1, 256)) e.levels = 'Enter a whole number from 1 to 256.';
  if (!Number.isInteger(v.levelWireOffset)) e.levelWireOffset = 'Enter a whole number, for example -1.';
  else if (whole(v.levels, 1, 256) && (1 + v.levelWireOffset < 0 || v.levels + v.levelWireOffset > 255))
    e.levelWireOffset = `This puts the levels at ${1 + v.levelWireOffset} to ${v.levels + v.levelWireOffset} on the router. They must be in 0 to 255.`;
  if (v.defaultLevels === null)
    e.defaultLevels = `This is not a level or range in ${numberingRange(v.levels, v.levelNumbering)}. Use the level numbers of this router.`;
  if (!whole(v.nameRereadSeconds, 0, 86400) || (v.nameRereadSeconds > 0 && v.nameRereadSeconds < 10)) e.nameRereadSeconds = 'Enter 0 (off) or 10 to 86400.';
  if (!whole(v.salvoGroup, 0, 127)) e.salvoGroup = 'Enter a whole number from 0 to 127.';
  if (!whole(v.tallyResyncSeconds, 0, 86400) || (v.tallyResyncSeconds > 0 && v.tallyResyncSeconds < 30)) e.tallyResyncSeconds = 'Enter 0 (off) or 30 to 86400.';
  if (!whole(v.protectDevice, 0, 4095)) e.protectDevice = 'Enter 0 (off) or a number to 4095.';
  return e;
}

// A check of one setting also checks the settings that depend on it. Matrix depends on Name requests and Multi
// destination. Level offset and Default levels depend on Levels and Level numbering.
export const RECHECK = Object.freeze({
  nameRequests: ['matrix'],
  fireMode: ['matrix'],
  levels: ['levelWireOffset', 'defaultLevels'],
  levelNumbering: ['defaultLevels'],
});

// The words of a value in the review and in the notes.
const WORDS = {
  nameRequests: { std: 'Standard', ext: 'Extended' },
  fireMode: { sequential: 'One after another', native: 'Router salvo' },
  numberOffset: { 1: 'Start at 1', 0: 'Start at 0' },
};
export function valueText(attr, v, all = {}) {
  if (attr === 'levelNumbering') return numberingRange(all.levels, v);
  // Default levels show in the numbering the router has after the save, both was and now, so the two compare.
  if (attr === 'defaultLevels' && v != null) return v === '' ? 'Every level' : levelsText(v, all.levels, all.levelNumbering || '1');
  if (WORDS[attr]?.[v] !== undefined) return WORDS[attr][v];
  if (v === '' || v == null || Number.isNaN(v)) return 'Blank';
  return String(v);
}

// What Add router saves, in one line of the review.
export function addText(v) {
  const name = v.title || `Router ${v.prefix}`;
  const device = v.protectDevice ? `Sode's device number ${v.protectDevice}` : 'Protect is off';
  return `${name} at ${v.address || 'no address'}, matrix ${v.matrix}, ${v.levels} levels (${numberingRange(v.levels, v.levelNumbering)}), ${device}`;
}

// The list shows a find box and the problem filter from this many routers.
export const FIND_FROM = 8;

// The line above the list: "4 routers. 3 connected, 1 down." keys: the linkState key of each router. A key is null
// when Sode does not know the state (no server link, or a router to add).
export function countLine(keys) {
  if (keys.length < 2) return '';
  const n = (k) => keys.filter((x) => x === k).length;
  const head = `${keys.length} routers.`;
  if (keys.every((k) => !k)) return head;
  const parts = [`${n('connected')} connected`];
  if (n('down') + n('never')) parts.push(`${n('down') + n('never')} down`);
  if (n('connecting')) parts.push(`${n('connecting')} connecting`);
  return `${head} ${parts.join(', ')}.`;
}

// findMatch: the find box matches the prefix, the Router title or the address.
export const findMatch = (r, q) => !q.trim() || `${r.prefix} ${r.title} ${r.address}`.toLowerCase().includes(q.trim().toLowerCase());

// A router with a problem: a link that is not up, or a setting with an error.
export const hasProblem = (key, errors) => (!!key && key !== 'connected') || errors > 0;

// The marks of a list row, each {text, cls}. Default marks the first router while there are two or more. The others
// are unsaved work, so caution.
export function rowMarks({ first, many, isNew, removed, edited }) {
  const out = [];
  if (first && many) out.push({ text: 'Default', cls: 'pb-badge' });
  if (isNew) out.push({ text: 'New', cls: 'pb-badge pb-badge--warn' });
  else if (removed) out.push({ text: 'Remove on Save', cls: 'pb-badge pb-badge--warn' });
  else if (edited) out.push({ text: 'Edited', cls: 'pb-badge pb-badge--warn' });
  return out;
}

// The facts of the Protects group: Sode's and other devices' Protects on this router.
export function protectsText(device, sode, other) {
  if (!device) return '';
  const word = (n) => `${n} ${n === 1 ? 'Protect' : 'Protects'}`;
  return `Sode has ${word(sode)} on this router. Other devices have ${other}.`;
}

// The facts of the Tie lines group.
export function tiesText(from, to) {
  const n = from + to;
  return `${n} ${n === 1 ? 'tie line' : 'tie lines'}: ${from} from this router, ${to} to this router.`;
}
