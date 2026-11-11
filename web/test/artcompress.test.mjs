// What the editor does to an upload before sending it (artcompress.js). The canvas needs a browser
// (TestChromeEditorArtUpload); the rules and the GIF and SVG paths run here.
import test from 'node:test';
import assert from 'node:assert/strict';
import { compressPlan, keepEncoded, gifSize, gifProblem, prepareArt, ART_EDGE } from '../artcompress.js';
import { ART_MAX_BYTES } from '../sidebarrules.js';

test('a large image shrinks so neither side passes 1024, aspect kept', () => {
  assert.equal(ART_EDGE, 1024);
  assert.deepEqual(compressPlan('image/jpeg', 4000, 2000), { width: 1024, height: 512, type: 'image/jpeg', resized: true });
  assert.deepEqual(compressPlan('image/jpeg', 1000, 3000), { width: 341, height: 1024, type: 'image/jpeg', resized: true });
  assert.deepEqual(compressPlan('image/png', 1025, 1025), { width: 1024, height: 1024, type: 'image/png', resized: true });
});

test('the type is kept: PNG stays PNG so transparency survives, JPEG stays JPEG', () => {
  assert.equal(compressPlan('image/png', 2000, 2000).type, 'image/png');
  assert.equal(compressPlan('image/jpeg', 2000, 2000).type, 'image/jpeg');
});

test('GIF and SVG are never re-encoded', () => {
  assert.equal(compressPlan('image/gif', 4000, 4000), null);
  assert.equal(compressPlan('image/svg+xml', 4000, 4000), null);
  assert.equal(compressPlan('image/png', 0, 10), null);
});

test('a small image is not enlarged and is only replaced when the result is smaller', () => {
  const plan = compressPlan('image/png', 100, 50);
  assert.deepEqual([plan.width, plan.height, plan.resized], [100, 50, false]);
  assert.deepEqual([compressPlan('image/jpeg', 1024, 300).width, compressPlan('image/jpeg', 1024, 300).resized], [1024, false]);
  assert.equal(keepEncoded(plan, 1000, 1200), false);
  assert.equal(keepEncoded(plan, 1000, 800), true);
  assert.equal(keepEncoded(compressPlan('image/png', 1900, 1900), 1000, 1200), true);
});

// A GIF header: signature, then the logical screen width and height, little-endian.
const gifHead = (w, h) => new Uint8Array([0x47, 0x49, 0x46, 0x38, 0x39, 0x61, w & 255, w >> 8, h & 255, h >> 8]);
const gifFile = (w, h, size = 10) => {
  const bytes = new Uint8Array(size);
  bytes.set(gifHead(w, h));
  return new Blob([bytes], { type: 'image/gif' });
};

test('a GIF is measured from its header, without decoding it', () => {
  assert.deepEqual(gifSize(gifHead(1024, 300)), { width: 1024, height: 300 });
  assert.deepEqual(gifSize(gifHead(4000, 1)), { width: 4000, height: 1 });
  assert.equal(gifSize(new Uint8Array([1, 2, 3])), null);
  assert.equal(gifSize(new TextEncoder().encode('not a gif at all')), null);
});

test('a GIF within 1024 by 1024 and 2 MB passes; a larger one is refused with how to fix it', () => {
  assert.equal(gifProblem({ width: 1024, height: 1024 }, ART_MAX_BYTES), '');
  assert.match(gifProblem({ width: 1025, height: 10 }, 100), /resize it and upload again/i);
  assert.match(gifProblem({ width: 10, height: 1025 }, 100), /resize it and upload again/i);
  assert.match(gifProblem({ width: 10, height: 10 }, ART_MAX_BYTES + 1), /resize it and upload again/i);
  // An unreadable header is left to the server, which reads the bytes again.
  assert.equal(gifProblem(null, 100), '');
});

test('a GIF within the limits is sent as it is, so its animation is kept', async () => {
  const f = gifFile(800, 600, 4096);
  const out = await prepareArt(f, null);
  assert.equal(out.problem, '');
  assert.equal(out.blob, f);
});

test('an oversize GIF is refused before sending', async () => {
  const big = await prepareArt(gifFile(2000, 100), null);
  assert.match(big.problem, /resize it and upload again/i);
  assert.equal(big.blob, undefined);
  const heavy = await prepareArt(gifFile(10, 10, ART_MAX_BYTES + 1), null);
  assert.match(heavy.problem, /resize it and upload again/i);
});

test('SVG is sent untouched, with only the 2 MB cap', async () => {
  const svg = new Blob(['<svg xmlns="http://www.w3.org/2000/svg"/>'], { type: 'image/svg+xml' });
  const out = await prepareArt(svg, null);
  assert.equal(out.blob, svg);
  assert.equal(out.problem, '');
  const huge = await prepareArt(new Blob([new Uint8Array(ART_MAX_BYTES + 1)], { type: 'image/svg+xml' }), null);
  assert.match(huge.problem, /2 MB/);
});

test('another type is refused before anything is read', async () => {
  const out = await prepareArt(new Blob(['x'], { type: 'image/webp' }), null);
  assert.match(out.problem, /JPEG, PNG, GIF or SVG/);
  assert.equal(out.blob, undefined);
});
