import test from 'node:test';
import assert from 'node:assert/strict';
import { iconURL, setBrand } from '../brandicon.js';

test('iconURL', () => {
  assert.equal(iconURL(180, ''), 'icon.png?s=180');
  assert.equal(iconURL(192), 'icon.png?s=192');
  assert.equal(iconURL(512, 'teal'), 'icon.png?s=512&c=teal');
});

const fake = () => {
  const props = {};
  return { dataset: {}, props, style: { setProperty: (k, v) => (props[k] = v), removeProperty: (k) => delete props[k] } };
};

test('a custom hex sets the color itself and clears with the rest', () => {
  const el = fake();
  setBrand(el, '#a1b2c3');
  assert.equal(el.dataset.brand, 'custom');
  assert.equal(el.props['--brand-color'], '#a1b2c3');
  setBrand(el, 'teal');
  assert.equal(el.dataset.brand, 'teal');
  assert.ok(!('--brand-color' in el.props));
  assert.equal(iconURL(180, '#a1b2c3'), 'icon.png?s=180&c=%23a1b2c3');
});

test('setBrand sets and clears', () => {
  const el = fake();
  setBrand(el, 'pink');
  assert.equal(el.dataset.brand, 'pink');
  setBrand(el, '');
  assert.ok(!('brand' in el.dataset));
});
