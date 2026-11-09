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
  assert.equal(await createArt(async () => ({ ok: false }), 'p-1', () => 'blob:x').url('a-0123abcd'), null);
  assert.equal(
    await createArt(async () => {
      throw new Error('offline');
    }, 'p-1', () => 'blob:x').url('a-0123abcd'),
    null,
  );
});
