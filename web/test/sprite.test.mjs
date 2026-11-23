import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { symbolMarkup, localHref, nameOfHref } from '../sprite.js';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const sprite = readFileSync(join(web, 'vendor/patchbay/icons/lucide.svg'), 'utf8');

test('a symbol is cut from the sprite under the local id, and only that symbol', () => {
  const s = symbolMarkup(sprite, 'x');
  assert.match(s, /^<symbol id="pb-i-x"[\s>]/);
  assert.match(s, /<\/symbol>$/);
  assert.equal(s.match(/<symbol/g).length, 1);
  assert.ok(!symbolMarkup(sprite, 'check').includes('check-check'));
  assert.equal(symbolMarkup(sprite, 'no-such-icon'), '');
  assert.equal(symbolMarkup(sprite, 'a"b'), '');
});

test('local hrefs read back as the icon name', () => {
  assert.equal(localHref('settings'), '#pb-i-settings');
  assert.equal(nameOfHref(localHref('chevron-left')), 'chevron-left');
  assert.equal(nameOfHref('#settings'), '');
});

// Chrome loads and lays out one copy of the external sprite (about 16,700 nodes) for each <use>, and frees it only at a major GC. See TestChromeRunPageHoldsNoMemory.
test('no page or module points a <use> at the external sprite', () => {
  const bad = readdirSync(web)
    .filter((f) => /\.(js|html)$/.test(f))
    .filter((f) => /lucide\.svg#/.test(readFileSync(join(web, f), 'utf8')));
  assert.deepEqual(bad, []);
});

test('Sode-owned icons are in sodeicons.svg, never in the vendored sprite', () => {
  const own = readFileSync(join(web, 'sodeicons.svg'), 'utf8');
  const names = [...own.matchAll(/<symbol id="([^"]+)"/g)].map((m) => m[1]);
  assert.ok(names.includes('sode-grid'));
  for (const n of names) {
    assert.ok(n.startsWith('sode-'), n);
    assert.match(symbolMarkup(own, n), new RegExp(`^<symbol id="pb-i-${n}"`));
    assert.equal(symbolMarkup(sprite, n), '');
  }
});
