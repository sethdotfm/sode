// Pick mode for Recent: which rows can be picked and what they add.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem: () => {}, removeItem: () => {} };

const { indexRows, pickedEntries, MODES } = await import('./all.mjs');

const e = (id, src, dst, extra = {}) => ({ id, src, dst, levels: [1], ...extra });
const follow = e(11, 1, 8, { ext: true, follow: 3 });
const rows = [
  { e: e(10, 4, 4, { follows: [follow] }) },
  {
    f: {
      group: 'g',
      entries: [e(3, 1, 5), e(4, 1, 6)],
      shown: [e(3, 1, 5, { follows: [follow] }), e(4, 1, 6)],
      followers: [e(7, 2, 9, { ext: true, followGroup: 'g' })],
    },
  },
];
const index = indexRows(rows);

test('indexRows keys takes, folder takes and followers; a folder holds its entries', () => {
  assert.deepEqual([...index.keys()].sort(), ['e10', 'e3', 'e4', 'e7', 'fg']);
  assert.deepEqual(
    index.get('fg').entries.map((x) => x.id),
    [3, 4],
  );
});

test('a folder adds its takes, oldest first, and its follows only when asked', () => {
  assert.deepEqual(
    pickedEntries(index, new Set(['fg']), false).map((t) => t.dst),
    [5, 6],
  );
  assert.deepEqual(
    pickedEntries(index, new Set(['fg']), true).map((t) => t.dst),
    [5, 6, 9],
  );
});

test('a take adds its own follows when asked; the folder and one of its takes count once', () => {
  assert.deepEqual(
    pickedEntries(index, new Set(['e10']), false).map((t) => t.dst),
    [4],
  );
  assert.deepEqual(
    pickedEntries(index, new Set(['e10']), true).map((t) => t.dst),
    [4, 8],
  );
  assert.deepEqual(
    pickedEntries(index, new Set(['fg', 'e3']), false).map((t) => t.dst),
    [5, 6],
  );
});

test('entries that cannot be a take are left out; unknown keys are ignored', () => {
  assert.deepEqual(
    pickedEntries(index, new Set(['fg', 'nope']), false, (x) => x.dst !== 5).map((t) => t.dst),
    [6],
  );
  assert.deepEqual(pickedEntries(index, new Set(), true), []);
});

test('each mode says its own count, Add label and whether Add is on', () => {
  const taken = [{}, {}];
  const history = { picked: new Set(['a']) };
  assert.equal(MODES.history.count(history, taken), '1 chosen, 2 takes');
  assert.equal(MODES.history.count(history, [{}]), '1 chosen, 1 take');
  assert.equal(MODES.history.addText(history, taken), 'Add 2');
  assert.equal(MODES.history.addText(history, []), 'Add');
  assert.equal(MODES.history.canAdd(history, []), false);
  const boxes = { note: 'Added 1' };
  assert.equal(MODES.boxes.count(boxes, []), 'Added 1');
  assert.equal(MODES.boxes.addText(boxes, []), 'Add');
  assert.equal(MODES.boxes.canAdd(boxes, []), true);
  assert.equal(MODES.history.cancelText + MODES.boxes.cancelText, 'CancelDone');
});
