// Redo: plans built from takes that worked.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const setup = async () => {
  const m = await import('./all.mjs');
  const { S } = m;
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([
    [1, {}],
    [2, {}],
    [3, {}],
  ]);
  S.srcBy = new Map([[7, { n: 7, num: 8, name: 'CAM 8', label: 'CAM 8' }]]);
  S.dstBy = new Map([[5, { n: 5, num: 6, name: 'MON 6', label: 'MON 6' }]]);
  S.tally = new Map();
  return m;
};
const take = (o = {}) => ({
  id: 1,
  src: 7,
  srcName: 'CAM 8',
  dst: 5,
  dstName: 'MON 6',
  levels: [1, 2, 3],
  prev: [
    [1, 0],
    [2, 0],
    [3, 0],
  ],
  ok: true,
  ...o,
});

test('canRedo: needs a real source and an existing level', async () => {
  const { canRedo } = await setup();
  assert.equal(canRedo(take()), true);
  for (const o of [{ ok: false }, { ext: true }, { ext: true, found: true }, { ext: true, follow: 3 }, { undo: 2 }]) assert.equal(canRedo(take(o)), true);
  assert.equal(canRedo(take({ src: -1 })), false);
  assert.equal(canRedo(take({ levels: [9] })), false);
  assert.equal(canRedo(null), false);
});

test('againPlan: single entries', async () => {
  const { againPlan, planVerb } = await setup();
  const ok = againPlan({ e: take() });
  assert.equal(ok.verb, 'redo');
  assert.equal(planVerb(ok), 'redo');
  const it = ok.items[0];
  assert.deepEqual(
    it.groups.map((g) => [g.src.n, g.levels]),
    [[7, [1, 2, 3]]],
  );
  assert.equal(it.retry, null);
  assert.equal(it.undo, null);
  const bad = take({ ok: false });
  const r = againPlan({ e: bad });
  assert.equal(r.verb, 'retry');
  assert.equal(r.items[0].retry, bad);
  for (const o of [{ undo: 2 }, { ext: true }, { ext: true, follow: 1 }]) {
    const p = againPlan({ e: take(o) });
    assert.equal(p.verb, 'redo');
    assert.equal(p.items.length, 1);
    assert.equal(p.items[0].retry, null);
  }
  assert.equal(againPlan({ e: take({ src: -1 }) }).items.length, 0);
});

test('againPlan: folders', async () => {
  const { againPlan } = await setup();
  const allOk = { entries: [take({ id: 1 }), take({ id: 2, dst: 6 }), take({ id: 3, src: -1 })] };
  const p = againPlan({ f: allOk });
  assert.equal(p.verb, 'redo');
  assert.deepEqual(
    p.items.map((i) => i.dst.n),
    [5, 6],
  );
  const failing = { entries: [take({ id: 1 }), take({ id: 2, dst: 6, ok: false }), take({ id: 3, dst: 8, ok: false })] };
  const r = againPlan({ f: failing });
  assert.equal(r.verb, 'retry');
  assert.deepEqual(
    r.items.map((i) => i.retry.id),
    [2, 3],
  );
});

test('alreadyThere: any non-undo item whose dst has the source everywhere', async () => {
  const { S, againPlan, undoPlan, alreadyThere } = await setup();
  const item = (o) => againPlan({ e: take(o) }).items[0];
  assert.equal(alreadyThere(item()), false);
  S.tally = new Map([
    [
      5,
      new Map([
        [1, 7],
        [2, 7],
        [3, 7],
      ]),
    ],
  ]);
  assert.equal(alreadyThere(item()), true);
  assert.equal(alreadyThere(item({ ok: false })), true);
  assert.equal(alreadyThere(undoPlan(take()).items[0]), false);
  S.tally = new Map([
    [
      5,
      new Map([
        [1, 7],
        [2, 7],
        [3, 4],
      ]),
    ],
  ]);
  assert.equal(alreadyThere(item()), false);
});

test('VERBS drives wording', async () => {
  const { VERBS, planVerb, planSummary, againPlan, patchPlan, undoPlan } = await setup();
  assert.equal(planVerb(patchPlan(null, [], [])), 'take');
  assert.equal(planVerb(undoPlan(take())), 'undo');
  assert.deepEqual(
    Object.entries(VERBS)
      .filter(([, v]) => v.resets)
      .map(([k]) => k),
    ['patch'],
  );
  assert.equal(VERBS.redo.busy, 'Redoing…');
  assert.equal(VERBS.retry.busy, 'Retrying…');
  assert.ok(planSummary(againPlan({ e: take() })).length > 0);
  assert.ok(planSummary(againPlan({ e: take({ ok: false }) })).length > 0);
});
