// The service worker's reload rule (swreg.js): only a page that started while Sode was down reloads for a new build.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
globalThis.WebSocket = { OPEN: 1 };

const { emit } = await import('../core.js');
const { registerShell } = await import('../swreg.js');

// A page with a worker in control (it loaded through it). Returns what the rule did.
function page() {
  const did = { updates: 0, reloads: 0 };
  let change = null;
  globalThis.window = { isSecureContext: true };
  globalThis.document = { baseURI: 'http://sode.test/panels/abc' };
  globalThis.location = { reload: () => did.reloads++ };
  setNavigator({
    serviceWorker: {
      controller: {},
      register: async () => ({ update: async () => did.updates++ }),
      addEventListener: (ev, fn) => ev === 'controllerchange' && (change = fn),
    },
  });
  registerShell();
  return { did, change: () => change() };
}
// Node has its own read-only navigator.
const setNavigator = (value) => Object.defineProperty(globalThis, 'navigator', { value, configurable: true, writable: true });
const settle = () => new Promise((r) => setTimeout(r, 0));

test('a page that started with Sode down asks for the worker at its first hello and reloads for a new one', async () => {
  const p = page();
  emit('wsclose');
  emit('hello');
  await settle();
  assert.equal(p.did.updates, 1);
  p.change();
  assert.equal(p.did.reloads, 1);
});

test('a page that started online never reloads for a new worker', async () => {
  const p = page();
  emit('hello');
  emit('wsclose');
  await settle();
  assert.equal(p.did.updates, 0);
  p.change();
  assert.equal(p.did.reloads, 0);
});

test('no secure origin, no worker', () => {
  let asked = false;
  globalThis.window = { isSecureContext: false };
  setNavigator({ serviceWorker: { register: () => (asked = true) } });
  registerShell();
  assert.equal(asked, false);
});
