// The Grid popup while the preset editor adds patches. It checks what a click on a name adds, the notes and the keys.
// It also checks the editor's check of new rows.
import test from 'node:test';
import assert from 'node:assert/strict';
import { dstNow, srcNow } from '../gridpanelnow.js';
import { addedLine, noRouteLine } from '../gridpoprules.js';
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

test('while adding, Enter adds as Space does and never confirms a take', () => {
  assert.equal(gridKeyAction(key('Enter'), { staged: 2, adding: true, bar: null }), 'press');
  assert.equal(gridKeyAction(key('Enter'), { staged: 0, adding: true, bar: 'dsts' }), 'pick');
  assert.equal(gridKeyAction(key('Enter'), { staged: 0, adding: true, bar: 'corner' }), null);
  assert.equal(gridKeyAction(key('Enter'), { staged: 2, adding: true, inControl: true }), null);
  assert.equal(gridKeyAction(key('Enter'), { staged: 2, adding: false, bar: null }), 'confirm');
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
