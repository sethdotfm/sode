// Default artwork: how a cell resolves its own artwork and its item's default (panellook.js), and how an open
// panel notices a changed default (panelupdate.js revOf).
import test from 'node:test';
import assert from 'node:assert/strict';
import { withDefault, defaultsBy, defaultFor, itemKey, artOf, artBOf, artBBadge, textHidden, artUrlPath, nameMismatch } from '../panellook.js';
import { revOf, noticeRev, createUpdate } from '../panelupdate.js';
import { defaultNote, defaultBody, saveBody } from '../editorrules.js';
import { createGrid, setCell } from '../gridmodel.js';

const defs = defaultsBy([
  { kind: 'source', n: 3, name: 'CAM 3', art: 'icon:camera', artB: '2', hideText: true },
  { kind: 'destination', n: 0, name: 'MON 0', art: 'd-0123abcd' },
  { kind: 'preset', scope: 'everyone', name: 'Show', artB: 'S' },
]);

test('the item a cell shows is its default key, and an unassigned cell shows none', () => {
  assert.equal(itemKey({ kind: 'source', n: 3 }), 'source/3');
  assert.equal(itemKey({ kind: 'destination', n: 0 }), 'destination/0');
  assert.equal(itemKey({ kind: 'preset', scope: 'everyone', name: 'Show' }), 'preset/everyone/Show');
  assert.equal(itemKey({ kind: 'source' }), '');
  assert.equal(itemKey({ kind: 'take' }), '');
  assert.equal(defaultFor({ kind: 'source', n: 4 }, defs), null);
});

test('a cell with no artwork of its own takes the default, Hide text included', () => {
  const c = withDefault({ kind: 'source', n: 3, r: 1, c: 1 }, defs);
  assert.equal(c.art, 'icon:camera');
  assert.equal(c.artB, '2');
  assert.equal(textHidden(c), true);
  assert.equal(c.r, 1);
});

test("a cell's own artwork wins whole: neither icon of the default fills in", () => {
  const c = withDefault({ kind: 'source', n: 3, artB: '7' }, defs);
  assert.equal(artOf(c), null);
  assert.equal(artBOf(c), '7');
  const d = withDefault({ kind: 'source', n: 3, art: 'icon:mic' }, defs);
  assert.deepEqual(artOf(d), { icon: 'mic' });
  assert.equal(artBOf(d), '');
});

test('Hide text comes from the default unless the cell sets its own, on or off', () => {
  assert.equal(textHidden(withDefault({ kind: 'source', n: 3, art: 'icon:mic' }, defs)), true);
  assert.equal(textHidden(withDefault({ kind: 'source', n: 3, hideText: false }, defs)), false);
  assert.equal(textHidden(withDefault({ kind: 'source', n: 3, art: 'icon:mic', hideText: false }, defs)), false);
  assert.equal(textHidden(withDefault({ kind: 'preset', scope: 'everyone', name: 'Show', hideText: true }, defs)), true);
});

test('a cell that loses its item shows no default, and keeps its own artwork', () => {
  assert.equal(artOf(withDefault({ kind: 'source' }, defs)), null);
  assert.equal(textHidden(withDefault({ kind: 'source' }, defs)), false);
  assert.deepEqual(artOf(withDefault({ kind: 'source', art: 'icon:mic' }, defs)), { icon: 'mic' });
});

test('a library icon as a default stays a reference: never .icon, so Icon B is a badge, fetched from the library', () => {
  const lib = defaultsBy([{ kind: 'source', n: 5, art: 'lib:0123abcd', artB: '4' }]);
  const c = withDefault({ kind: 'source', n: 5 }, lib);
  assert.equal(c.art, 'lib:0123abcd');
  assert.deepEqual(artOf(c), { id: 'lib:0123abcd', lib: true });
  assert.equal(artBBadge(c), true);
  assert.equal(artUrlPath('p-11111111', c.art), 'api/v1/icon-library/0123abcd');
  assert.equal(defaultBody({ kind: 'source', n: 5, art: 'lib:0123abcd' }, 'p-11111111', true).art, 'lib:0123abcd');
});

test('a default image is an image to artNode: never .icon, so Icon B is a badge, fetched from its own route', () => {
  const c = withDefault({ kind: 'destination', n: 0 }, defs);
  assert.deepEqual(artOf(c), { id: 'd-0123abcd' });
  assert.equal(artBBadge({ ...c, artB: '3' }), true);
  assert.equal(artUrlPath('p-11111111', 'd-0123abcd'), 'api/v1/default-artwork/images/d-0123abcd');
  assert.equal(artUrlPath('p-11111111', 'a-0123abcd'), 'api/v1/panels/p-11111111/artwork/a-0123abcd');
});

test('Hide text set off against a default is saved as false, and Use default removes it', () => {
  let g = createGrid({ rows: 1, cols: 1, cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n: 3 }] });
  g = setCell(g, { r: 1, c: 1 }, { hideText: false });
  assert.equal(saveBody({ id: 'p-1', canEdit: true }, 'Wall', g).cells[0].hideText, false);
  g = setCell(g, { r: 1, c: 1 }, { art: undefined, artB: undefined, hideText: undefined });
  assert.equal('hideText' in saveBody({ id: 'p-1' }, 'Wall', g).cells[0], false);
});

test('the stored cell is never changed', () => {
  const cell = { kind: 'source', n: 3 };
  withDefault(cell, defs);
  assert.deepEqual(cell, { kind: 'source', n: 3 });
});

test('a Source or Destination default set under another router name is flagged', () => {
  const d = defaultFor({ kind: 'source', n: 3 }, defs);
  assert.equal(nameMismatch(d, 'CAM 3'), false);
  assert.equal(nameMismatch(d, 'CAM 9'), true);
  assert.equal(nameMismatch(d, ''), false); // names not read yet
  assert.equal(nameMismatch(defaultFor({ kind: 'preset', scope: 'everyone', name: 'Show' }, defs), 'Other'), false);
});

test('an open panel sees a changed default as an update with no panel save', () => {
  const p = { rev: 4, artDefaults: [{ kind: 'source', n: 3, art: 'icon:camera' }] };
  const q = { rev: 4, artDefaults: [{ kind: 'source', n: 3, art: 'icon:mic' }] };
  assert.notEqual(revOf(p), revOf(q));
  assert.equal(revOf(p), revOf({ ...p }));
  assert.equal(revOf({}), undefined);
  const u = noticeRev(createUpdate(revOf(p)), revOf(q), 1000);
  assert.equal(u.since, 1000);
  assert.equal(noticeRev(createUpdate(revOf(p)), revOf(p), 1000).since, 0);
});

test("the Cell tab says whether the cell shows its item's default, and warns when the router name moved", () => {
  const d = defaultFor({ kind: 'source', n: 3 }, defs);
  assert.deepEqual(defaultNote(null, false, 'CAM 3'), { text: '', warn: '' });
  assert.match(defaultNote(d, false, 'CAM 3').text, /shows the Default artwork of its source/);
  assert.match(defaultNote(d, true, 'CAM 3').text, /own artwork shows instead/);
  assert.equal(defaultNote(d, false, 'CAM 3').warn, '');
  assert.match(defaultNote(d, false, 'CAM 9').warn, /default may be for another item: it was set for CAM 3, and the router now names this source CAM 9/);
});

test("Set as default sends the cell's item, its own artwork and Hide text, and the panel its image is on", () => {
  const cell = { kind: 'source', n: 3, art: 'a-0123abcd', artB: '2', hideText: true, r: 1, c: 1 };
  assert.deepEqual(defaultBody(cell, 'p-1', true), { kind: 'source', n: 3, art: 'a-0123abcd', artB: '2', hideText: true, panel: 'p-1' });
  const preset = { kind: 'preset', scope: 'me', name: 'Show', art: 'icon:mic' };
  assert.deepEqual(defaultBody(preset, 'p-1', true), { kind: 'preset', scope: 'me', name: 'Show', art: 'icon:mic', artB: '', hideText: false, panel: 'p-1' });
  assert.deepEqual(defaultBody({ kind: 'destination', n: 0, art: 'icon:tv' }, 'p-1', false), { kind: 'destination', n: 0 });
});
