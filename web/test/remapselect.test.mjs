import { test } from 'node:test';
import assert from 'node:assert/strict';
import { newSelection, setKeys, move, toggle, click, dragFrom, dragTo, clear, picked } from '../remapselectrules.js';
import { remapRows, newChoices, bulk, applyChoice } from '../remaprules.js';

const sel = (keys) => {
  const s = newSelection();
  setKeys(s, keys);
  return s;
};

test('arrows move the cursor, Space selects, Shift extends', () => {
  const s = sel(['a', 'b', 'c', 'd']);
  move(s, 1);
  assert.equal(s.cursor, 0);
  toggle(s);
  move(s, 1, true);
  move(s, 1, true);
  assert.deepEqual(picked(s), ['a', 'b', 'c']);
  move(s, -1, true);
  assert.deepEqual(picked(s), ['a', 'b']);
  move(s, 1);
  assert.deepEqual(picked(s), ['a', 'b'], 'a move without Shift keeps the selection');
  toggle(s);
  assert.deepEqual(picked(s), ['a', 'b', 'c']);
  toggle(s);
  assert.deepEqual(picked(s), ['a', 'b']);
  move(s, 9);
  assert.equal(s.cursor, 3);
});

test('click selects one, Cmd/Ctrl-click adds, Shift-click extends from the anchor', () => {
  const s = sel(['a', 'b', 'c', 'd']);
  click(s, 'b');
  assert.deepEqual(picked(s), ['b']);
  click(s, 'd', { add: true });
  assert.deepEqual(picked(s), ['b', 'd']);
  click(s, 'd', { add: true });
  assert.deepEqual(picked(s), ['b']);
  click(s, 'b');
  click(s, 'd', { shift: true });
  assert.deepEqual(picked(s), ['b', 'c', 'd']);
  click(s, 'a', { shift: true });
  assert.deepEqual(picked(s), ['a', 'b']);
});

test('a drag selects the range it covers; Esc clears', () => {
  const s = sel(['a', 'b', 'c', 'd']);
  dragFrom(s, 'c');
  dragTo(s, 'a');
  assert.deepEqual(picked(s), ['a', 'b', 'c']);
  dragTo(s, 'd');
  assert.deepEqual(picked(s), ['c', 'd']);
  clear(s);
  assert.deepEqual(picked(s), []);
});

test('rows that leave the list leave the selection', () => {
  const s = sel(['a', 'b', 'c']);
  click(s, 'a');
  click(s, 'c', { add: true });
  setKeys(s, ['c', 'd']);
  assert.deepEqual(picked(s), ['c']);
  assert.equal(s.keys[s.cursor], 'c');
});

const items = (o) => Object.entries(o).map(([n, name]) => ({ n: +n, name }));
const ref = (kind, n, name) => ({ key: `f:${kind}:${n}`, kind, n, name, presets: 1 });
const tg = {
  uid: 'live',
  sources: items({ 1: 'CAM 01', 2: '', 4: 'VT 1', 9: 'X', 40: 'CAM 3' }),
  destinations: items({ 5: 'MON', 6: 'MON 1', 7: 'MON 1' }),
};
const rows = remapRows(
  {
    uid: 'f',
    refs: [ref('source', 3, 'CAM 1'), ref('source', 4, ''), ref('source', 2, ''), ref('source', 23, 'CAM 3'), ref('destination', 5, 'MON 1')],
  },
  tg,
);
const key = (kind, n) => `f:${kind}:${n}`;

test('bulk buttons with no selection change every row that needs a choice', () => {
  const ch = newChoices(rows);
  bulk(rows, ch, 'suggest');
  assert.deepEqual(ch[key('source', 3)], [1]);
  bulk(rows, ch, 'keepBlank');
  assert.deepEqual(ch[key('source', 4)], [4]);
  assert.equal(ch[key('source', 2)], undefined, 'a blank name over a blank target number imports as is');
  bulk(rows, ch, 'skip');
  for (const k of [key('source', 3), key('source', 4), key('destination', 5)]) assert.deepEqual(ch[k], [], k);
  assert.deepEqual(ch[key('source', 23)], [40], 'Reset to Skip leaves the automatic moves');
});

test('bulk buttons and a choice with rows selected change every selected row', () => {
  const ch = newChoices(rows);
  const some = new Set([key('source', 3), key('source', 23)]);
  bulk(rows, ch, 'skip', some);
  assert.deepEqual(ch[key('source', 23)], [], 'a selected moved row goes to Skip too');
  assert.deepEqual(ch[key('source', 4)], []);
  const all = new Set([key('source', 3), key('source', 4), key('destination', 5)]);
  applyChoice(rows, ch, key('source', 3), 'n:9', all);
  assert.deepEqual(ch[key('source', 3)], [9]);
  assert.deepEqual(ch[key('source', 4)], [9]);
  assert.deepEqual(ch[key('destination', 5)], [], 'a number goes only to rows of the same kind');
  applyChoice(rows, ch, key('destination', 5), 'all', all);
  assert.deepEqual(ch[key('destination', 5)], [6, 7]);
  assert.deepEqual(ch[key('source', 3)], [9], 'Add both only where the row offers it');
  applyChoice(rows, ch, key('source', 4), 'keep', all);
  assert.deepEqual(ch[key('source', 4)], [4]);
  assert.deepEqual(ch[key('source', 3)], [9], 'Keep number only on blank rows');
  applyChoice(rows, ch, key('source', 3), 'skip', all);
  for (const k of all) assert.deepEqual(ch[k], [], k);
  applyChoice(rows, ch, key('source', 3), 'n:2', new Set([key('source', 4)]));
  assert.deepEqual(ch[key('source', 3)], [2], 'a row outside the selection changes alone');
  assert.deepEqual(ch[key('source', 4)], []);
});
