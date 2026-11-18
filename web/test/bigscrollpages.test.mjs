import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { SCROLLERS } from '../bigscroll.js';

// A page with scrollers a user needs that forgets bigScrollAll draws no bars, in every browser.
test('pages with scrollers call bigScrollAll', () => {
  for (const f of ['main.js', 'log.js', 'panelpage.js']) {
    assert.match(readFileSync(new URL(`../${f}`, import.meta.url), 'utf8'), /^(if \(!id\) )?bigScrollAll\(\);/m, f);
  }
});

test('SCROLLERS entries have an axis and a known size', () => {
  for (const [sel, axis, size] of SCROLLERS) {
    assert.ok(sel);
    assert.ok(axis === 'x' || axis === 'y', sel);
    assert.ok(size === undefined || size === 'little', sel);
  }
});

test('a little entry comes before the broader big entry that also matches', () => {
  const at = (sel) => SCROLLERS.findIndex(([s]) => s === sel);
  assert.ok(at('.sode__search .sode__menu') < at('.sode__menu'));
  assert.ok(at('[data-sets] > .sode__sets-run') < at('.sode__sets-run'));
});
