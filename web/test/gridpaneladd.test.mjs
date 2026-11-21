// The Grid popup while the preset editor adds patches. It checks what a click on a name adds, the notes and the keys.
// It also checks the editor's check of new rows.
import test from 'node:test';
import assert from 'node:assert/strict';
import { dstNow, srcNow } from '../gridpanelnow.js';
import { addedLine, noRouteLine, rowsWord } from '../gridpoprules.js';
import { presetMarks, pressItems, addPolicy } from '../gridpaneladd.js';
import { gridKeyAction } from '../gridpanelkeys.js';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem: () => {}, removeItem: () => {} };
const { itemsToAdd, MODES } = await import('./all.mjs');

// dst -> level -> src, as envFrom makes it.
const tally = new Map([
  [
    5,
    new Map([
      [1, 3],
      [2, 3],
      [3, 7],
    ]),
  ],
  [6, new Map([[1, 3]])],
  [
    8,
    new Map([
      [1, 3],
      [2, 3],
    ]),
  ],
  [9, new Map([[1, -1]])],
]);

test('a destination adds its source now on the chosen levels, one row for each source', () => {
  assert.deepEqual(dstNow(tally, 5, [1, 2]), { items: [{ src: 3, dsts: [5], levels: [1, 2] }], outside: 0 });
  // A breakaway: video and audio from two sources make two rows.
  assert.deepEqual(dstNow(tally, 5, [3, 1, 2]).items, [
    { src: 3, dsts: [5], levels: [1, 2] },
    { src: 7, dsts: [5], levels: [3] },
  ]);
});

test('a destination skips levels with no source and sources outside the Rights', () => {
  assert.deepEqual(dstNow(tally, 9, [1, 2]), { items: [], outside: 0 });
  assert.deepEqual(dstNow(tally, 42, [1]), { items: [], outside: 0 });
  assert.deepEqual(
    dstNow(tally, 5, [1, 2, 3], (s) => s !== 7),
    { items: [{ src: 3, dsts: [5], levels: [1, 2] }], outside: 1 },
  );
});

test('a source adds every destination on the grid it goes to, one row for each set of levels', () => {
  assert.deepEqual(srcNow(tally, 3, [1, 2], [8, 6, 5, 9]).items, [
    { src: 3, dsts: [8, 5], levels: [1, 2] },
    { src: 3, dsts: [6], levels: [1] },
  ]);
  // Only the destinations the grid has (the Rights).
  assert.deepEqual(srcNow(tally, 3, [1, 2], [6]).items, [{ src: 3, dsts: [6], levels: [1] }]);
  assert.deepEqual(srcNow(tally, 7, [1, 2], [5, 6]).items, []);
});

const labels = { src: (n) => `S${n}`, dst: (n) => `D${n}` };

test('the popup says what a click added', () => {
  assert.equal(addedLine({ kind: 'xp', n: { src: 3, dst: 5 }, items: [{}] }, 1, labels), 'Added S3 to D5.');
  assert.equal(addedLine({ kind: 'dsts', n: 5, items: [{ src: 3 }], outside: 0 }, 1, labels), 'Added D5 from S3.');
  assert.equal(
    addedLine({ kind: 'dsts', n: 5, items: [{ src: 3 }, { src: 7 }], outside: 1 }, 2, labels),
    'Added D5 from 2 sources. 1 level left out: the source is not in your Rights.',
  );
  assert.equal(addedLine({ kind: 'srcs', n: 3, items: [{ dsts: [6] }] }, 1, labels), 'Added S3 to D6.');
  assert.equal(addedLine({ kind: 'srcs', n: 3, items: [{ dsts: [8, 5] }, { dsts: [6] }] }, 2, labels), 'Added S3 to 3 destinations (2 rows).');
});

test('the popup says why a name added nothing', () => {
  assert.equal(noRouteLine({ kind: 'dsts', n: 9, outside: 0 }, labels), 'D9 has no source on these levels.');
  assert.equal(noRouteLine({ kind: 'dsts', n: 5, outside: 2 }, labels), 'D5 gets its source from outside your Rights.');
  assert.equal(noRouteLine({ kind: 'srcs', n: 7 }, labels), 'S7 goes to no destination on these levels.');
});

const key = (k) => ({ key: k, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false });

test('with no sending, Enter presses as Space does and never confirms a take', () => {
  assert.equal(gridKeyAction(key('Enter'), { staged: 2, canSend: false, bar: null }), 'press');
  assert.equal(gridKeyAction(key('Enter'), { staged: 0, canSend: false, bar: 'dsts' }), 'pick');
  assert.equal(gridKeyAction(key('Enter'), { staged: 0, canSend: false, bar: 'corner' }), null);
  assert.equal(gridKeyAction(key('Enter'), { staged: 2, canSend: false, inControl: true }), null);
  assert.equal(gridKeyAction(key('Enter'), { staged: 2, bar: null }), 'confirm');
});

test('with no sending, the keys that stage a line, unstage, undo or confirm do nothing', () => {
  const c = { staged: 2, canSend: false, bar: null };
  for (const k of ['v', 'u', 'Delete', 'Backspace']) assert.equal(gridKeyAction(key(k), c), null, k);
  assert.equal(gridKeyAction(key('u'), { staged: 2, bar: null }), 'undo');
  assert.equal(gridKeyAction(key(' '), c), 'press');
  assert.equal(gridKeyAction(key('ArrowDown'), c), 'down');
});

test('the add policy adds what a press names, and its marks follow the rows after fresh()', () => {
  const w = { levels: [1, 2], tally, srcs: new Set([3]), dsts: [5, 6, 8] };
  assert.deepEqual(pressItems('xp', { src: 4, dst: 7 }, w), { items: [{ src: 4, dsts: [7], levels: [1, 2] }], outside: 0 });
  assert.deepEqual(pressItems('dsts', 5, { ...w, levels: [1, 2, 3] }), { items: [{ src: 3, dsts: [5], levels: [1, 2] }], outside: 1 });
  assert.deepEqual(pressItems('srcs', 3, w).items, [
    { src: 3, dsts: [5, 8], levels: [1, 2] },
    { src: 3, dsts: [6], levels: [1] },
  ]);
  let rows = [];
  const got = [];
  const p = addPolicy({ add: (r) => (got.push(r), { ok: true, text: 'ok' }), rows: () => rows, words: { name: 'in preset', route: ', in the preset' } });
  assert.deepEqual(p.press('xp', { src: 4, dst: 7 }, w), { ok: true, text: 'ok' });
  assert.equal(got[0].kind, 'xp');
  assert.deepEqual(got[0].levels, [1, 2]);
  assert.equal(p.marks().rows, 0);
  rows = [{ src: 4, dsts: [7], levels: [] }];
  assert.equal(p.marks().rows, 0);
  p.fresh();
  assert.equal(p.marks().xps.has('4,7'), true);
  assert.equal(p.marks().words.name, 'in preset');
});

const it = (src, dsts, levels = []) => ({ src, dsts, levels });

test('new rows: a row already there is left out, in any order', () => {
  assert.deepEqual(itemsToAdd([it(1, [2, 3], [1, 2])], [it(1, [3, 2], [2, 1]), it(4, [5])]), { items: [it(4, [5])], err: '' });
  assert.deepEqual(itemsToAdd([], [it(4, [5]), it(4, [5])]), { items: [it(4, [5])], err: '' });
  assert.deepEqual(itemsToAdd([it(4, [5])], [it(4, [5])]), { items: [], err: '' });
});

test('new rows: a clash or a cap they would make is said at once', () => {
  const clash = itemsToAdd([it(1, [5], [1])], [it(2, [5], [1])]);
  assert.match(clash.err, /gets both source/);
  assert.equal(itemsToAdd([it(1, [5], [1])], [it(2, [5], [2])]).err, '');
  const big = itemsToAdd(
    [],
    [
      it(
        1,
        Array.from({ length: 101 }, (_, k) => k),
        [1],
      ),
    ],
  );
  assert.match(big.err, /At most 100 destinations per source/);
});

test('in the boxes mode the bar has no Add of its own: Take is Add', () => {
  assert.equal(MODES.boxes.addShown, false);
  assert.notEqual(MODES.history.addShown, false);
});

test('the preset rows as grid marks and a count', () => {
  const m = presetMarks([
    { src: 3, dsts: [5, 8], levels: [1] },
    { src: 7, dsts: [5], levels: [2] },
  ]);
  assert.equal(m.rows, 2);
  assert.deepEqual([...m.xps], ['3,5', '3,8', '7,5']);
  assert.deepEqual([...m.srcs], [3, 7]);
  assert.deepEqual([...m.dsts], [5, 8]);
  assert.equal(rowsWord(1), '1 row in the preset');
  assert.equal(rowsWord(12), '12 rows in the preset');
});
