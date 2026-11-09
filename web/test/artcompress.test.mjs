// What the editor does to an upload before sending it (artcompress.js). Pure part only; the canvas needs a browser.
import test from 'node:test';
import assert from 'node:assert/strict';
import { compressPlan, keepEncoded, ART_EDGE } from '../artcompress.js';

test('a large photo shrinks to the long edge and becomes JPEG', () => {
  assert.deepEqual(compressPlan('image/jpeg', 4000, 2000), { width: ART_EDGE, height: 256, type: 'image/jpeg', resized: true });
  assert.deepEqual(compressPlan('image/jpeg', 1000, 3000), { width: 171, height: ART_EDGE, type: 'image/jpeg', resized: true });
  assert.equal(ART_EDGE, 512);
});

test('PNG stays PNG so transparency survives', () => {
  assert.equal(compressPlan('image/png', 2000, 2000).type, 'image/png');
  assert.equal(compressPlan('image/webp', 2000, 2000).type, 'image/jpeg');
});

test('GIF and SVG are never re-encoded', () => {
  assert.equal(compressPlan('image/gif', 4000, 4000), null);
  assert.equal(compressPlan('image/svg+xml', 4000, 4000), null);
  assert.equal(compressPlan('image/png', 0, 10), null);
});

test('a small image is not enlarged and is only replaced when the result is smaller', () => {
  const plan = compressPlan('image/png', 100, 50);
  assert.deepEqual([plan.width, plan.height, plan.resized], [100, 50, false]);
  assert.equal(keepEncoded(plan, 1000, 1200), false);
  assert.equal(keepEncoded(plan, 1000, 800), true);
  assert.equal(keepEncoded(compressPlan('image/png', 900, 900), 1000, 1200), true);
});
