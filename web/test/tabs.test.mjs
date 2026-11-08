import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

test('stepTab moves between tabs with the arrow keys and wraps', async () => {
  const { stepTab } = await import('../tabs.js');
  const ids = ['users', 'clients', 'rights'];
  assert.equal(stepTab(ids, 'users', 'ArrowRight'), 'clients');
  assert.equal(stepTab(ids, 'rights', 'ArrowRight'), 'users');
  assert.equal(stepTab(ids, 'users', 'ArrowLeft'), 'rights');
  assert.equal(stepTab(ids, 'clients', 'Home'), 'users');
  assert.equal(stepTab(ids, 'clients', 'End'), 'rights');
  assert.equal(stepTab(ids, 'clients', 'a'), null);
});
