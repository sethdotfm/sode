import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
globalThis.WebSocket = { OPEN: 1 };

const { S, connState } = await import('../core.js');

test('a lost browser connection is the Server link', () => {
  Object.assign(S, { wsLost: true, wsState: 'down', router: 'up' });
  const { title, links } = connState().down;
  assert.match(title, /^Lost link: Server link/);
  assert.deepEqual(links, [false, null]);
});

test('a router that is down with the server up is the Router link', () => {
  Object.assign(S, { wsLost: false, wsState: 'up', router: 'down', routerAddr: '10.0.0.1:8910' });
  const { title, links } = connState().down;
  assert.match(title, /^Lost link: Router link/);
  assert.deepEqual(links, [true, false]);
});

test('no banner while both links are up', () => {
  Object.assign(S, { wsLost: false, wsState: 'up', router: 'up' });
  assert.equal(connState().down, null);
});

test('ok only while both links are up', () => {
  Object.assign(S, { wsLost: false, wsState: 'up', router: 'up' });
  assert.equal(connState().ok, true);
  for (const router of ['connecting', 'down', '', undefined]) {
    Object.assign(S, { router, routerLastErr: '' });
    assert.equal(connState().ok, false, `router ${router}`);
  }
  Object.assign(S, { wsState: 'connecting', router: 'up' });
  assert.equal(connState().ok, false);
  Object.assign(S, { wsLost: true, wsState: 'down', router: 'up' });
  assert.equal(connState().ok, false);
});
