// Stale-WebSocket rules (#142). Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

test('wsStale: only after the stale limit', async () => {
  const { wsStale, WS_STALE_MS } = await import('./all.mjs');
  assert.equal(wsStale(1000, 1000 + WS_STALE_MS), false);
  assert.equal(wsStale(1000, 1001 + WS_STALE_MS), true);
  assert.equal(wsStale(1000, 1500), false);
});

test('wsFresh: open and heard from recently', async () => {
  const { wsFresh, WS_FRESH_MS } = await import('./all.mjs');
  assert.equal(wsFresh(true, 1000, 1000 + WS_FRESH_MS), true);
  assert.equal(wsFresh(true, 1000, 1001 + WS_FRESH_MS), false);
  assert.equal(wsFresh(false, 1000, 1001), false);
});

test('fresh window is shorter than the stale limit', async () => {
  const { WS_FRESH_MS, WS_STALE_MS } = await import('./all.mjs');
  assert.ok(WS_FRESH_MS < WS_STALE_MS);
});
