import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const setup = async () => {
  const m = await import('./all.mjs');
  const { S } = m;
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.srcBy = new Map([1, 2, 3].map((n) => [n, { n, num: n + 1, name: `SRC ${n + 1}`, label: `SRC ${n + 1}` }]));
  S.dstBy = new Map([10, 11].map((n) => [n, { n, num: n + 1, name: `DST ${n + 1}`, label: `DST ${n + 1}` }]));
  return m;
};

test('gridPlan: one item for each destination, a breakaway as two groups', async () => {
  const { gridPlan, S } = await setup();
  const plan = gridPlan(
    [
      { src: 1, dst: 10, levels: [1] },
      { src: 2, dst: 10, levels: [2] },
      { src: 3, dst: 11, levels: [1, 2] },
    ],
    ['a warning'],
  );
  assert.equal(plan.verb, 'grid');
  assert.deepEqual(plan.warnings, ['a warning']);
  const item = (dst, groups) => ({ dst: S.dstBy.get(dst), groups, skipped: [], untouched: [], undo: null, retry: null });
  assert.deepEqual(plan.items, [
    item(10, [
      { src: S.srcBy.get(1), levels: [1] },
      { src: S.srcBy.get(2), levels: [2] },
    ]),
    item(11, [{ src: S.srcBy.get(3), levels: [1, 2] }]),
  ]);
});

test('gridPlan with the patch boxes route: a mixed plan whose groups say which are staged', async () => {
  const { gridPlan } = await setup();
  const plan = gridPlan(
    [
      { src: 1, dst: 10, levels: [1], box: false },
      { src: 2, dst: 10, levels: [2], box: true },
      { src: 2, dst: 11, levels: [2], box: true },
    ],
    [],
  );
  assert.equal(plan.mixed, true);
  assert.deepEqual(
    plan.items.map((i) => i.groups.map((g) => [g.src.n, g.staged])),
    [
      [
        [1, true],
        [2, false],
      ],
      [[2, false]],
    ],
  );
  assert.equal(gridPlan([{ src: 3, dst: 11, levels: [1] }], []).mixed, undefined, 'staged routes alone are not mixed');
});
