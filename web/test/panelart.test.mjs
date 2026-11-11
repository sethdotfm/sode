// Artwork loading (panelart.js): fetched with the key, shown from a blob URL, one fetch per artwork id.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createArt } from '../panelart.js';

test('artwork is fetched once per id from the panel route and returned as a blob URL', async () => {
  const paths = [];
  const call = async (p) => (paths.push(p), { ok: true, blob: async () => ({ size: 3 }) });
  const art = createArt(call, 'p-1', () => 'blob:x');
  assert.equal(await art.url('a-0123abcd'), 'blob:x');
  assert.equal(await art.url('a-0123abcd'), 'blob:x');
  assert.deepEqual(paths, ['api/v1/panels/p-1/artwork/a-0123abcd']);
});

test('a refused or failed fetch gives null so the cell shows no artwork', async () => {
  assert.equal(
    await createArt(
      async () => ({ ok: false }),
      'p-1',
      () => 'blob:x',
    ).url('a-0123abcd'),
    null,
  );
  assert.equal(
    await createArt(
      async () => {
        throw new Error('offline');
      },
      'p-1',
      () => 'blob:x',
    ).url('a-0123abcd'),
    null,
  );
});

// A blob URL holds its bytes until it is revoked or the page unloads, so every Test it live and every panel switch
// would otherwise keep one more copy of each image.
test('revoke lets go of every blob URL made, and a later url() fetches afresh', async () => {
  const made = [];
  const gone = [];
  let n = 0;
  const call = async () => ({ ok: true, blob: async () => ({ size: 3 }) });
  const art = createArt(
    call,
    'p-1',
    () => (made.push(`blob:${++n}`), `blob:${n}`),
    (u) => gone.push(u),
  );
  await art.url('a-1');
  await art.url('a-2');
  const pending = art.url('a-3');
  art.revoke();
  await pending;
  assert.deepEqual(gone.sort(), ['blob:1', 'blob:2', 'blob:3']);
  assert.equal(await art.url('a-1'), 'blob:4');
});

// A deleted library icon: its one blob URL goes and the next look fetches again (a 404 there draws nothing).
test('forget lets go of one id only, and a later look fetches it afresh', async () => {
  const gone = [];
  let n = 0;
  const paths = [];
  const call = async (p) => (paths.push(p), { ok: true, blob: async () => ({ size: 3 }) });
  const art = createArt(
    call,
    'p-1',
    () => `blob:${++n}`,
    (u) => gone.push(u),
  );
  await art.url('lib:0123abcd');
  await art.url('a-1');
  art.forget('lib:0123abcd');
  await Promise.resolve();
  assert.deepEqual(gone, ['blob:1']);
  assert.equal(await art.url('a-1'), 'blob:2');
  assert.equal(await art.url('lib:0123abcd'), 'blob:3');
  assert.equal(paths.length, 3);
  art.forget('a-never');
});

// An Icon library SVG is drawn as a mask in the text colour, so the page needs the file's type, which only the fetch knows.
test('look gives the blob URL with the fetched type, from the same one fetch', async () => {
  let n = 0;
  const call = async (p) => (n++, { ok: !p.endsWith('bad'), blob: async () => ({ type: 'image/svg+xml' }) });
  const art = createArt(call, 'p-1', () => 'blob:x');
  assert.deepEqual(await art.look('lib:0123abcd'), { url: 'blob:x', type: 'image/svg+xml' });
  assert.equal(await art.url('lib:0123abcd'), 'blob:x');
  assert.equal(n, 1);
  assert.equal(await art.look('a-bad'), null);
});
