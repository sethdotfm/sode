import test from 'node:test';
import assert from 'node:assert/strict';

// The level texts of an entry, a plan or a Protect on a router after the first use that router's levels and Level
// numbering. Router A has two levels numbered from 1; router B has four numbered from 0.

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
const doc = node();
globalThis.document = new Proxy(doc, { get: (t, k) => (k === 'baseURI' ? 'http://sode.test/' : doc[k]) });
const store = new Map();
globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };
let sock = null;
const sent = [];
globalThis.WebSocket = class {
  static OPEN = 1;
  readyState = 1;
  constructor() {
    sock = this;
  }
  send(m) {
    sent.push(JSON.parse(m));
  }
  close() {}
};
globalThis.setInterval = () => 0;

const { connect, S, srcItem, dstItem } = await import('../core.js');
const { undoneNote, followNames } = await import('../history.js');
const { failureBanner, undoNotes, undoPlan, lastChip, patchPlan, noteLast, lastTake } = await import('../plans.js');
const { protectPlan, sendProtect, everyHold } = await import('../protects.js');

// label: the server's Level.Label in the router's Level numbering.
const level = (ui, from = 1) => ({ ui, wire: ui - 1, label: String(ui - 1 + from) });
const hello = {
  t: 'hello',
  version: '',
  cfg: { numberOffset: 1, routerName: 'Main', filterSets: [], presets: [], bookmarks: [], levelNumbering: '1', defaultLevels: '' },
  routers: [
    {
      uid: 'aaa',
      prefix: 'A',
      title: 'Main',
      numberOffset: 1,
      levels: [1, 2].map((u) => level(u)),
      levelNumbering: '1',
      srcs: [[0, 'CAM 1']],
      dsts: [[2, 'MON 3']],
      tally: [],
      status: { router: 'up' },
    },
    {
      uid: 'bbb',
      prefix: 'B',
      title: 'Stage',
      numberOffset: 0,
      levels: [1, 2, 3, 4].map((u) => level(u, 0)),
      levelNumbering: '0',
      srcs: [
        [4, 'B CAM'],
        [5, 'B CAM 2'],
      ],
      dsts: [[2, 'B MON']],
      tally: [],
      status: { router: 'up' },
      protected: [[2, 3, 9, 2]],
      protectBy: [],
      protectDevice: 3,
    },
  ],
};

connect();
sock.onopen();
sock.onmessage({ data: JSON.stringify(hello) });

const bEntry = (o = {}) => ({
  id: 7,
  ok: true,
  router: 'bbb',
  src: 5,
  srcName: 'B CAM 2',
  dst: 2,
  dstName: 'B MON',
  levels: [1, 2, 3, 4],
  prev: [1, 2, 3, 4].map((u) => [u, 4]),
  ...o,
});

test('a partly undone entry on router B names its levels in its numbering', () => {
  const e = bEntry({ undone: { state: 'partly', by: [9], levels: [1, 2] } });
  assert.equal(undoneNote(e.undone, e.router), 'levels 0–1 undone by #9');
});

test('the phone External follow line names the destination on the router of the entry', () => {
  const text = followNames(bEntry({ follows: [{ dst: 2, dstName: 'B MON' }] }));
  assert.equal(text, `External follow: ${dstItem(2, 'B MON', 'bbb').label}`);
  assert.match(text, /^External follow: B\./);
});

test('a failure banner names the levels of the entry in its router numbering', () => {
  const e = bEntry({ ok: false, failed: [3, 4], now: [] });
  const b = failureBanner({ verb: 'take', items: [{ entry: 7, dst: dstItem(2, '', 'bbb') }] }, [e]);
  assert.match(b.key, /levels 2–3 no answer/);
});

test('the Undo confirm notes name levels of the destination router', () => {
  const it = undoPlan(bEntry({ ok: false, failed: [3], now: [[3, 4]] })).items[0];
  assert.deepEqual(it.untouched, [3]);
  assert.deepEqual(undoNotes(it), ['Level 2: the take did not change it, left as is.']);
  assert.deepEqual(undoNotes({ dst: dstItem(2, '', 'bbb'), untouched: [], skipped: [1, 2] }), ['Levels 0–1: nothing was routed before, left as is.']);
});

test('the Last levels chip uses the levels of the destinations router', () => {
  assert.ok(noteLast(patchPlan(srcItem(5, '', 'bbb'), [dstItem(2, '', 'bbb')], [2, 4])));
  const t = lastTake();
  assert.deepEqual(lastChip(t, { src: null, dsts: [], levels: [] }).lvl, ['1, 3']);
});

test('a failed Protect on router B names the levels still held in its numbering', () => {
  sendProtect(protectPlan(dstItem(2, '', 'bbb'), true));
  const id = sent.at(-1).id;
  sock.onmessage({ data: JSON.stringify({ t: 'protectSetResult', id, ok: false, results: [{ dst: 2, ok: false, err: 'no answer', still: [1, 2] }] }) });
  assert.match(S.result.text, /^B\.2 B MON: no answer\. Still Protected on levels 0–1\.$/);
});

test('Settings > Protects lists the holds of every router', () => {
  const holds = everyHold();
  assert.deepEqual(
    holds.map((h) => [h.uid, h.n, h.hold.levels]),
    [['bbb', 2, [3]]],
  );
});
