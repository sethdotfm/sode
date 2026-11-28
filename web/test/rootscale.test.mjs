import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'base.css'), 'utf8');

// The big screen rule in base.css, read back as numbers so the test follows the file and not a copy of the formula.
const big = css.slice(css.indexOf('@media (min-width: 2400px)'));
const rule = /clamp\(([\d.]+)%, ([\d.]+)% \+ \(min\(100vw, ([\d.]+)vh\) - (\d+)px\) \/ (\d+), ([\d.]+)%\)/g;
const [desk, grid] = [...big.matchAll(rule)].map((m) => m.slice(1).map(Number));

// root gives the root font size in px of a W x H window: 16 px below the media rule, else the clamp.
const root = (r, w, h) => {
  const [lo, base, vh, from, per, hi] = r;
  if (w < from) return (16 * lo) / 100;
  const grow = (Math.min(w, (vh * h) / 100) - from) / per;
  return Math.min((16 * hi) / 100, Math.max((16 * lo) / 100, (16 * base) / 100 + grow));
};
const near = (got, want, what) => assert.ok(Math.abs(got - want) < 0.05, `${what}: ${got.toFixed(2)} px, want ${want} px`);

test('the desktop and Grid panel rules are clamps bound by the screen height', () => {
  assert.ok(desk && grid, 'two clamp rules under @media (min-width: 2400px)');
  assert.ok(big.indexOf('html:not([data-layout="phone"])') < big.indexOf('.sode-gridpanel'), 'the desktop rule comes first');
});

test('the root is 16 px up to 2400 px wide and grows with the screen height', () => {
  for (const [w, h] of [
    [1366, 768],
    [1920, 1080],
    [2400, 1350],
    [2500, 750],
  ])
    near(root(desk, w, h), 16, `${w}x${h}`);
  near(root(desk, 2560, 1440), 16.9, '2560x1440');
  near(root(desk, 3440, 1440), 16.9, '3440x1440, an ultrawide');
  near(root(desk, 3840, 2160), 24, '3840x2160');
  near(root(desk, 7680, 4320), 24, '7680x4320 stops at 150%');
});

test('the Grid panel page grows from 90% to 135%', () => {
  near(root(grid, 1920, 1080), 14.4, 'Grid 1920x1080');
  near(root(grid, 2400, 1350), 14.4, 'Grid 2400x1350');
  near(root(grid, 2560, 1440), 15.2, 'Grid 2560x1440');
  near(root(grid, 3840, 2160), 21.6, 'Grid 3840x2160');
});
