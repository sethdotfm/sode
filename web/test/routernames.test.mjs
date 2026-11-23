import test from 'node:test';
import assert from 'node:assert/strict';
import { readRouterNames } from '../routernames.js';

const reply = (paths) => async (path) => (path in paths ? paths[path] : null);

test('a refused read of the router list gives no names, not names on the first router', async () => {
  assert.equal(await readRouterNames(reply({ 'api/v1/names': { srcs: [], dsts: [] } })), null);
});

test('one router reads its names with no router parameter', async () => {
  const names = { srcs: [[0, 'CAM 1']], dsts: [] };
  const r = await readRouterNames(reply({ 'api/v1/routers': { routers: [{ uid: 'a1', prefix: 'A' }] }, 'api/v1/names': names }));
  assert.equal(r.places.many, false);
  assert.equal(r.first, names);
});
