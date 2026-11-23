// Undo and Redo of tie takes (docs/tielines-design.md, Undo; Redo and Retry). The tests cover the verdicts in the
// confirm, the first path of a Redo in the picker and the step rules of Ctrl+Z.
import test from 'node:test';
import assert from 'node:assert/strict';
import { tieUndoIds, tieUndoPlan, putBackText, pickerPlan } from '../tieundorules.js';
import { lastState, usedLast, chosen } from '../tiepickrules.js';

const tab = new Map();
globalThis.sessionStorage = { getItem: (k) => tab.get(k) ?? null, setItem: (k, v) => tab.set(k, String(v)), removeItem: (k) => tab.delete(k) };
const { backStep, entryKey } = await import('../stepback.js');

// The destination entry B.42 of a tie take from A.8 through TIE 3, and the hop entry that changed TIE 3.
const hop = { id: 1, time: 'T1', router: 'rA', src: 7, dst: 20, levels: [1], prev: [[1, 2]], ok: true, tie: 't3', group: 'g1', groupN: 2 };
const dest = {
  id: 2,
  time: 'T1',
  router: 'rB',
  src: 30,
  dst: 41,
  levels: [1],
  prev: [[1, 30]],
  ok: true,
  origin: { router: 'rA', n: 7, name: 'CAM 8' },
  hops: [{ router: 'rA', src: 7, dst: 20, levels: [1], tie: 't3', tieName: 'TIE 3', entry: 1, result: 'connected', prev: [[1, 2]] }],
  group: 'g1',
  groupN: 2,
};
const item = (e, o = {}) => ({ dst: { n: e.dst }, groups: [{ src: { n: e.prev[0][1] }, levels: [1] }], undo: e, ...o });
const undoPlan = (...items) => ({ verb: 'undo', items });
const label = (e) => e.name || `${e.router}.${e.n}`;
const put = { entry: 1, tie: 't3', tieName: 'TIE 3', to: { router: 'rA', n: 2, name: 'CAM 3' }, affected: [{ router: 'rB', n: 41, name: 'B MON' }] };

test('tieUndoIds: only the destination entries of tie takes that worked', () => {
  const plain = { ...dest, id: 9, origin: undefined };
  assert.deepEqual(tieUndoIds(undoPlan(item(dest), item(plain), item(hop))), [2]);
  assert.deepEqual(tieUndoIds(undoPlan(item({ ...dest, ok: false }))), [], 'a failed tie take has Put back on its row');
  assert.deepEqual(tieUndoIds(undoPlan(item(dest, { putBack: true }))), []);
  assert.deepEqual(tieUndoIds({ ...undoPlan(item(dest)), tieChecked: true }), [], 'a checked plan is not asked again');
  assert.deepEqual(tieUndoIds({ verb: 'redo', items: [item(dest)] }), []);
});

test('tieUndoPlan: same is a plain Undo; putBack offers the tie line in a box that is off', () => {
  const plan = undoPlan(item(dest));
  assert.deepEqual(
    tieUndoPlan(plan, [{ entry: 2, verdict: 'same' }], () => null, label),
    { plan },
  );
  const putItem = (id) => ({ dst: { n: 20 }, groups: [], undo: { id }, putBack: true });
  const out = tieUndoPlan(plan, [{ entry: 2, verdict: 'putBack', putBack: [put] }], putItem, label);
  assert.equal(out.plan.extra.on, false);
  assert.deepEqual(
    out.plan.extra.items.map((i) => [i.undo.id, i.putBack]),
    [[1, true]],
  );
  assert.equal(out.plan.extra.label, 'Also put TIE 3 back to CAM 3. Affected destinations: B MON');
  // The Undo of the destination stays as it was.
  assert.equal(out.plan.items, plan.items);
  // A hop entry that is not loaded cannot be offered: Undo sends nothing.
  assert.ok(tieUndoPlan(plan, [{ entry: 2, verdict: 'putBack', putBack: [put] }], () => null, label).changed);
});

test('tieUndoPlan: changed sends nothing, and the picker goes from the source from before', () => {
  const check = { entry: 2, verdict: 'changed', text: 'TIE 3 now carries CAM 9, not CAM 3', want: { router: 'rA', n: 2, name: 'CAM 3' }, levels: [1] };
  const out = tieUndoPlan(undoPlan(item(dest)), [{ entry: 5, verdict: 'same' }, check], () => null, label);
  assert.equal(out.changed, check);
  const p = pickerPlan(dest, check, { src: (w) => ({ n: w.n, router: w.router }), dst: (e) => ({ n: e.dst, router: e.router }) });
  assert.deepEqual(
    [p.verb, p.items[0].dst, p.items[0].groups[0].src, p.items[0].groups[0].levels, p.items[0].undo],
    ['undo', { n: 41, router: 'rB' }, { n: 2, router: 'rA' }, [1], null],
  );
});

test('putBackText: many tie lines and many Affected destinations', () => {
  const many = Array.from({ length: 6 }, (_, i) => ({ router: 'rB', n: i }));
  const text = putBackText(
    [
      { ...put, affected: many },
      { entry: 3, tie: 't9', to: null, affected: [] },
    ],
    label,
  );
  assert.equal(text, 'Also put TIE 3 back to CAM 3, t9 back. Affected destinations: rB.0, rB.1, rB.2, rB.3, and 2 more');
});

// Paths of one route, cheapest first, as the 409 body gives them.
const path = (tie, cost, o = {}) => ({ hops: [{ tie: { id: tie }, sends: true }, { tie: null }], cost, affected: [], ...o });

test('lastState: Redo keeps the old tie lines while they are usable, also when they cost more', () => {
  const route = { paths: [path('t1', 2), path('t3', 2), path('t9', 5)] };
  let st = lastState(route, ['t3']);
  assert.equal(chosen(st).hops[0].tie.id, 't3');
  assert.equal(usedLast(st, 0, 't3'), true);
  assert.equal(usedLast(st, 0, 't1'), false);
  st = lastState(route, ['t9']);
  assert.equal(chosen(st).hops[0].tie.id, 't9', 'it costs more than the cheapest, and stays chosen');
  assert.equal(usedLast(st, 0, 't9'), true);
  assert.equal(st.lastWhy, '');
  st = lastState({ paths: [path('t1', 2), path('t3', 1, { protected: true })] }, ['t3']);
  assert.equal(chosen(st).hops[0].tie.id, 't1', 'a Protected path is never chosen');
  assert.match(st.lastWhy, /^t3, used last time, is not usable now\./);
  st = lastState(route, ['gone']);
  assert.equal(chosen(st).hops[0].tie.id, 't1');
  assert.match(st.lastWhy, /^gone, used last time, makes no path now\./);
  // Two steps: the first old tie line stays, and the planner chooses the second one.
  const two = (a, b, cost, o = {}) => ({ hops: [{ tie: { id: a } }, { tie: { id: b, name: b.toUpperCase() }, ...o }, { tie: null }], cost, affected: [] });
  st = lastState({ paths: [two('t1', 'u1', 1), two('t3', 'u2', 3), two('t3', 'u9', 4, { sends: true, protect: 'Ana' })] }, ['t3', 'u9']);
  assert.deepEqual(
    chosen(st)
      .hops.slice(0, 2)
      .map((h) => h.tie.id),
    ['t3', 'u2'],
  );
  assert.match(st.lastWhy, /^U9, used last time, is not usable now\./);
});

test('Ctrl+Z: the step of a tie take is its destination entry, never a hop entry', () => {
  const hist = [dest, hop];
  const own = [dest, hop].map(entryKey);
  assert.deepEqual(
    backStep(hist, own).entries.map((x) => x.id),
    [2],
  );
  // A hop on its own is no step, even when this tab has its key.
  assert.equal(backStep([hop], own), null);
  // Its undo marks the destination Undone: the next press goes further back.
  const undo = { id: 3, time: 'T3', router: 'rB', src: 30, dst: 41, levels: [1], prev: [[1, 30]], ok: true, undo: 2 };
  assert.equal(backStep([undo, ...hist], [...own, entryKey(undo)]), null);
});
