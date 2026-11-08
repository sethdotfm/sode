// The signed-in user's own rights grey out what the server would refuse. The server stays the authority.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const setup = async () => {
  const mod = await import('./all.mjs');
  const { S } = mod;
  S.cfg = { numberOffset: 1, filterSets: [] };
  S.ws = {};
  S.wsState = 'up';
  S.router = 'up';
  S.levelBy = new Map([
    [1, {}],
    [2, {}],
  ]);
  return mod;
};
const item = (n) => ({ n, num: n + 1, name: `X${n}`, label: `X${n}` });

test('a plan outside the signed-in rights is blocked before confirm, with the same words the server uses', async () => {
  const { S, patchPlan, planBlocked, rightsOff, applyMyRights } = await setup();
  const plan = patchPlan(item(3), [item(9)], [1]);
  assert.equal(planBlocked(plan).block, null, 'no rights known: nothing greyed');
  applyMyRights({ dsts: [8] });
  const { block } = planBlocked(plan);
  assert.equal(block.kind, 'error');
  assert.equal(block.title, 'Outside your rights');
  assert.equal(block.text, 'destination 10 is outside your rights');
  assert.equal(rightsOff('dsts', 9), true);
  assert.equal(rightsOff('dsts', 8), false);
  applyMyRights(null);
  assert.equal(planBlocked(plan).block, null, 'rights are forgotten on sign out');
  assert.equal(S.myRights, null);
});

test('rightsOff greys a source or destination row only when its list excludes it', async () => {
  const { rightsOff, applyMyRights } = await setup();
  applyMyRights({ sources: [1, 2] });
  assert.equal(rightsOff('sources', 1), false);
  assert.equal(rightsOff('sources', 5), true);
  assert.equal(rightsOff('dsts', 5), false);
  applyMyRights({ sources: [] });
  assert.equal(rightsOff('sources', 1), true);
});
