import test from 'node:test';
import assert from 'node:assert/strict';
import { routerLinks, routerLink, barMark, allUp } from '../routerlinkrules.js';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document ??= node();
globalThis.localStorage ??= { getItem: () => null, setItem() {}, removeItem() {} };
globalThis.WebSocket ??= { OPEN: 1 };
const { S, connState, routerData } = await import('../core.js');
// of for routerLinks: one router's fields, as routerData gives them.
const one = (d) => () => ({ uid: '', prefix: '', title: '', addr: '', link: null, err: '', ...d });

const up = { wsState: 'up' };
const many = (states) =>
  states.map((link, i) => routerLink(up, `u${i}`, String.fromCharCode(65 + i), '', `10.0.0.${i}:8910`, link, link === 'down' ? 'refused' : ''));

test('the state follows connState for every server and router state', () => {
  for (const wsState of ['up', 'connecting', 'down']) {
    for (const router of ['up', 'connecting', 'down', 'odd', '']) {
      Object.assign(S, { wsLost: false, wsState });
      Object.assign(routerData(''), { link: router, addr: '1.2.3.4:8910', err: '' });
      assert.equal(routerLinks(S, routerData)[0].state, connState().rt[0], `${wsState} ${router}`);
      assert.equal(`Router ${routerLinks(S, routerData)[0].word}`, connState().rt[1], `${wsState} ${router}`);
    }
  }
});

test('one router: one entry with its fields, and one icon with no prefix', () => {
  const s = { wsState: 'up', routers: [{ uid: 'abc', prefix: 'A', title: 'Studio' }] };
  const d = { uid: 'abc', prefix: 'A', title: 'Studio', addr: '1.2.3.4:8910', link: 'down', err: 'refused' };
  const list = routerLinks(s, one(d));
  assert.deepEqual(list, [
    { uid: 'abc', prefix: 'A', title: 'Studio', addr: '1.2.3.4:8910', link: 'down', state: 'error', word: 'down', err: 'refused', never: false },
  ]);
  assert.deepEqual(barMark(list), { state: 'error', word: 'Router', aria: 'Router down', down: 1 });
  assert.equal(allUp(list), false);
  assert.equal(allUp(routerLinks(s, one({ ...d, link: 'up' }))), true);
  assert.equal(routerLinks(s, one({ ...d, link: 'up' }))[0].err, '', 'an up link shows no error');
});

test('no server link: every router is unknown', () => {
  const [r] = routerLinks({ wsState: 'down' }, one({ link: 'up', addr: 'x' }));
  assert.deepEqual([r.state, r.word, r.link], ['idle', 'unknown', '']);
});

test('allUp: every router up', () => {
  assert.equal(allUp(many(['up', 'connecting', 'down'])), false);
  assert.equal(allUp(many(['up', 'up'])), true);
});
