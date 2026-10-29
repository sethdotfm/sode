import test from 'node:test';
import assert from 'node:assert/strict';
import { KINDS, KIND_IDS, EVENT_KINDS, normalizeKinds, allKinds, pressKind } from '../logkinds.js';

const [a, b, c] = KIND_IDS;

test('KIND_IDS are unique and match KINDS', () => {
  assert.equal(new Set(KIND_IDS).size, KIND_IDS.length);
  assert.deepEqual(KIND_IDS, KINDS.map((g) => g.id));
});

test('normalizeKinds: default, unknown ids and empty lists', () => {
  assert.deepEqual(normalizeKinds(undefined, [a]), [a]);
  assert.deepEqual(normalizeKinds('x', [a]), [a]);
  assert.deepEqual(normalizeKinds(['dump', 'zzz', a], [a]), [a, 'dump']);
  assert.deepEqual(normalizeKinds([], [a]), KIND_IDS);
  assert.deepEqual(normalizeKinds(['zzz'], [a]), KIND_IDS);
});

test('All is every kind selected', () => {
  assert.equal(allKinds(KIND_IDS), true);
  assert.equal(allKinds([a, b]), false);
});

test('pressing a kind with All on selects only it', () => {
  assert.deepEqual(pressKind(KIND_IDS, b), [b]);
});

test('kinds toggle, and the last one off goes back to All', () => {
  assert.deepEqual(pressKind([a], c), [a, c]);
  assert.deepEqual(pressKind([a, c], a), [c]);
  assert.deepEqual(pressKind([c], c), KIND_IDS);
  assert.equal(allKinds(pressKind(KIND_IDS.slice(0, -1), KIND_IDS.at(-1))), true);
});

test('a list saved when All was the six message groups is still All', () => {
  const old = ['connect', 'interrogate', 'keepalive', 'dump', 'names', 'other'];
  assert.deepEqual(normalizeKinds(old, ['connect']), KIND_IDS);
  // Choosing a subset that happens to be the old six plus an event is not All.
  assert.deepEqual(normalizeKinds(old.slice(0, 5), ['connect']), old.slice(0, 5));
  assert.deepEqual(normalizeKinds([...old, 'command'], ['connect']), [...old, 'command']);
});

test('every event kind belongs to a group', () => {
  const grouped = new Set(KINDS.flatMap((g) => g.kinds));
  for (const k of EVENT_KINDS) assert.ok(grouped.has(k), k);
});
