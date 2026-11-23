import test from 'node:test';
import assert from 'node:assert/strict';
import { routerLinks, routerLink, routerMarks, allUp, linkLabel, ROUTER_ICONS_MAX } from '../routerlinkrules.js';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document ??= node();
globalThis.localStorage ??= { getItem: () => null, setItem() {}, removeItem() {} };
globalThis.WebSocket ??= { OPEN: 1 };
const { S, connState } = await import('../core.js');

const up = { wsState: 'up' };
const many = (states) =>
  states.map((link, i) => routerLink(up, `u${i}`, String.fromCharCode(65 + i), '', `10.0.0.${i}:8910`, link, link === 'down' ? 'refused' : ''));

test('the state follows connState for every server and router state', () => {
  for (const wsState of ['up', 'connecting', 'down']) {
    for (const router of ['up', 'connecting', 'down', 'odd', '']) {
      Object.assign(S, { wsLost: false, wsState, router, routerAddr: '1.2.3.4:8910', routerLastErr: '' });
      assert.equal(routerLinks(S)[0].state, connState().rt[0], `${wsState} ${router}`);
      assert.equal(`Router ${routerLinks(S)[0].word}`, connState().rt[1], `${wsState} ${router}`);
    }
  }
});

test('one router: one entry with its fields, and one mark with no prefix', () => {
  const s = {
    wsState: 'up',
    router: 'down',
    routerUid: 'abc',
    prefix: 'A',
    routerAddr: '1.2.3.4:8910',
    routerLastErr: 'refused',
    cfg: { routerName: 'Studio' },
  };
  const list = routerLinks(s);
  assert.deepEqual(list, [{ uid: 'abc', prefix: 'A', title: 'Studio', addr: '1.2.3.4:8910', link: 'down', state: 'error', word: 'down', err: 'refused' }]);
  assert.deepEqual(routerMarks(list), [{ state: 'error', label: 'Router', aria: 'Router down', count: '' }]);
  assert.equal(linkLabel(list[0], 1), 'Router link');
  assert.equal(allUp(list), false);
  assert.equal(allUp(routerLinks({ ...s, router: 'up' })), true);
  assert.equal(routerLinks({ ...s, router: 'up' })[0].err, '', 'an up link shows no error');
});

test('no server link: every router is unknown', () => {
  const [r] = routerLinks({ wsState: 'down', router: 'up', routerAddr: 'x' });
  assert.deepEqual([r.state, r.word, r.link], ['idle', 'unknown', '']);
});

test('up to the cap: one mark per router, named by prefix', () => {
  const list = many(['up', 'connecting', 'down'].slice(0, ROUTER_ICONS_MAX));
  const marks = routerMarks(list);
  assert.equal(marks.length, list.length);
  assert.deepEqual(
    marks.map((m) => [m.state, m.label, m.aria]),
    [
      ['ok', 'Router A', 'Router A connected'],
      ['warn', 'Router B', 'Router B connecting'],
      ['error', 'Router C', 'Router C down'],
    ],
  );
  assert.equal(linkLabel({ prefix: 'B', title: 'Truck' }, 2), 'Router link B Truck');
  assert.equal(allUp(list), false);
  assert.equal(allUp(many(['up', 'up'])), true);
});

test('over the cap: one mark with a count, in the worst state', () => {
  assert.deepEqual(routerMarks(many(['up', 'up', 'up', 'connecting', 'up'])), [
    { state: 'warn', label: 'Routers', aria: '4 of 5 routers connected', count: '4/5' },
  ]);
  assert.equal(routerMarks(many(['up', 'down', 'connecting', 'up']))[0].state, 'error');
  assert.equal(routerMarks(many(['up', 'up', 'up', 'up']))[0].state, 'ok');
  assert.equal(routerMarks([...many(['up', 'up', 'up']), routerLink({ wsState: 'down' }, 'x', 'D', '', '', 'up', '')])[0].state, 'idle');
  assert.equal(routerMarks(many(['up', 'up']), 1).length, 1, 'the cap is a parameter');
});
