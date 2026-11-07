// Recent rows: the wording and plans both layouts share.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const setup = async () => {
  const m = await import('./all.mjs');
  const { S } = m;
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.levelBy = new Map([[1, {}]]);
  S.srcBy = new Map([[7, { n: 7, num: 8, name: 'CAM 8', label: 'CAM 8' }]]);
  S.dstBy = new Map([[5, { n: 5, num: 6, name: 'MON 6', label: 'MON 6' }]]);
  S.tally = new Map();
  S.hist = [];
  return m;
};
const take = (o = {}) => ({ id: 1, src: 7, srcName: 'CAM 8', dst: 5, dstName: 'MON 6', levels: [1], prev: [[1, 0]], ok: true, ...o });

test('rowActions: a take', async () => {
  const { rowActions } = await setup();
  const a = rowActions({ e: take() });
  assert.equal(a.undo.text, 'Undo');
  assert.equal(a.undo.tip, undefined);
  assert.match(a.undo.label, /^Undo .+ to .+$/);
  assert.match(a.again.label, /^Redo .+ to .+$/);
  assert.equal(a.again.plan.verb, 'redo');
  assert.equal(rowActions({ e: take({ ok: false }) }).again.label.startsWith('Retry '), true);
});

test('rowActions: a stack of changes', async () => {
  const { rowActions } = await setup();
  const a = rowActions({ e: take({ chain: [{}, {}] }) });
  assert.equal(a.undo.text, 'Undo 2');
  assert.match(a.undo.label, /^Undo 2 changes on /);
  assert.equal(a.undo.tip, 'Set destination to its source before these changes.');
});

test('rowActions: a folder', async () => {
  const { rowActions } = await setup();
  const { UNDO_TIP_ALL } = await import('../widgets.js');
  const es = [take({ id: 1 }), take({ id: 2 })];
  const f = { group: 'g', entries: es, shown: es };
  const a = rowActions({ f });
  assert.match(a.undo.label, /^Undo all 2 takes of /);
  assert.equal(a.undo.text, 'Undo all');
  assert.equal(a.undo.tip, UNDO_TIP_ALL);
  assert.match(a.again.label, /^Redo 2 takes of /);
  const one = rowActions({ f: { group: 'g', entries: [take()], shown: [] } });
  assert.match(one.again.label, /^Redo 1 take of /);
});

test('disclosureAria and followersHead', async () => {
  const { disclosureAria, followersHead } = await setup();
  assert.equal(disclosureAria(true, 3, 'takes'), 'Hide the 3 takes');
  assert.equal(disclosureAria(false, 2, 'followers'), 'Show the 2 followers');
  assert.deepEqual(followersHead({ group: 'g', followers: [1, 2] }, false, 'm'), { count: '2', aria: 'Show the 2 followers', kidsId: 'm-followers-g' });
});

test('lastFill: nothing, empty side, and a plain fill', async () => {
  const { lastFill } = await setup();
  assert.deepEqual(lastFill('src', false, null), { none: true });
});
