import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { accentFor, accentTokens, brandCaution, brandHex, contrast, pickerColors, oklch, passes, reservedBy, toHex, THEMES, MIN_CONTRAST } from '../accent.js';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const tokensCss = readFileSync(join(web, 'vendor/patchbay/css/patchbay.tokens.css'), 'utf8');

// The named colors as config.BrandColors sends them in the hello.
const NAMED = {
  blue: '#5fb2e8',
  teal: '#5ccfc9',
  sky: '#4dc3dd',
  slate: '#a3b4d4',
  violet: '#a67ef0',
  green: '#5fd09a',
  amber: '#e8a35e',
  pink: '#e87bb3',
};
const OFFERED = ['blue', 'teal', 'sky', 'slate', 'violet'];

test('NAMED is config.BrandColors', () => {
  const src = readFileSync(join(web, '../internal/config/config.go'), 'utf8');
  const go = [...src.matchAll(/\{"(\w+)", "\w+", "(#[0-9a-f]{6})", (true|false)\}/g)];
  assert.deepEqual(Object.fromEntries(go.map((m) => [m[1], m[2]])), NAMED);
  assert.deepEqual(
    go.filter((m) => m[3] === 'false').map((m) => m[1]),
    OFFERED,
  );
});

function checkAccent(hex) {
  const a = accentFor(hex);
  const bad = [];
  for (const theme of ['light', 'dark']) {
    for (const v of [a[theme], a[`${theme}Hover`]]) {
      for (const s of THEMES[theme].surfaces) if (contrast(v, s) < MIN_CONTRAST) bad.push(`${hex} ${theme} ${v} on ${s}: ${contrast(v, s).toFixed(2)}`);
      const on = THEMES[theme].onAccent;
      if (contrast(v, on) < MIN_CONTRAST) bad.push(`${hex} ${theme} text ${on} on ${v}: ${contrast(v, on).toFixed(2)}`);
    }
  }
  return bad;
}

// Each theme's light-dark() pair of a patchbay token.
function tokenPair(name) {
  const m = tokensCss.match(new RegExp(`${name}:\\s*light-dark\\((#[0-9a-f]{6}),\\s*(#[0-9a-f]{6})\\)`));
  assert.ok(m, name);
  return [m[1], m[2]];
}

test('the theme surfaces and text on accent are patchbay’s', () => {
  const [lightOn, darkOn] = tokenPair('--pb-text-on-accent');
  assert.equal(THEMES.light.onAccent, lightOn);
  assert.equal(THEMES.dark.onAccent, darkOn);
  for (const name of ['sunken', 'base', 'raised', 'overlay', 'high', 'control']) {
    const [l, d] = tokenPair(`--pb-surface-${name}`);
    assert.ok(THEMES.light.surfaces.includes(l), `light ${name} ${l}`);
    assert.ok(THEMES.dark.surfaces.includes(d), `dark ${name} ${d}`);
  }
});

test('every named color gives an accent that passes 4.5:1 in both themes, and text on its fill', () => {
  for (const [name, hex] of Object.entries(NAMED)) assert.deepEqual(checkAccent(hex), [], name);
});

test('a sweep of custom hues at several lightness values passes 4.5:1 in both themes', () => {
  const bad = [];
  let n = 0;
  for (let h = 0; h < 360; h += 10) {
    for (const L of [0.2, 0.35, 0.5, 0.65, 0.8, 0.95]) {
      for (const C of [0.04, 0.12, 0.25]) {
        bad.push(...checkAccent(toHex(L, C, h)));
        n++;
      }
    }
  }
  for (const hex of ['#000000', '#ffffff', '#808080', '#ff0000', '#ffff00', '#0000ff']) bad.push(...checkAccent(hex));
  assert.ok(n > 600);
  assert.deepEqual(bad.slice(0, 5), []);
});

test('the accent keeps the hue, and keeps a color that already passes', () => {
  // Blue in dark mode already passes, so it is the border color itself.
  assert.equal(accentFor(NAMED.blue).dark, NAMED.blue);
  for (const hex of Object.values(NAMED)) {
    const [, , h] = oklch(hex);
    for (const theme of ['light', 'dark']) {
      const [, , ah] = oklch(accentFor(hex)[theme]);
      const d = Math.min(Math.abs(ah - h), 360 - Math.abs(ah - h));
      assert.ok(d < 4, `${hex} ${theme}: hue ${h.toFixed(1)} became ${ah.toFixed(1)}`);
    }
  }
});

test('passes checks the theme’s surfaces and its text on accent', () => {
  assert.ok(passes('#006798', 'light'));
  assert.ok(!passes('#5fb2e8', 'light'));
  assert.ok(passes('#5fb2e8', 'dark'));
  assert.ok(!passes('#006798', 'dark'));
});

test('no offered color is in the reserved zone; the retired ones are', () => {
  for (const name of OFFERED) assert.deepEqual(reservedBy(NAMED[name]), [], name);
  assert.deepEqual(reservedBy(NAMED.green), ['routed']);
  assert.deepEqual(reservedBy(NAMED.amber), ['caution']);
  assert.deepEqual(reservedBy(NAMED.pink), ['protected']);
});

test('the reserved zone covers each state hue, tie lines and Step in', () => {
  const states = {
    '--pb-hue-red': 'failure',
    '--pb-hue-amber': 'caution',
    '--pb-hue-green': 'routed',
    '--pb-hue-magenta': 'protected',
    '--pb-tieline': 'tie line',
    '--pb-signal-10': 'protected',
  };
  for (const [token, state] of Object.entries(states)) {
    for (const hex of tokenPair(token)) assert.ok(reservedBy(hex).includes(state), `${token} ${hex}: ${reservedBy(hex)}`);
  }
  // 30 degrees each side.
  assert.ok(reservedBy(toHex(0.6, 0.12, 186)).includes('routed'));
  assert.deepEqual(reservedBy(toHex(0.6, 0.12, 200)), []);
  assert.deepEqual(reservedBy(toHex(0.6, 0.12, 300)), []);
  assert.ok(reservedBy(toHex(0.6, 0.12, 320)).includes('protected'));
  // A grey has no hue.
  for (const hex of ['#808080', '#ffffff', '#000000']) assert.deepEqual(reservedBy(hex), [], hex);
});

test('accentFor says whether the hue is reserved', () => {
  assert.equal(accentFor(NAMED.blue).reserved, false);
  assert.equal(accentFor(NAMED.amber).reserved, true);
  assert.equal(accentFor('#7f9a5a').reserved, true);
});

test('accentTokens sets the accent, hover and wash for both themes', () => {
  const t = accentTokens(accentFor(NAMED.blue));
  assert.match(t['--pb-accent'], /^light-dark\(#[0-9a-f]{6}, #5fb2e8\)$/);
  assert.match(t['--pb-accent-hover'], /^light-dark\(#[0-9a-f]{6}, #[0-9a-f]{6}\)$/);
  assert.equal(t['--pb-accent-wash'].split('rgba(95, 178, 232, 0.14)').length, 2);
});

test('the caution shows for a reserved named or custom color, and not for an offered one or Off', () => {
  const colors = Object.entries(NAMED).map(([name, hex]) => ({ name, hex }));
  for (const name of OFFERED) assert.equal(brandCaution(name, colors), false, name);
  for (const name of ['green', 'amber', 'pink']) assert.equal(brandCaution(name, colors), true, name);
  assert.equal(brandCaution('#e05050', colors), true);
  assert.equal(brandCaution('#4060e0', colors), false);
  assert.equal(brandCaution('', colors), false);
});

test('the picker leaves out a retired color unless it is the saved one', () => {
  const colors = [{ name: 'blue' }, { name: 'amber', retired: true }, { name: 'pink', retired: true }];
  assert.deepEqual(
    pickerColors(colors, 'blue').map((c) => c.name),
    ['blue'],
  );
  assert.deepEqual(
    pickerColors(colors, 'amber').map((c) => c.name),
    ['blue', 'amber'],
  );
});

test('brandHex reads a name from the hello list, or a custom color', () => {
  const colors = [{ name: 'blue', hex: '#5fb2e8' }];
  assert.equal(brandHex('blue', colors), '#5fb2e8');
  assert.equal(brandHex('#A0B0C0', colors), '#a0b0c0');
  assert.equal(brandHex('teal', colors), '');
  assert.equal(brandHex('', colors), '');
  assert.equal(brandHex('blue', undefined), '');
});
