// Settings > Routers' own CSS: a router chip takes patchbay's focus ring, and sizes come from tokens, not px.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const css = readFileSync(new URL('../routers.css', import.meta.url), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');

test('a router chip has no focus ring of its own', () => {
  assert.doesNotMatch(css, /\.sode__rrow:focus-visible/);
});

test('routers.css has no px sizes', () => {
  assert.equal(css.match(/\b\d+(\.\d+)?px\b/g), null);
});
