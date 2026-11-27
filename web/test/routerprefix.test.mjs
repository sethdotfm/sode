import test from 'node:test';
import assert from 'node:assert/strict';
import { searchText } from '../routerprefix.js';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

test('the main page search accepts A.23 for the router with prefix A, and refuses a prefix no router has', async () => {
  const { S, search, item, routerData } = await import('./all.mjs');
  S.cfg = {};
  Object.assign((await import('./all.mjs')).routerData(''), { off: 1 });
  routerData('').prefix = 'A';
  const list = [item(21, 'CAM 22'), item(22, 'VTR'), item(229, 'GFX')];
  assert.deepEqual(
    search(list, 'A.23').map((it) => it.num),
    ['23', '230'],
    'as typing 23 does',
  );
  assert.equal(search(list, 'a.23')[0].num, '23');
  assert.equal(search(list, ' A.230 ')[0].name, 'GFX');
  assert.deepEqual(search(list, 'B.23'), []);
  assert.equal(search(list, '23')[0].num, '23', 'no prefix still works');
  assert.equal(search(list, 'vtr')[0].num, '23');
});

// Two routers, A and B: the fields of one route are its source, destinations and level box.
const routers = ['A', 'B'];

test('with no prefix in the route, each router takes its own numbers', () => {
  assert.equal(searchText('23', 'A', [], routers), '23');
  assert.equal(searchText('23', 'B', [], routers), '23');
  assert.equal(searchText('A.23', 'A', [], routers), '23');
  assert.equal(searchText('A.23', 'B', [], routers), null);
  assert.equal(searchText('C.23', 'A', [], routers), null, 'C is no router');
  assert.equal(searchText('CAM 1', 'B', [], routers), 'CAM 1', 'a name is not a prefix');
});

test('once one field holds a prefix, the other fields of the route accept only that router', () => {
  const others = ['B.5', ''];
  assert.equal(searchText('23', 'B', others, routers), '23');
  assert.equal(searchText('B.23', 'B', others, routers), '23');
  assert.equal(searchText('23', 'A', others, routers), null);
  assert.equal(searchText('A.23', 'A', others, routers), null);
  assert.equal(searchText('A.23', 'B', others, routers), null);
  assert.equal(searchText('23', 'A', ['C.5'], routers), '23', 'a prefix no router has pins nothing');
});
