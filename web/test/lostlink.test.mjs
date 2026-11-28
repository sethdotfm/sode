import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
globalThis.WebSocket = { OPEN: 1 };

const { S, connState, routerData } = await import('../core.js');
// set: the Server link fields of S, and link, addr and err of the first router (routerData).
function set(o) {
  const { link, addr, err, ...ws } = o;
  Object.assign(S, ws);
  const first = routerData('');
  if ('link' in o) first.link = link;
  if ('addr' in o) first.addr = addr;
  if ('err' in o) first.err = err;
}

test('a lost browser connection is the Server link', () => {
  set({ wsLost: true, wsState: 'down', link: 'up' });
  const { title, links } = connState().down;
  assert.equal(title, 'Lost link: the Server link is down');
  assert.deepEqual(links, [false, null]);
});

test('a router that is down with the server up is the Router link', () => {
  set({ wsLost: false, wsState: 'up', link: 'down', addr: '10.0.0.1:8910' });
  const { title, links } = connState().down;
  assert.equal(title, 'Lost link: the Router link is down');
  assert.deepEqual(links, [true, false]);
});

test('a lost link names no next try', () => {
  set({ wsLost: true, wsState: 'down', link: 'up' });
  assert.equal(connState().down.text, undefined);
  set({ wsLost: false, wsState: 'up', link: 'down', addr: '10.0.0.1:8910' });
  assert.equal(connState().down.text, undefined);
});

test('no banner while both links are up', () => {
  set({ wsLost: false, wsState: 'up', link: 'up' });
  assert.equal(connState().down, null);
});

test('ok only while both links are up', () => {
  set({ wsLost: false, wsState: 'up', link: 'up' });
  assert.equal(connState().ok, true);
  for (const link of ['connecting', 'down', '', undefined]) {
    set({ link, err: '' });
    assert.equal(connState().ok, false, `router ${link}`);
  }
  set({ wsState: 'connecting', link: 'up' });
  assert.equal(connState().ok, false);
  set({ wsLost: true, wsState: 'down', link: 'up' });
  assert.equal(connState().ok, false);
});

// A router after the first takes its link from routerData, which is the first router's here (no hello listed it).
test('the banner names every router with the Server link down, and each reason on its own line when Sode is Down', () => {
  S.routers = [
    { uid: '', prefix: 'A', title: 'Router A' },
    { uid: 'bbb', prefix: 'B', title: 'Router B (studio 2)', address: '10.0.0.2:8910' },
  ];
  set({ wsLost: true, wsState: 'down', link: 'up' });
  assert.deepEqual(
    connState().down.routers.map((r) => r.prefix),
    ['', 'B'],
  );
  set({ wsLost: false, wsState: 'up', link: 'down', addr: '10.0.0.1:8910', err: 'refused' });
  const { title, err } = connState().down;
  assert.equal(title, 'Lost link: every Router link is down');
  assert.match(err, /^Router.*: refused\nRouter B \(studio 2\): refused$/);
  S.routers = [];
});
