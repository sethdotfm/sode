import test from 'node:test';
import assert from 'node:assert/strict';
import { iconURL, setBrand } from '../brandicon.js';

test('iconURL', () => {
  assert.equal(iconURL(180, ''), 'icon.png?s=180');
  assert.equal(iconURL(192), 'icon.png?s=192');
  assert.equal(iconURL(512, 'teal'), 'icon.png?s=512&c=teal');
});

test('setBrand sets and clears', () => {
  const el = { dataset: {} };
  setBrand(el, 'pink');
  assert.equal(el.dataset.brand, 'pink');
  setBrand(el, '');
  assert.ok(!('brand' in el.dataset));
});
