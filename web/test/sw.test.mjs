// The service worker's rules (sw.js), run in a vm as a classic script: what it keeps, and when the kept copy answers.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const src = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'sw.js'), 'utf8');

// A cache that keeps Responses by key; fetch is the test's.
function worker(fetch) {
  const store = new Map();
  const cache = {
    match: async (key) => store.get(key)?.clone(),
    put: async (key, res) => void store.set(key, res),
  };
  const caches = { open: async () => cache };
  const self = { addEventListener() {}, registration: { scope: 'http://sode.test/' } };
  const ctx = vm.createContext({ self, caches, fetch, URL, Request, Response });
  vm.runInContext(src, ctx);
  return { ctx, store };
}

test('pages and app shell files are kept; the API, the socket, healthz and the worker are not', () => {
  const { ctx } = worker();
  const kind = (path, method = 'GET') => ctx.kindOf(`http://sode.test/${path}`, method, 'http://sode.test/');
  for (const p of ['', 'panels', 'panels/', 'panels/editor', 'panels/abc123', 'panels/hall-grid']) assert.equal(kind(p), 'page', p);
  for (const f of ['main.js', 'base.css', 'panels-logo.svg', 'icon.png', 'vendor/patchbay/css/patchbay.css', 'vendor/fonts/IBMPlexSans-Regular.woff2']) {
    assert.equal(kind(f), 'file', f);
  }
  for (const n of [
    'api/v1/tally',
    'api/v1/panels/abc',
    'api/health',
    'api/export',
    'ws',
    'healthz',
    'sw.js',
    'manifest.webmanifest',
    'log',
    'panels/abc/key',
  ]) {
    assert.equal(kind(n), '', n);
  }
  assert.equal(kind('main.js', 'POST'), '');
  assert.equal(kind(''), 'page');
  assert.equal(ctx.kindOf('http://other.test/main.js', 'GET', 'http://sode.test/'), '');
  // Under a proxy's path prefix, the scope is the prefix.
  assert.equal(ctx.kindOf('http://sode.test/sode/panels/x', 'GET', 'http://sode.test/sode/'), 'page');
  assert.equal(ctx.kindOf('http://sode.test/sode/api/v1/tally', 'GET', 'http://sode.test/sode/'), '');
});

test('a page is kept by its path, whatever its query', () => {
  const { ctx } = worker();
  assert.equal(ctx.cacheKey('http://sode.test/panels/abc?instance=Hall', 'page'), 'http://sode.test/panels/abc');
  assert.equal(ctx.cacheKey('http://sode.test/icon.png?s=192', 'file'), 'http://sode.test/icon.png?s=192');
});

const page = (body, etag) => new Response(body, { headers: etag ? { ETag: etag } : {} });
// A real fetch gives type 'basic' for the same origin; a Response made here is 'default'.
const basic = (res) => Object.defineProperty(res, 'type', { value: 'basic' });

test('Sode answers first and its answer is kept; with Sode down the kept copy answers', async () => {
  let up = true;
  const { ctx, store } = worker(async () => {
    if (!up) throw new TypeError('Failed to fetch');
    return basic(page('v1', '"a"'));
  });
  const key = 'http://sode.test/panels/abc';
  const writes = [];
  assert.equal(await (await ctx.networkFirst({}, key, (p) => writes.push(p))).text(), 'v1');
  await Promise.all(writes);
  assert.equal(await store.get(key).clone().text(), 'v1');
  up = false;
  assert.equal(await (await ctx.networkFirst({}, key)).text(), 'v1');
  await assert.rejects(ctx.networkFirst({}, 'http://sode.test/panels/never'), TypeError);
});

test("a proxy's 5xx while Sode is down gives the kept copy", async () => {
  let status = 200;
  const { ctx } = worker(async () => basic(new Response(status === 200 ? 'v1' : 'Bad Gateway', { status, headers: { ETag: '"a"' } })));
  const key = 'http://sode.test/';
  const writes = [];
  await ctx.networkFirst({}, key, (p) => writes.push(p));
  await Promise.all(writes);
  status = 502;
  const res = await ctx.networkFirst({}, key);
  assert.equal(res.status, 200);
  assert.equal(await res.text(), 'v1');
});

test('an answer with the kept ETag writes nothing; a new ETag replaces the copy; an error is never kept', async () => {
  let next = () => basic(page('v1', '"a"'));
  const { ctx, store } = worker(async () => next());
  const key = 'http://sode.test/main.js';
  const run = async () => {
    const writes = [];
    await ctx.networkFirst({}, key, (p) => writes.push(p));
    await Promise.all(writes);
  };
  await run();
  const kept = store.get(key);
  await run();
  assert.equal(store.get(key), kept, 'same ETag, same copy');
  next = () => basic(page('v2', '"b"'));
  await run();
  assert.equal(await store.get(key).clone().text(), 'v2');
  next = () => basic(new Response('gone', { status: 404 }));
  await run();
  assert.equal(await store.get(key).clone().text(), 'v2');
});
