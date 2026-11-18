// A fetcher that sends If-None-Match and hands back the last body on a 304.
import test from 'node:test';
import assert from 'node:assert/strict';
import { withEtag } from '../etagfetch.js';

// A server that answers from a body and its tag, as /api/v1/tally does.
function server(state) {
  const seen = [];
  const fetcher = async (path, init = {}) => {
    seen.push({ path, inm: init.headers?.['If-None-Match'], method: init.method });
    if (init.headers?.['If-None-Match'] === state.tag) return new Response(null, { status: 304, headers: { ETag: state.tag } });
    return new Response(JSON.stringify(state.body), { status: 200, headers: { ETag: state.tag } });
  };
  return { fetcher, seen };
}

test('a 304 gives the last body as a 200', async () => {
  const { fetcher, seen } = server({ tag: '"a"', body: { n: 1 } });
  const call = withEtag(fetcher);
  assert.deepEqual(await (await call('api/v1/tally')).json(), { n: 1 });
  const again = await call('api/v1/tally');
  assert.equal(again.status, 200);
  assert.deepEqual(await again.json(), { n: 1 });
  assert.deepEqual(
    seen.map((x) => x.inm),
    [undefined, '"a"'],
  );
});

test('a new tag replaces the kept body', async () => {
  const s = { tag: '"a"', body: { n: 1 } };
  const { fetcher } = server(s);
  const call = withEtag(fetcher);
  await call('p');
  Object.assign(s, { tag: '"b"', body: { n: 2 } });
  assert.deepEqual(await (await call('p')).json(), { n: 2 });
  assert.deepEqual(await (await call('p')).json(), { n: 2 });
});

test('paths keep their own tags, and a failure drops the kept copy', async () => {
  const { fetcher, seen } = server({ tag: '"a"', body: { n: 1 } });
  const call = withEtag(fetcher);
  await call('x');
  await call('y');
  assert.equal(seen[1].inm, undefined);
  const bad = withEtag(async (_p, init) =>
    init?.headers?.['If-None-Match'] ? new Response('{}', { status: 401 }) : new Response('{"n":1}', { headers: { ETag: '"a"' } }),
  );
  await bad('x');
  assert.equal((await bad('x')).status, 401);
  assert.equal((await bad('x')).status, 200);
});

test('only a plain GET is tagged', async () => {
  const { fetcher, seen } = server({ tag: '"a"', body: {} });
  const call = withEtag(fetcher);
  await call('x', { method: 'POST' });
  await call('x', { method: 'POST' });
  assert.deepEqual(
    seen.map((x) => x.inm),
    [undefined, undefined],
  );
});

test('a caller that sets If-None-Match itself keeps the raw 304', async () => {
  const { fetcher } = server({ tag: '"a"', body: {} });
  const call = withEtag(fetcher);
  assert.equal((await call('x', { headers: { 'If-None-Match': '"a"' } })).status, 304);
});

test('a shared path gives the same parsed object until the tag moves', async () => {
  const s = { tag: '"a"', body: { routes: [1] } };
  const { fetcher } = server(s);
  const call = withEtag(fetcher, (p) => p === 'api/v1/tally');
  const first = await (await call('api/v1/tally')).json();
  assert.equal(await (await call('api/v1/tally')).json(), first);
  Object.assign(s, { tag: '"b"', body: { routes: [2] } });
  const moved = await (await call('api/v1/tally')).json();
  assert.notEqual(moved, first);
  assert.deepEqual(moved, { routes: [2] });
  await call('x');
  const x1 = await (await call('x')).json();
  const x2 = await (await call('x')).json();
  assert.notEqual(x1, x2);
  assert.deepEqual(x1, x2);
});
