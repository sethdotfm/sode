// The panel editor's sidebar rules (sidebarrules.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { setSetting, fontPercent, artProblem, levelsPatch, ART_ICONS, rightsLine, MODE_HINT, watchView } from '../sidebarrules.js';
import { artOf, lookOf } from '../panellook.js';

test('a setting at its default is dropped, so a stored panel only holds what was chosen', () => {
  let s = {};
  s = setSetting(s, 'titleBar', false);
  s = setSetting(s, 'fontSize', 150);
  s = setSetting(s, 'guard', 'hold');
  assert.deepEqual(s, { titleBar: false, fontSize: 150, guard: 'hold' });
  s = setSetting(s, 'titleBar', true);
  s = setSetting(s, 'fontSize', 100);
  s = setSetting(s, 'guard', 'none');
  assert.deepEqual(s, {});
  assert.deepEqual(setSetting({ multi: false }, 'multi', true), {});
  assert.deepEqual(setSetting({}, 'multi', false), { multi: false });
  assert.deepEqual(setSetting({}, 'mode', 'instant'), { mode: 'instant' });
  assert.deepEqual(setSetting({ mode: 'instant' }, 'mode', 'take'), {});
  assert.deepEqual(setSetting({ levels: [1] }, 'levels', []), {});
  assert.deepEqual(setSetting({}, 'levels', [2, 1]), { levels: [1, 2] });
  assert.deepEqual(setSetting({ backdrop: 'a-0123abcd' }, 'backdrop', ''), {});
  assert.deepEqual(setSetting({ x: 1 }, 'wake', true), { x: 1, wake: true });
});

test('what the editor stores is what the panel page reads', () => {
  const s = setSetting(setSetting(setSetting({}, 'titleBar', false), 'fontSize', 150), 'backdrop', 'a-0123abcd');
  assert.deepEqual({ ...lookOf(s) }, { titleBar: false, scale: 1.5, backdrop: 'a-0123abcd', wake: false, haptics: false, fullscreen: false });
});

test('font size is a whole percent from 50 to 200', () => {
  assert.deepEqual(fontPercent('120'), { value: 120 });
  assert.deepEqual(fontPercent(' 50 '), { value: 50 });
  for (const bad of ['', 'big', '49', '201', '99.5', '-1']) assert.ok(fontPercent(bad).error, bad);
});

test('artwork is checked for type and size before it is sent', () => {
  assert.equal(artProblem({ type: 'image/png', size: 1 << 20 }), '');
  for (const type of ['image/jpeg', 'image/gif', 'image/svg+xml']) assert.equal(artProblem({ type, size: 10 }), '');
  assert.match(artProblem({ type: 'image/png', size: (1 << 20) + 1 }), /1 MB/);
  assert.match(artProblem({ type: 'image/webp', size: 10 }), /JPEG, PNG, GIF or SVG/);
  assert.match(artProblem({ type: '', size: 10 }), /JPEG, PNG, GIF or SVG/);
});

test('a cell uses every level unless some are chosen, and none chosen means every level', () => {
  assert.equal(levelsPatch([1, 2, 3], [1, 2, 3]), undefined);
  assert.equal(levelsPatch([1, 2, 3], []), undefined);
  assert.deepEqual(levelsPatch([1, 2, 3], [3, 1]), [1, 3]);
});

test('every icon on offer is in the vendored sprite and reads back as cell artwork', () => {
  const sprite = readFileSync(fileURLToPath(new URL('../vendor/patchbay/icons/lucide.svg', import.meta.url)), 'utf8');
  assert.ok(ART_ICONS.length >= 20);
  for (const name of ART_ICONS) {
    assert.ok(sprite.includes(`id="${name}"`), name);
    assert.deepEqual(artOf({ art: `icon:${name}` }), { icon: name });
  }
});

test('the Rights line says what a client may do in plain words', () => {
  assert.equal(rightsLine(null), 'All Sources, Destinations, levels and Presets, read-only visibility.');
  assert.equal(rightsLine({ sources: [1, 2], levels: [1], visibility: 'hidden' }), '2 Sources, all Destinations, 1 level, all Presets, hidden visibility.');
  assert.match(rightsLine({ dsts: [], presets: ['A'] }), /no Destinations.*1 Preset,/);
});

test('Watch is offered for a saved panel the user may edit', () => {
  assert.equal(watchView(null, {}), null);
  assert.equal(watchView({ id: 'a', canEdit: false, watching: true }, {}), null);
  assert.deepEqual(watchView({ id: 'a', canEdit: true, watching: true }, {}), { id: 'a', on: true });
  assert.deepEqual(watchView({ id: 'a', canEdit: true, watching: false }, {}), { id: 'a', on: false });
});

test('a saved panel response has no watching, so the last known state stands', () => {
  const known = {};
  watchView({ id: 'a', canEdit: true, watching: true }, known);
  assert.deepEqual(watchView({ id: 'a', canEdit: true }, known), { id: 'a', on: true });
  assert.deepEqual(watchView({ id: 'b', canEdit: true }, known), { id: 'b', on: false });
});

test('each mode has a hint', () => {
  assert.ok(MODE_HINT.take && MODE_HINT.instant);
});

test('level boxes show every level ticked until a subset is stored, and the last one stays on', async () => {
  const { levelsShown, levelsToggle, levelLocked } = await import('../sidebarrules.js');
  const all = [1, 2, 3];
  assert.deepEqual(levelsShown(all, undefined), [1, 2, 3]);
  assert.deepEqual(levelsShown(all, []), [1, 2, 3]);
  assert.deepEqual(levelsShown(all, [3, 1]), [1, 3]);
  assert.deepEqual(levelsShown(all, [9]), [1, 2, 3]);
  assert.deepEqual(levelsToggle(all, undefined, 2, false), [1, 3]);
  assert.equal(levelsToggle(all, [1, 3], 2, true), undefined);
  assert.deepEqual(levelsToggle(all, [1, 3], 3, false), [1]);
  assert.equal(levelLocked(all, [1], 1), true);
  assert.equal(levelLocked(all, [1, 3], 1), false);
  assert.equal(levelLocked([1], undefined, 1), true);
  assert.equal(levelLocked(all, undefined, 1), false);
});
