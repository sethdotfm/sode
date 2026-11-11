// What the panel editor reads from its URL and from /api/v1/panels (editorrules.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  editorTarget,
  groupPanels,
  newPanel,
  saveBody,
  isDirty,
  placeMenu,
  kindLabel,
  kindChoices,
  menuChord,
  chordText,
  saveChord,
  signInNote,
  openFailNote,
  typeLabel,
} from '../editorrules.js';
import { createGrid, merge, select } from '../gridmodel.js';

test('the editor opens the panel in ?panel=, and anything else starts a new one', () => {
  assert.deepEqual(editorTarget('?panel=p-1a2b'), { id: 'p-1a2b' });
  assert.deepEqual(editorTarget('?panel=p-1a2b&x=1'), { id: 'p-1a2b' });
  assert.deepEqual(editorTarget('?new'), { id: '' });
  assert.deepEqual(editorTarget(''), { id: '' });
  assert.deepEqual(editorTarget('?panel=../x'), { id: '' });
});

test('the picker lists panels the user may change as My panels and use-only ones as Shared with me, by name', () => {
  const g = groupPanels([
    { id: 'a', name: 'Zed', canEdit: true },
    { id: 'b', name: 'Beta', canEdit: false },
    { id: 'c', name: 'alpha', canEdit: true },
  ]);
  assert.deepEqual(
    g.mine.map((p) => p.id),
    ['c', 'a'],
  );
  assert.deepEqual(
    g.shared.map((p) => p.id),
    ['b'],
  );
  assert.deepEqual(groupPanels(undefined), { mine: [], shared: [] });
});

test('a new panel is 4x8, acts as the signed-in user, is shared with nobody and has no cells', () => {
  const p = newPanel();
  assert.deepEqual([p.rows, p.cols, p.pages[0].cells, p.acts, p.share.mode], [4, 8, [], 'session', 'me']);
  assert.ok(p.name);
});

test('the save body keeps every field the editor does not own and swaps in the name and the grid', () => {
  const loaded = {
    id: 'p-1',
    owner: 'sam',
    canEdit: true,
    clientRights: { x: 1 },
    acts: 'api',
    share: { mode: 'all' },
    settings: { mode: 'instant' },
    future: { k: 1 },
    rows: 4,
    cols: 8,
    cells: [],
  };
  const g = merge(select(select(createGrid({ rows: 2, cols: 2 }), 1, 1), 2, 2, true));
  const b = saveBody(loaded, ' Studio ', g);
  assert.equal(b.name, 'Studio');
  assert.deepEqual([b.rows, b.cols, b.pages[0].cells.length], [2, 2, 1]);
  assert.deepEqual([b.acts, b.share.mode, b.settings.mode, b.future.k], ['api', 'all', 'instant', 1]);
  assert.ok(!('canEdit' in b) && !('clientRights' in b));
  assert.equal(loaded.rows, 4);
});

test('dirty means the name or the grid differs from what was loaded', () => {
  const loaded = { name: 'A', rows: 2, cols: 2, cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'take' }] };
  const g = createGrid(loaded);
  assert.equal(isDirty(loaded, 'A', g), false);
  const reordered = { ...loaded, cells: [{ kind: 'take', rs: 1, cs: 1, c: 1, r: 1 }] };
  assert.equal(isDirty(reordered, 'A', g), false);
  assert.equal(isDirty(loaded, 'B', g), true);
  assert.equal(isDirty(loaded, 'A', merge(select(select(g, 1, 1), 2, 2, true))), true);
  assert.equal(isDirty(null, 'New panel', createGrid()), false);
});

test('the menu goes right of the selection, else left, else below, and stays inside its area', () => {
  const vp = { left: 0, top: 0, right: 1000, bottom: 600 };
  const menu = { w: 200, h: 120 };
  assert.deepEqual(placeMenu({ left: 100, top: 50, right: 200, bottom: 100 }, menu, vp, 8), { side: 'right', left: 208, top: 50 });
  assert.deepEqual(placeMenu({ left: 700, top: 50, right: 900, bottom: 100 }, menu, vp, 8), { side: 'left', left: 492, top: 50 });
  assert.deepEqual(placeMenu({ left: 10, top: 50, right: 990, bottom: 100 }, menu, vp, 8), { side: 'below', left: 10, top: 108 });
  assert.equal(placeMenu({ left: 100, top: 550, right: 200, bottom: 590 }, menu, vp, 8).top, 480);
  // The area ends where the sidebar starts: a selection in the last column flips the menu to its left.
  const area = { left: 0, top: 0, right: 700, bottom: 600 };
  assert.deepEqual(placeMenu({ left: 500, top: 50, right: 690, bottom: 100 }, menu, area, 8), { side: 'left', left: 292, top: 50 });
  assert.equal(placeMenu({ left: 20, top: 50, right: 680, bottom: 100 }, menu, area, 8).side, 'below');
});

// The sidebar edits the selected cell, so the menu must not sit over it: beside the grid it bounds the menu's area,
// under the grid (the narrow layout) it does not.
test('the menu area is the viewport, cut at the sidebar when the sidebar is beside the grid', async () => {
  const { menuArea } = await import('../editorrules.js');
  const vp = { w: 1600, h: 900 };
  const grid = { left: 40, top: 100, right: 1100, bottom: 640 };
  assert.deepEqual(menuArea(vp, { left: 1140, top: 100, right: 1560, bottom: 800 }, grid), { left: 0, top: 0, right: 1140, bottom: 900 });
  assert.deepEqual(menuArea(vp, { left: 40, top: 700, right: 1560, bottom: 1400 }, grid), { left: 0, top: 0, right: 1600, bottom: 900 });
  assert.deepEqual(menuArea(vp, null, grid), { left: 0, top: 0, right: 1600, bottom: 900 });
});

// A flyout (Type, Find) opens away from the selection, so on the menu's own side; when that side has no room it
// flips to the other, and when neither fits it takes the side with more room.
test('a flyout opens on the menu side when it fits there, else on the other side', async () => {
  const { subSide } = await import('../editorrules.js');
  const area = { left: 0, top: 0, right: 1000, bottom: 600 };
  assert.equal(subSide({ side: 'right', left: 300 }, 200, 300, area, 8), 'right');
  assert.equal(subSide({ side: 'right', left: 600 }, 200, 300, area, 8), 'left');
  assert.equal(subSide({ side: 'left', left: 310 }, 200, 300, area, 8), 'left');
  assert.equal(subSide({ side: 'left', left: 100 }, 200, 300, area, 8), 'right');
  assert.equal(subSide({ side: 'below', left: 100 }, 200, 300, area, 8), 'right');
  // Neither side fits a 600 flyout from a menu at 300..500: the right has 492, the left 292.
  assert.equal(subSide({ side: 'left', left: 300 }, 200, 600, area, 8), 'right');
  assert.equal(subSide({ side: 'right', left: 600 }, 200, 700, area, 8), 'left');
});

// The Find flyout sits beside its menu item and is pulled back inside the viewport when it would run off the bottom or
// the right; it never moves when it already fits.
test('a flyout that runs off the viewport is shifted back inside, and one that fits stays put', async () => {
  const { flyoutShift } = await import('../editorrules.js');
  const vp = { w: 1000, h: 600 };
  assert.deepEqual(flyoutShift({ left: 300, top: 100, right: 600, bottom: 400 }, vp, 8), { x: 0, y: 0 });
  assert.deepEqual(flyoutShift({ left: 300, top: 400, right: 600, bottom: 700 }, vp, 8), { x: 0, y: -108 });
  assert.deepEqual(flyoutShift({ left: 900, top: 100, right: 1200, bottom: 400 }, vp, 8), { x: -208, y: 0 });
  assert.deepEqual(flyoutShift({ left: -50, top: -20, right: 250, bottom: 280 }, vp, 8), { x: 58, y: 28 });
  // Taller than the viewport: the top edge wins, so the box and the first results stay reachable.
  assert.deepEqual(flyoutShift({ left: 100, top: 100, right: 400, bottom: 900 }, vp, 8), { x: 0, y: -92 });
});

test('Type offers every kind the panel page knows, then Empty', () => {
  const c = kindChoices();
  assert.deepEqual(c.map((x) => x.kind).slice(-1), ['']);
  assert.ok(c.some((x) => x.kind === 'take'));
  assert.equal(kindLabel('destination'), 'Destination');
  assert.equal(kindLabel(''), 'Empty');
  assert.ok(c.every((x) => !x.disabled));
});

test('while Fixed destination is on, Type offers Destination greyed with the reason beside it', () => {
  const d = kindChoices({ fixed: true }).find((x) => x.kind === 'destination');
  assert.equal(d.disabled, true);
  assert.equal(d.label, 'Destination');
  assert.equal(d.hint, 'Fixed destination is on');
  assert.ok(
    kindChoices({ fixed: true })
      .filter((x) => x.kind !== 'destination')
      .every((x) => !x.disabled),
  );
});

test('a change to the fixed destinations is an unsaved change and is saved, in its own order', () => {
  const loaded = { ...newPanel(), id: 'p-0123abcd', settings: { fixedDests: [4, 2] } };
  const g = createGrid(loaded);
  assert.equal(isDirty(loaded, loaded.name, g, { settings: { fixedDests: [4, 2] } }), false);
  assert.equal(isDirty(loaded, loaded.name, g, { settings: { fixedDests: [2, 4] } }), true);
  assert.equal(isDirty(loaded, loaded.name, g, { settings: {} }), true);
  assert.deepEqual(saveBody(loaded, loaded.name, g, { settings: { fixedDests: [9] } }).settings, { fixedDests: [9] });
});

test('panel fields: edited settings are saved and make the panel dirty, in any key order', () => {
  const loaded = { id: 'p-1', name: 'A', acts: 'session', share: { mode: 'me' }, rows: 1, cols: 1, cells: [], settings: { guard: 'hold', fontSize: 120 } };
  const g = createGrid(loaded);
  assert.equal(isDirty(loaded, 'A', g, { settings: { fontSize: 120, guard: 'hold' } }), false);
  assert.equal(isDirty(loaded, 'A', g, { settings: { guard: 'hold' } }), true);
  assert.equal(isDirty(loaded, 'A', g, { acts: 'api' }), true);
  assert.equal(isDirty(loaded, 'A', g, { clientRights: { levels: [1] } }), true);
  assert.equal(isDirty(loaded, 'A', g, {}), false);
  const body = saveBody(loaded, 'A', g, { settings: { titleBar: false }, acts: 'api' });
  assert.deepEqual(body.settings, { titleBar: false });
  assert.equal(body.acts, 'api');
  assert.equal('clientRights' in body, false);
  assert.deepEqual(saveBody(loaded, 'A', g, { clientRights: { levels: [1] } }).clientRights, { levels: [1] });
  assert.deepEqual(saveBody(loaded, 'A', g).settings, loaded.settings);
});

test('a short name is an unsaved field: it dirties the editor, saves, and clearing it counts', () => {
  const loaded = { ...newPanel(), id: 'p-0123abcd', name: 'A', slug: 'lobby' };
  const g = createGrid(loaded);
  assert.equal(isDirty(loaded, 'A', g, { slug: 'lobby' }), false);
  assert.equal(isDirty(loaded, 'A', g, { slug: 'wall' }), true);
  assert.equal(isDirty(loaded, 'A', g, { slug: '' }), true);
  assert.equal(isDirty({ ...loaded, slug: undefined }, 'A', g, { slug: '' }), false);
  assert.equal(saveBody(loaded, 'A', g, { slug: '' }).slug, '');
  assert.equal(saveBody(loaded, 'A', g).slug, 'lobby');
});

// The menu as the editor builds it for each selection: Merge reads Unmerge on a merged square, and is off on a
// single square. Only Merge has a chord, Ctrl+M on every platform (macOS keeps Cmd+M for minimising the window).
const key = (k, mods = { ctrlKey: true }) => ({ key: k, ...mods });
const variant = (merge) => [
  { name: 'Merge', label: merge.label, chord: 'm', disabled: merge.disabled, run: () => 'merge' },
  { name: 'Type', label: 'Type: Source', sub: [], run: () => 'type' },
  { name: 'Find', label: 'Find', panel: {}, run: () => 'find' },
];

test('Ctrl+M runs Merge, or Unmerge on a merged square, and nothing else has a chord', () => {
  assert.equal(menuChord(variant({ label: 'Merge' }), key('m'))?.run(), 'merge');
  assert.equal(menuChord(variant({ label: 'Merge' }), key('M', { ctrlKey: true }))?.run(), 'merge');
  assert.equal(menuChord(variant({ label: 'Merge' }), key('m', { metaKey: true })), null, 'Cmd+M is the window');
  assert.equal(menuChord(variant({ label: 'Merge' }), key('m', { ctrlKey: true, metaKey: true })), null);
  assert.equal(menuChord(variant({ label: 'Unmerge' }), key('m'))?.label, 'Unmerge');
  for (const k of ['t', 'f']) assert.equal(menuChord(variant({ label: 'Merge' }), key(k)), null);
  assert.equal(menuChord(variant({ label: 'Merge' }), key('m', {})), null, 'bare M is typing');
});

test('a disabled Merge ignores its chord, and a read-only editor has no menu to run', () => {
  assert.equal(menuChord(variant({ label: 'Merge', disabled: true }), key('m')), null);
  assert.equal(menuChord([], key('m')), null);
});

test('the chord reads Ctrl+M, or the Control sign on a Mac, never the Cmd sign', () => {
  assert.equal(chordText('m', false), 'Ctrl+M');
  assert.equal(chordText('m', true), '\u2303M');
});

test('Ctrl or Cmd+S saves, in a field too; with Shift or Alt it is something else', () => {
  assert.equal(saveChord({ key: 's', ctrlKey: true }), true);
  assert.equal(saveChord({ key: 's', metaKey: true }), true);
  assert.equal(saveChord({ key: 'S', metaKey: true }), true);
  assert.equal(saveChord({ key: 's' }), false);
  assert.equal(saveChord({ key: 's', metaKey: true, shiftKey: true }), false);
  assert.equal(saveChord({ key: 's', ctrlKey: true, altKey: true }), false);
  assert.equal(saveChord({ key: 'd', ctrlKey: true }), false);
  assert.equal(saveChord({ key: 'ы', code: 'KeyS', ctrlKey: true }), true, 'a Cyrillic layout still saves');
});

test('the Type item names the kind of the selected squares', () => {
  assert.equal(typeLabel([{ kind: 'source' }]), 'Type: Source');
  assert.equal(typeLabel([{ kind: 'take' }, { kind: 'take' }]), 'Type: Take');
  assert.equal(typeLabel([{ kind: 'take' }, { kind: 'clear' }]), 'Type: Mixed');
  assert.equal(typeLabel([]), 'Type: Empty');
});

// The editor never sits blank: every way it can fail to reach the panel says why and what to do.
test('each sign-in state says what to do, with Sign in again where signing in is the fix', () => {
  assert.deepEqual(signInNote('sign-in'), { text: 'Sign in with your PIN to edit panels.', button: 'Sign in' });
  assert.deepEqual(signInNote('no-users'), { text: 'Panels need a user, and Sode has none. Add one in Settings, Users.', button: '' });
  assert.equal(signInNote('ended').button, 'Sign in again');
  assert.match(signInNote('ended').text, /ended/);
  const refused = signInNote('refused', 'your PIN changed');
  assert.equal(refused.button, 'Sign in again');
  assert.match(refused.text, /your PIN changed/);
  assert.match(signInNote('offline').text, /Sode is not answering/);
  assert.equal(signInNote('offline').button, '');
  assert.deepEqual(signInNote('ready'), { text: '', button: '' });
  assert.deepEqual(signInNote('wait'), { text: '', button: '' }, 'the page lock draws its own pad');
});

test('a panel that cannot be opened says why and what to do', () => {
  assert.match(openFailNote({ status: 404 }), /No panel here that you can use\. Choose another panel, or ask its owner to share it with you\./);
  assert.match(openFailNote({ status: 0, message: 'Failed to fetch' }), /Sode did not answer/);
  assert.match(openFailNote({ status: 401, message: 'key revoked' }), /sign in again/i);
  assert.equal(openFailNote({ status: 403, message: 'This address may not use the API.' }), 'This address may not use the API.');
  assert.match(openFailNote({ status: 500, message: '' }), /could not be opened \(500\)/);
});
