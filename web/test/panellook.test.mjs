// Panel look and device settings (panellook.js, panelfeel.js) and the pulse rules in panels.css.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { lookOf, artOf, artBBadge, cellScale, artUrlPath } from '../panellook.js';
import { featureSupport, createWake } from '../panelfeel.js';

const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'panels.css'), 'utf8');

test('a panel with no look settings shows the title bar at normal size, follows the browser theme and has no device features', () => {
  assert.deepEqual(lookOf({}), { titleBar: true, scale: 1, theme: 'auto', wake: false, fullscreen: false, landscape: false });
  assert.deepEqual(lookOf(undefined), lookOf({}));
});

test('look settings are read as stored, and odd values fall back', () => {
  const l = lookOf({ titleBar: false, fontSize: 150, theme: 'dark', wake: true });
  assert.deepEqual(l, { titleBar: false, scale: 1.5, theme: 'dark', wake: true, fullscreen: false, landscape: false });
  assert.equal(lookOf({ fontSize: 'big' }).scale, 1);
  assert.equal(lookOf({ fontSize: 5 }).scale, 0.5);
  assert.equal(lookOf({ fontSize: 5000 }).scale, 2);
  assert.equal(lookOf({ theme: 'light' }).theme, 'light');
  assert.equal(lookOf({ theme: 'sepia' }).theme, 'auto');
  assert.equal('backdrop' in lookOf({ backdrop: 'a-0123abcd' }), false);
  assert.equal(lookOf({ wake: 'yes' }).wake, false);
});

test('cell artwork is a vendored icon or an uploaded artwork id, and nothing else', () => {
  assert.deepEqual(artOf({ art: 'icon:video' }), { icon: 'video' });
  assert.deepEqual(artOf({ art: 'a-0123abcd' }), { id: 'a-0123abcd' });
  assert.equal(artOf({ art: 'icon:' }), null);
  assert.equal(artOf({ art: 'icon:a b' }), null);
  assert.equal(artOf({ art: 'https://x/y.svg' }), null);
  assert.equal(artOf({}), null);
});

test('a library icon is an image, never a sprite icon, so Icon B draws as its corner badge', () => {
  assert.deepEqual(artOf({ art: 'lib:0123abcd' }), { id: 'lib:0123abcd', lib: true });
  assert.equal(artOf({ art: 'lib:0123ABCD' }), null);
  assert.equal(artOf({ art: 'lib:../x' }), null);
  assert.equal(artBBadge({ art: 'lib:0123abcd', artB: '2' }), true);
  assert.equal(artUrlPath('p-1', 'lib:0123abcd'), 'api/v1/icon-library/0123abcd');
});

test('a cell font size multiplies the panel one, and a bad one counts as 1', () => {
  assert.equal(cellScale({ fontSize: 50 }), 0.5);
  assert.equal(cellScale({}), 1);
  assert.equal(cellScale({ fontSize: -3 }), 0.5);
});

test('artwork is read from the panel route, with the id as a path segment', () => {
  assert.equal(artUrlPath('p-1', 'a-0123abcd'), 'api/v1/panels/p-1/artwork/a-0123abcd');
});

test('wake lock reports support from the browser object', () => {
  assert.deepEqual(featureSupport({}), { wake: false });
  assert.deepEqual(featureSupport({ wakeLock: { request() {} } }), { wake: true });
  assert.deepEqual(featureSupport(undefined), { wake: false });
});

test('the wake lock is held while the page is visible and asked for again after it hides', async () => {
  const calls = [];
  const nav = {
    wakeLock: {
      request: async () => ({
        released: false,
        release: async function () {
          this.released = true;
          calls.push('release');
        },
        addEventListener() {},
      }),
    },
  };
  const doc = {
    visibilityState: 'visible',
    listeners: {},
    addEventListener(t, f) {
      this.listeners[t] = f;
    },
    removeEventListener(t) {
      delete this.listeners[t];
    },
  };
  const wake = createWake(nav, doc);
  await wake.on();
  assert.equal(wake.held(), true);
  doc.visibilityState = 'hidden';
  await doc.listeners.visibilitychange();
  doc.visibilityState = 'visible';
  await doc.listeners.visibilitychange();
  assert.equal(wake.held(), true);
  await wake.off();
  assert.equal(wake.held(), false);
  assert.equal(doc.listeners.visibilitychange, undefined);
});

test('a refused wake lock leaves the page working', async () => {
  const wake = createWake(
    {
      wakeLock: {
        request: async () => {
          throw new Error('denied');
        },
      },
    },
    { visibilityState: 'visible', addEventListener() {}, removeEventListener() {} },
  );
  await wake.on();
  assert.equal(wake.held(), false);
});

test('a missing wake lock API makes on() do nothing', async () => {
  const wake = createWake({}, { visibilityState: 'visible', addEventListener() {}, removeEventListener() {} });
  await wake.on();
  assert.equal(wake.held(), false);
});

test('one animated grid property drives every pulse, so no cell has an animation of its own', () => {
  assert.equal([...css.matchAll(/animation\s*:/g)].length, 2); // the grid, and none under reduced motion
  assert.match(css, /--panel-pulse-s:\s*2s/);
  assert.match(css, /--panel-pulse-floor:\s*72%/);
});

// An inherited registered property animated on :root restyles the whole document every frame, on every page that
// loads panels.css (the editor idled at 27% CPU with renderer memory rising). The pulse runs on the grid, and only
// while a cell is in a pulsing state.
test('the pulse runs on a panel grid with a pulsing cell, never on the root', () => {
  assert.doesNotMatch(css, /:root\s*\{[^}]*animation/);
  const rule = /\.sode-panel__grid:has\(([^)]*)\)\s*\{[^}]*animation:\s*sode-panel-pulse[^;]*infinite/.exec(css);
  assert.ok(rule, 'the animation is on .sode-panel__grid:has(...)');
  for (const state of ['[data-s="staged"]', '[data-s="pending"]', '.sode-panel__take--armed']) assert.ok(rule[1].includes(state), `${state} starts the pulse`);
});

test('with reduced motion the pulse stops and the state is still carried by static rings', () => {
  const reduced = /@media \(prefers-reduced-motion: reduce\)\s*\{[^@]*?\}\s*\}/.exec(css)[0];
  assert.match(reduced, /animation:\s*none/);
  for (const sel of ['[data-s="staged"]', '[data-s="pending"]', '--armed']) {
    const rule = new RegExp(`${sel.replace(/[[\]"-]/g, '\\$&')}[^{]*\\{[^}]*box-shadow: inset`).test(css);
    assert.equal(rule, true, `${sel} has a ring`);
  }
});

test('fullscreen is a stored switch, and hide text only counts with artwork', async () => {
  const { textHidden } = await import('../panellook.js');
  assert.equal(lookOf({ fullscreen: true }).fullscreen, true);
  assert.equal(lookOf({ fullscreen: 'yes' }).fullscreen, false);
  assert.equal(textHidden({ hideText: true, art: 'icon:tv' }), true);
  assert.equal(textHidden({ hideText: true, art: 'a-0123abcd' }), true);
  assert.equal(textHidden({ hideText: true }), false);
  assert.equal(textHidden({ hideText: true, art: 'https://x/y.png' }), false);
  assert.equal(textHidden({ art: 'icon:tv' }), false);
});

test('enterFullscreen asks once from a press and does nothing where it is missing or already on', async () => {
  const { enterFullscreen } = await import('../panelfeel.js');
  let asked = 0;
  const doc = { fullscreenEnabled: true, documentElement: { requestFullscreen: () => (asked++, Promise.reject(new Error('refused'))) } };
  assert.equal(enterFullscreen(doc), true);
  assert.equal(asked, 1);
  assert.equal(enterFullscreen({ ...doc, fullscreenElement: {} }), false);
  assert.equal(enterFullscreen({ ...doc, fullscreenEnabled: false }), false);
  assert.equal(enterFullscreen({}), false);
  assert.equal(enterFullscreen(undefined), false);
});

test('landscape locks the turn once fullscreen starts, and a refused lock changes nothing', async () => {
  const { enterFullscreen } = await import('../panelfeel.js');
  const locked = [];
  const scr = { orientation: { lock: (v) => (locked.push(v), Promise.reject(new Error('refused'))) } };
  const doc = { fullscreenEnabled: true, documentElement: { requestFullscreen: () => Promise.resolve() } };
  assert.equal(enterFullscreen(doc, true, scr), true);
  await new Promise((r) => setTimeout(r));
  assert.deepEqual(locked, ['landscape']);
  enterFullscreen(doc, false, scr);
  await new Promise((r) => setTimeout(r));
  assert.deepEqual(locked, ['landscape']);
  assert.equal(enterFullscreen(doc, true, {}), true); // iOS: no orientation.lock
});

test('hidden text stays readable by screen readers, on the panel page and in the editor square', () => {
  assert.match(
    css,
    /\.sode-panel__cell\[data-hide-text\] \.sode-panel__text,\s*\.sode-ed__sq\[data-hide-text\] \.sode-ed__text \{[^}]*clip-path: inset\(50%\)/,
  );
});

// Icon B is a digit 0 to 16 or a letter A to Z drawn beside Icon A, so a cell can read "[camera] 2".
test('Icon B is one of the fixed glyphs, and anything else is dropped', async () => {
  const { artBOf, textHidden, ART_B_CHOICES } = await import('../panellook.js');
  assert.deepEqual(ART_B_CHOICES.slice(0, 3), ['0', '1', '2']);
  assert.equal(ART_B_CHOICES.length, 17 + 26);
  assert.equal(ART_B_CHOICES.at(-1), 'Z');
  for (const g of ART_B_CHOICES) assert.equal(artBOf({ artB: g }), g);
  for (const bad of ['17', 'a', 'AB', '', 7, null, '<b>']) assert.equal(artBOf({ artB: bad }), '', String(bad));
  assert.equal(artBOf({}), '');
  // Icon B alone counts as artwork, so Hide text can apply to it.
  assert.equal(textHidden({ hideText: true, artB: '2' }), true);
  assert.equal(textHidden({ hideText: true, artB: 'bogus' }), false);
});

// An uploaded image fills the cell and Icon B draws as a corner badge over it; beside a vendored icon Icon B keeps
// its place in the row.
test('Icon B is a corner badge over an image and sits in the row beside an icon', async () => {
  const { artBOf, artBBadge } = await import('../panellook.js');
  assert.equal(artBOf({ art: 'a-0123abcd', artB: '2' }), '2');
  assert.equal(artBBadge({ art: 'a-0123abcd', artB: '2' }), true);
  assert.equal(artBBadge({ art: 'icon:camera', artB: '2' }), false);
  assert.equal(artBBadge({ artB: '2' }), false, 'Icon B alone fills the row');
  assert.equal(artBBadge({ art: 'a-0123abcd' }), false, 'no Icon B, no badge');
  assert.equal(artBOf({ art: 'https://x/y.png', artB: '2' }), '2', 'a dropped art value is no image');
  assert.equal(artBBadge({ art: 'https://x/y.png', artB: '2' }), false);
});

// Artwork always draws with contain, never cover; an image fills the whole row and the badge sits in a corner.
test('an image fills the row with contain and the badge is held in a corner', () => {
  assert.match(css, /\.sode-panel__art-a \{[^}]*object-fit: contain/);
  assert.doesNotMatch(css, /object-fit: cover/);
  assert.match(css, /\.sode-panel__art--fill > \.sode-panel__art-a \{[^}]*width: 100%/);
  assert.match(css, /\.sode-panel__art-b--badge \{[^}]*position: absolute/);
});

test('the artwork row sizes Icon B from the cell, so it scales with the square', () => {
  assert.match(css, /\.sode-panel__art \{[^}]*container-type: size/);
  assert.match(css, /\.sode-panel__art-b \{[^}]*cqh/);
});

// Side by side, Icon A takes 40% of the row and Icon B 60%; alone, either fills the row as before.
test('Icon A and Icon B share the row 40 to 60, and each alone fills it', () => {
  assert.match(css, /\.sode-panel__art-a:not\(:only-child\) \{[^}]*flex: 0 1 40%/);
  assert.match(css, /\.sode-panel__art-b:not\(:only-child\) \{[^}]*flex: 0 1 60%/);
  assert.match(css, /\.sode-panel__art-b--wide \{[^}]*font-size: min\(/);
});

// With the text hidden nothing sits over the artwork, so it is drawn at full strength in the cell, the editor square
// and the sidebar preview alike.
test('artwork is dimmed under text and at full strength when the text is hidden', () => {
  assert.match(css, /\.sode-panel__art \{[^}]*opacity: 0\.3/);
  assert.match(css, /\[data-hide-text\] > \.sode-panel__art \{[^}]*opacity: 1/);
});

test('a wake lock the browser cannot honour gets a note, and https is named', async () => {
  const { featureNotes } = await import('../panelfeel.js');
  const on = { wake: true };
  assert.equal(featureNotes({}, {}, false).length, 0);
  assert.match(featureNotes(on, {}, false)[0], /https/);
  assert.match(featureNotes(on, {}, true)[0], /cannot keep the screen awake/);
  assert.equal(featureNotes(on, { wakeLock: { request() {} } }, true).length, 0);
});

test('a portrait page with Keep landscape on is covered, since iOS cannot lock the turn', () => {
  assert.match(css, /@media \(orientation: portrait\) \{\s*html\[data-sode-landscape\] \.sode-panel__turn \{[^}]*display: flex/);
});
