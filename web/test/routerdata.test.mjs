import test from 'node:test';
import assert from 'node:assert/strict';

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

const { S, connect, routerData, routerOfText, routerOfItem, srcItem, dstItem, sameEnd } = await import('../core.js');
const { bySource, fedBy } = await import('../routes.js');
const { planBlocked, patchPlan, applyMyRights, rightsOff, lastFromPlan, noteLast, lastTake } = await import('../plans.js');

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
      levels: [level(1), level(2)],
      srcs: [[0, 'CAM 1']],
      dsts: [[2, 'MON 3']],
      tally: [[2, 1, 0]],
      status: { router: 'up', addr: '10.0.0.1:8910' },
    },
    {
      uid: 'bbb',
      prefix: 'B',
      title: 'Stage',
      numberOffset: 0,
      levels: [level(1)],
      srcs: [[4, 'B CAM']],
      dsts: [[2, 'B MON']],
      tally: [[2, 1, 4]],
      status: { router: 'down', addr: '10.0.0.2:8910', err: 'refused' },
    },
  ],
};
const send = (m) => sock.onmessage({ data: JSON.stringify(m) });

test('the hello gives each router its own names, tally, levels and link through routerData', () => {
  connect();
  sock.onopen();
  send(hello);
  assert.deepEqual(
    S.routers.map((r) => r.prefix),
    ['A', 'B'],
  );
  const a = routerData(''),
    b = routerData('bbb');
  assert.equal(routerData('aaa'), a, 'the first router by its UID');
  assert.equal(a.first, true);
  assert.deepEqual([a.uid, a.prefix, a.title, a.levels.length, a.link], ['aaa', 'A', 'Main', 2, 'up'], 'the first router comes from the list');
  assert.deepEqual([a.dstBy.get(2).label, b.dstBy.get(2).label], ['A.3 MON 3', 'B.2 B MON'], 'with two routers, the first router shows its prefix too');
  assert.deepEqual([a.tally.get(2).get(1), b.tally.get(2).get(1)], [0, 4]);
  assert.deepEqual([b.levels.length, b.link, b.err], [1, 'down', 'refused']);
  assert.equal(routerData('nope'), a, 'a UID that the hello did not list is the first router');

  assert.deepEqual(
    bySource(2, [1], 'bbb').map((r) => r.src),
    [4],
  );
  assert.deepEqual(fedBy(4, [1], 'bbb'), [2]);
  assert.deepEqual(fedBy(4, [1]), []);
});

test('tally, names and status messages go to the router they name', () => {
  send({ t: 'tally', u: [[2, 1, 7]], routerUid: 'bbb' });
  send({ t: 'tally', u: [[2, 2, 5]], routerUid: 'aaa' });
  send({ t: 'tally', u: [[3, 1, 1]] });
  assert.equal(routerData('bbb').tally.get(2).get(1), 7);
  assert.equal(routerData('').tally.get(2).get(1), 0, 'the first router keeps its own');
  assert.deepEqual([routerData('').tally.get(2).get(2), routerData('').tally.get(3).get(1)], [5, 1]);
  assert.equal(routerData('bbb').tally.has(3), false);

  send({ t: 'names', srcs: [[4, 'B CAM 2']], dsts: [], at: 't', routerUid: 'bbb' });
  assert.equal(routerData('bbb').srcBy.get(4).name, 'B CAM 2');
  assert.equal(routerData('').srcBy.get(0).name, 'CAM 1');

  send({ t: 'status', router: 'up', addr: '10.0.0.2:8910', routerUid: 'bbb' });
  assert.equal(routerData('bbb').link, 'up');
  assert.equal(routerData('').link, 'up');
});

test('items know their router, and a typed prefix picks the router', () => {
  const b = srcItem(4, '', 'bbb');
  assert.deepEqual([b.router, b.prefix, b.num, routerOfItem(b)], ['bbb', 'B', '4', 'bbb']);
  assert.equal(routerOfItem(srcItem(0)), '');
  assert.equal(routerOfItem(srcItem(0, '', 'aaa')), '', 'the first router sends no UID');
  assert.equal(sameEnd(dstItem(2), dstItem(2, '', 'bbb')), false);
  assert.equal(sameEnd(dstItem(2, '', 'bbb'), dstItem(2, 'x', 'bbb')), true);
  assert.deepEqual([routerOfText('B.4'), routerOfText('A.4'), routerOfText('4'), routerOfText('C.4')], ['bbb', '', '', '']);
});

test('a take on the second router needs its link; routes across routers open the Tie line picker', () => {
  const bSrc = srcItem(4, '', 'bbb'),
    bDst = dstItem(2, '', 'bbb');
  assert.equal(planBlocked(patchPlan(bSrc, [bDst], [1])).block, null);
  send({ t: 'status', router: 'down', addr: '10.0.0.2:8910', routerUid: 'bbb' });
  assert.equal(planBlocked(patchPlan(bSrc, [bDst], [1])).block?.text, 'Sode is not connected to router B.');
  assert.equal(planBlocked(patchPlan(srcItem(0), [dstItem(2)], [1])).block, null, 'the first router is still up');
  send({ t: 'status', router: 'up', addr: '10.0.0.2:8910', routerUid: 'bbb' });
  const one = planBlocked(patchPlan(srcItem(0), [bDst], [1]));
  assert.equal(one.block, null);
  assert.equal(one.tie, true);
  // A take with a plain route too opens the pickers, then the confirm.
  const two = planBlocked(patchPlan(srcItem(0), [bDst, dstItem(3)], [1]));
  assert.equal(two.block, null);
  assert.equal(two.tie, true);
  // An undo goes another way.
  const undo = patchPlan(srcItem(0), [bDst], [1]);
  undo.items[0].undo = { id: 1 };
  assert.equal(planBlocked(undo).block?.title, 'Route across routers');
});

test('a right for A.5 allows A.5 and refuses B.5, greyed and blocked', () => {
  applyMyRights({ sources: [{ router: 'aaa', n: 0 }], dsts: [{ router: 'aaa', n: 2 }] });
  assert.equal(rightsOff('sources', srcItem(0)), false);
  assert.equal(rightsOff('sources', srcItem(0, '', 'bbb')), true);
  assert.equal(rightsOff('dsts', dstItem(2, '', 'bbb')), true);
  assert.equal(planBlocked(patchPlan(srcItem(0), [dstItem(2)], [1])).block, null);
  const bSrc = srcItem(0, '', 'bbb'),
    bDst = dstItem(2, '', 'bbb');
  assert.equal(planBlocked(patchPlan(bSrc, [bDst], [1])).block?.title, 'Outside your Rights');
  applyMyRights(null);
});

test('a take across routers is Last with its typed ends: the source on A, the destination on B', () => {
  send({ t: 'names', srcs: [[4, 'B CAM 2']], dsts: [[2, 'B MON']], at: 't', routerUid: 'bbb' });
  const bDst = dstItem(2, '', 'bbb');
  const plan = patchPlan(srcItem(0), [bDst], [1]);
  assert.deepEqual(lastFromPlan(plan), { src: [0, 'CAM 1'], dsts: [[2, 'B MON']], levels: [1], dstRouter: 'bbb' });
  assert.ok(noteLast(plan));
  const t = lastTake();
  assert.equal(routerOfItem(t.src), '');
  assert.ok(sameEnd(t.dsts[0], bDst));
  // Last names one router for its destinations: a take with destinations on A and B is no Last.
  assert.equal(lastFromPlan(patchPlan(srcItem(0), [bDst, dstItem(2)], [1])), null);
  // A take on one router keeps the old form.
  assert.deepEqual(lastFromPlan(patchPlan(srcItem(0), [dstItem(2)], [1])), { src: [0, 'CAM 1'], dsts: [[2, 'MON 3']], levels: [1] });
});
