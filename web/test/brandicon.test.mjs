import test from 'node:test';
import assert from 'node:assert/strict';
import { iconURL, setBrand } from '../brandicon.js';

test('iconURL', () => {
  assert.equal(iconURL(180, ''), 'icon.png?s=180');
  assert.equal(iconURL(192), 'icon.png?s=192');
  assert.equal(iconURL(512, 'teal'), 'icon.png?s=512&c=teal');
});

test('setBrand sets and clears', () => {
  const props = new Map();
  const el = { dataset: {}, style: { setProperty: (k, v) => props.set(k, v), removeProperty: (k) => props.delete(k) } };
  setBrand(el, 'pink');
  assert.equal(el.dataset.brand, 'pink');
  assert.ok(!props.has('--brand-color'));
  setBrand(el, '#12ab34');
  assert.ok(!('brand' in el.dataset));
  assert.equal(props.get('--brand-color'), '#12ab34');
  setBrand(el, '');
  assert.ok(!('brand' in el.dataset));
  assert.ok(!props.has('--brand-color'));
});
