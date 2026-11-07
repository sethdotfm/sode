// scopeWriter: which saver and payload key a scope goes to, and what removeIn leaves.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const { scopeWriter } = await import('../globalsave.js');

const make = (target) => {
  const calls = [];
  const saver = (who) => ({ change: (payload, done) => (calls.push([who, payload]), done()) });
  const dropped = [];
  const lists = { global: [{ name: 'A' }, { name: 'b' }], user: [{ name: 'A' }] };
  const w = scopeWriter({
    global: saver('global'),
    user: saver('user'),
    target: target && (() => (target.on ? saver('target') : null)),
    key: 'items',
    globalKey: 'sets',
    listOf: (scope) => lists[scope],
    same: (a, b) => a.toLowerCase() === b.toLowerCase(),
    dropLocal: (n) => dropped.push(n),
  });
  return { w, calls, dropped };
};

test('saveIn picks saver and key by scope', () => {
  const { w, calls } = make();
  let n = 0;
  w.saveIn('global', [1], () => n++);
  w.saveIn('user', [2], () => n++);
  assert.equal(n, 2);
  assert.deepEqual(calls, [
    ['global', { sets: [1] }],
    ['user', { items: [2] }],
  ]);
});

test('a target takes every scope', () => {
  const t = { on: true };
  const { w, calls } = make(t);
  w.saveIn('global', [1], () => {});
  t.on = false;
  w.saveIn('global', [2], () => {});
  assert.deepEqual(calls, [
    ['target', { items: [1] }],
    ['global', { sets: [2] }],
  ]);
});

test('removeIn drops locally at once, else sends the list without the name', () => {
  const { w, calls, dropped } = make();
  let n = 0;
  w.removeIn('local', 'x', () => n++);
  w.removeIn('global', 'a', () => n++);
  assert.equal(n, 2);
  assert.deepEqual(dropped, ['x']);
  assert.deepEqual(calls, [['global', { sets: [{ name: 'b' }] }]]);
});
