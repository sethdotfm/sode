// The panel editor's sidebar rules (sidebarrules.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { setSetting, fontPercent, artProblem, levelsPatch, ART_ICONS, artFilter, rightsRows } from '../sidebarrules.js';
import { artOf, lookOf } from '../panellook.js';

// The server reads a missing askInstanceName back as true, so dropping it would leave the editor unsaved after off and on.
test('askInstanceName is kept as chosen, true included', () => {
  assert.deepEqual(setSetting({}, 'askInstanceName', false), { askInstanceName: false });
  assert.deepEqual(setSetting({ askInstanceName: false }, 'askInstanceName', true), { askInstanceName: true });
});

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
  assert.deepEqual(setSetting({ theme: 'dark' }, 'theme', 'auto'), {});
  assert.deepEqual(setSetting({ x: 1 }, 'wake', true), { x: 1, wake: true });
});

test('what the editor stores is what the panel page reads', () => {
  const s = setSetting(setSetting(setSetting({}, 'titleBar', false), 'fontSize', 150), 'theme', 'light');
  assert.deepEqual({ ...lookOf(s) }, { titleBar: false, scale: 1.5, theme: 'light', wake: false, fullscreen: false, landscape: false });
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

test('the standard cell kind icons are offered as artwork, type icons first', async () => {
  const { CATEGORY_ICON } = await import('../findrules.js');
  const kinds = Object.values(CATEGORY_ICON);
  assert.deepEqual(ART_ICONS.slice(0, kinds.length), kinds);
  for (const name of ['check', 'equal', 'x', 'search', 'undo-2', 'redo-2', 'refresh-cw', 'arrow-left-right', 'plus', 'minus'])
    assert.ok(ART_ICONS.includes(name), name);
  assert.equal(new Set(ART_ICONS).size, ART_ICONS.length, 'no icon is offered twice');
});

test('the Rights table says what a client may do', () => {
  assert.deepEqual(rightsRows(null), [['Sources', 'All'], ['Destinations', 'All'], ['Levels', 'All'], ['Presets', 'All'], ['Visibility', 'Read-only']]);
  const r = rightsRows({ sources: [1, 2], dsts: [], visibility: 'hidden' });
  assert.deepEqual(r.map((x) => x[1]), ['2', 'None', 'All', 'All', 'Hidden']);
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

test('fixed destinations keep the order they were added in, at most 16, and an empty list turns the setting off', async () => {
  const { addFixed, dropFixed, FIXED_MAX } = await import('../sidebarrules.js');
  let s = addFixed({ mode: 'instant' }, 7);
  s = addFixed(s, 2);
  s = addFixed(s, 7);
  assert.deepEqual(s, { mode: 'instant', fixedDests: [7, 2] });
  assert.deepEqual(dropFixed(s, 7), { mode: 'instant', fixedDests: [2] });
  assert.deepEqual(dropFixed(dropFixed(s, 7), 2), { mode: 'instant' });
  assert.equal(FIXED_MAX, 16);
  let full = {};
  for (let n = 0; n < 20; n++) full = addFixed(full, n);
  assert.equal(full.fixedDests.length, 16);
});

test('adding to a stored list the server would refuse starts a new list', async () => {
  const { addFixed, dropFixed } = await import('../sidebarrules.js');
  assert.deepEqual(addFixed({ fixedDests: [3, 3] }, 5), { fixedDests: [5] });
  assert.deepEqual(dropFixed({ fixedDests: [-1, 4] }, 4), {});
});

test('the icon search keeps names that contain every typed word', () => {
  assert.deepEqual(artFilter(['arrow-left', 'arrow-right', 'tv'], ' Arrow '), ['arrow-left', 'arrow-right']);
  assert.deepEqual(artFilter(['arrow-left', 'tv'], 'arrow left'), ['arrow-left']);
  assert.deepEqual(artFilter(['tv'], ''), ['tv']);
});
