// A socket that has gone quiet is not sent into (#142). Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
globalThis.WebSocket = { OPEN: 1 };

// A socket stub that has been quiet for `ago` ms.
function stub(readyState, ago) {
  const calls = { sent: [], closed: 0, lost: 0 };
  return {
    calls,
    readyState,
    lastRx: Date.now() - ago,
    send: (m) => calls.sent.push(m),
    close: () => { calls.closed++; },
    onclose: () => { calls.lost++; },
    onmessage: () => {},
  };
}

test('wsSend sends on an open socket heard from recently', async () => {
  const { wsSend, S } = await import('./all.mjs');
  S.ws = stub(1, 1000);
  assert.equal(wsSend({ t: 'x' }), true);
  assert.equal(S.ws.calls.sent.length, 1);
});

test('wsSend refuses a quiet socket, drops it and reports the loss once', async () => {
  const { wsSend, S } = await import('./all.mjs');
  const ws = (S.ws = stub(1, 8000));
  assert.equal(wsSend({ t: 'x' }), false);
  assert.deepEqual([ws.calls.sent.length, ws.calls.closed, ws.calls.lost], [0, 1, 1]);
  assert.equal(ws.onclose, null);
});

test('wsSend refuses a socket that is not open without touching it', async () => {
  const { wsSend, S } = await import('./all.mjs');
  const ws = (S.ws = stub(0, 0));
  assert.equal(wsSend({ t: 'x' }), false);
  assert.deepEqual([ws.calls.closed, ws.calls.lost], [0, 0]);
  S.ws = null;
  assert.equal(wsSend({ t: 'x' }), false);
});
