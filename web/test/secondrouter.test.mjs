import test from 'node:test';
import assert from 'node:assert/strict';

// Undo, Redo, Last and the History texts of an entry on a router after the first use that router's levels. Router A
// has one level; router B has four.

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
const doc = node();
globalThis.document = new Proxy(doc, { get: (t, k) => (k === 'baseURI' ? 'http://sode.test/' : doc[k]) });
const store = new Map();
globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };
let sock = null;
globalThis.WebSocket = class {
  static OPEN = 1;
  readyState = 1;
  constructor() {
    sock = this;
  }
  send() {}
  close() {}
};
globalThis.setInterval = () => 0;

const { connect, srcItem, dstItem, boxSearch, protectsOf } = await import('../core.js');
const { undoPlan, canRedo, againPlan, patchPlan, noteLast, lastTake, planBlocked, armedAdd } = await import('../plans.js');
const { beforeGroups, stackBefore, failureGroups } = await import('../history.js');

const level = (ui) => ({ ui, wire: ui - 1, label: String(ui) });
const hello = {
  t: 'hello',
  version: '',
  cfg: { filterSets: [], presets: [], bookmarks: [] },
  routers: [
    {
      uid: 'aaa',
      prefix: 'A',
      title: 'Main',
      numberOffset: 1,
      levels: [level(1)],
      srcs: [
        [0, 'CAM 1'],
        [22, 'Cam 9'],
      ],
      dsts: [
        [2, 'MON 3'],
        [3, 'MON 4'],
      ],
      tally: [],
      status: { router: 'up' },
    },
    {
      uid: 'bbb',
      prefix: 'B',
      title: 'Stage',
      numberOffset: 0,
      levels: [1, 2, 3, 4].map(level),
      srcs: [
        [4, 'B CAM'],
        [5, 'B CAM 2'],
      ],
      dsts: [
        [2, 'B MON'],
        [6, 'B MON 7'],
      ],
      tally: [],
      status: { router: 'up' },
      // B.7 has a Router lock from another device on level 2.
      protected: [[6, 2, 9, 2]],
      protectBy: [],
      protectDevice: 3,
    },
  ],
};

connect();
sock.onopen();
sock.onmessage({ data: JSON.stringify(hello) });

const bEntry = {
  id: 7,
  ok: true,
  router: 'bbb',
  src: 5,
  srcName: 'B CAM 2',
  dst: 2,
  dstName: 'B MON',
  levels: [1, 2, 3, 4],
  prev: [
    [1, 4],
    [2, 4],
    [3, 4],
    [4, 4],
  ],
};

test('Undo of a take on router B puts back all four of its levels', () => {
  const it = undoPlan(bEntry).items[0];
  assert.deepEqual(
    it.groups.map((g) => g.levels),
    [[1, 2, 3, 4]],
  );
  const failed = { ...bEntry, ok: false, failed: [3, 4], now: [[3, 4]] };
  assert.deepEqual(undoPlan(failed).items[0].untouched, [3]);
});

test('Redo and Retry of a take on router B keep its levels', () => {
  assert.equal(canRedo({ ...bEntry, levels: [3] }), true);
  assert.deepEqual(againPlan({ e: bEntry }).items[0].groups[0].levels, [1, 2, 3, 4]);
});

test('History texts of an entry on router B use its levels and names', () => {
  assert.deepEqual(
    beforeGroups(bEntry).map((g) => g.levels),
    [[1, 2, 3, 4]],
  );
  assert.equal(stackBefore(bEntry).text, 'B.4 B CAM');
  const failed = { ...bEntry, ok: false, failed: [2, 3], now: [[2, 4]] };
  assert.deepEqual(failureGroups(failed), [
    { levels: [2], text: 'still on B.4 B CAM' },
    { levels: [3], text: 'no answer' },
  ]);
});

test('Last of a take on router B keeps its levels', () => {
  assert.ok(noteLast(patchPlan(srcItem(5, '', 'bbb'), [dstItem(2, '', 'bbb')], [2, 4])));
  assert.deepEqual(lastTake().levels, [2, 4]);
});

test('with two routers, a take on a first router that is down names it', () => {
  sock.onmessage({ data: JSON.stringify({ t: 'status', router: 'down', routerUid: 'aaa' }) });
  assert.equal(planBlocked(patchPlan(srcItem(0), [dstItem(2)], [1])).block?.text, 'Sode is not connected to router A.');
  sock.onmessage({ data: JSON.stringify({ t: 'status', router: 'up', routerUid: 'aaa' }) });
});

test('the armed next add stays on the router of the last add', () => {
  const next = armedAdd(dstItem(2, '', 'bbb'), () => false);
  assert.deepEqual([next.n, next.router], [6, 'bbb']);
  assert.equal(
    armedAdd(dstItem(2, '', 'bbb'), (d) => d.n === 6 && d.router === 'bbb'),
    null,
  );
  assert.equal(armedAdd(dstItem(2), () => false).n, 3);
});

test('an edited picked label searches the router and number it names', () => {
  // A.23 Cam 9 is the label the box shows after a pick; the user deletes the end of it.
  const found = boxSearch('src', 'A.23 cam');
  assert.deepEqual(
    found.map((it) => it.label),
    ['A.23 Cam 9'],
  );
  assert.deepEqual(
    boxSearch('src', 'B.5 b cam').map((it) => it.label),
    ['B.5 B CAM 2'],
  );
  assert.equal(boxSearch('src', 'B.23 cam').length, 0, 'router B has no 23');
  assert.equal(boxSearch('src', 'C.23 cam').length, 0, 'no router C');
});

test('the page keeps the Protects of router B, and a take to a protected destination there is blocked', () => {
  assert.deepEqual([protectsOf('bbb').prot.get(6)?.get(2), protectsOf('bbb').device], [{ dev: 9, kind: 2 }, 3]);
  assert.equal(protectsOf('').prot.has(6), false);
  const plan = patchPlan(srcItem(5, '', 'bbb'), [dstItem(6, '', 'bbb')], [2]);
  assert.equal(planBlocked(plan).block?.title, 'Destination protected');
  // A protected message of router B frees it.
  sock.onmessage({ data: JSON.stringify({ t: 'protected', u: [[6, 2, -1, 2]], by: [], routerUid: 'bbb' }) });
  assert.equal(planBlocked(plan).block, null);
});
