// What the panel editor reads from its URL and from /api/v1/panels (editorrules.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { editorTarget, groupPanels, newPanel, saveBody, isDirty, placeMenu, kindLabel, kindChoices, menuShortcut, menuLetters, menuTarget, todoText } from '../editorrules.js';
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
  assert.deepEqual([p.rows, p.cols, p.cells, p.acts, p.share.mode], [4, 8, [], 'session', 'me']);
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
  assert.deepEqual([b.rows, b.cols, b.cells.length], [2, 2, 1]);
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

test('the menu goes right of the selection, else left, else below, and stays on screen', () => {
  const vp = { w: 1000, h: 600 };
  const menu = { w: 200, h: 120 };
  assert.deepEqual(placeMenu({ left: 100, top: 50, right: 200, bottom: 100 }, menu, vp, 8), { side: 'right', left: 208, top: 50 });
  assert.deepEqual(placeMenu({ left: 700, top: 50, right: 900, bottom: 100 }, menu, vp, 8), { side: 'left', left: 492, top: 50 });
  assert.deepEqual(placeMenu({ left: 10, top: 50, right: 990, bottom: 100 }, menu, vp, 8), { side: 'below', left: 10, top: 108 });
  assert.equal(placeMenu({ left: 100, top: 550, right: 200, bottom: 590 }, menu, vp, 8).top, 480);
});

test('Type offers every kind the panel page knows, then Empty', () => {
  const c = kindChoices();
  assert.deepEqual(c.map((x) => x.kind).slice(-1), ['']);
  assert.ok(c.some((x) => x.kind === 'take'));
  assert.equal(kindLabel('destination'), 'Destination');
  assert.equal(kindLabel(''), 'Empty');
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

test('Alt with a letter names a menu letter by key position, so Option on a Mac works too', () => {
  const k = (code, extra = {}) => menuShortcut({ code, altKey: true, ctrlKey: false, metaKey: false, ...extra });
  assert.equal(k('KeyM'), 'm');
  assert.equal(k('KeyF'), 'f');
  assert.equal(k('Digit1'), null);
  assert.equal(k('KeyM', { altKey: false }), null);
  assert.equal(k('KeyM', { ctrlKey: true }), null);
  assert.equal(k('KeyM', { metaKey: true }), null);
});

test('each menu item gets the first letter of its name that no earlier item took', () => {
  assert.deepEqual(menuLetters(['Merge', 'Type', 'Find']), ['m', 't', 'f']);
  assert.deepEqual(menuLetters(['Type', 'Tag', 'Tab']), ['t', 'a', 'b']);
  // A name with every letter taken falls back to the first free letter of the alphabet, in order.
  assert.deepEqual(menuLetters(['Aa', 'A', 'A']), ['a', 'b', 'c']);
  assert.deepEqual(menuLetters(['Merge', 'Type', 'Find']), menuLetters(['Merge', 'Type', 'Find']));
});

// The menu as the editor builds it for each selection: Merge reads Unmerge on a merged square, and is off on a
// single square. Find and Type are on whenever a square is selected.
const variant = (merge) => [
  { name: 'Merge', label: merge.label, disabled: merge.disabled, run: () => 'merge' },
  { name: 'Type', label: 'Type', sub: [], run: () => 'type' },
  { name: 'Find', label: 'Find', panel: {}, run: () => 'find' },
];
const VARIANTS = {
  'single square': variant({ label: 'Merge', disabled: true }),
  group: variant({ label: 'Merge', disabled: false }),
  merged: variant({ label: 'Unmerge', disabled: false }),
};

test('in every menu variant the letters are distinct and a letter finds its item', () => {
  for (const [what, items] of Object.entries(VARIANTS)) {
    const letters = menuLetters(items.map((i) => i.name));
    assert.equal(new Set(letters).size, letters.length, what);
    items.forEach((it, i) => (it.letter = letters[i]));
    assert.equal(menuTarget(items, 't')?.run(), 'type', what);
    assert.equal(menuTarget(items, 'f')?.run(), 'find', what);
    assert.equal(menuTarget(items, 'x'), null, what);
  }
  assert.equal(menuTarget(VARIANTS.group, 'm')?.run(), 'merge');
  assert.equal(menuTarget(VARIANTS.merged, 'm')?.label, 'Unmerge');
});

test('a disabled item ignores its letter', () => {
  assert.equal(menuTarget(VARIANTS['single square'], 'm'), null);
});

test('a read-only editor has no menu, so no letter finds anything', () => {
  assert.equal(menuTarget([], 'm'), null);
});

test('the unassigned count says what it counts', () => {
  assert.equal(todoText(0), 'No unassigned squares');
  assert.equal(todoText(1), '1 unassigned square');
  assert.equal(todoText(3), '3 unassigned squares');
});
