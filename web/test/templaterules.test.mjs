// Templates in the panel editor (templaterules.js): what a save sends, what an apply does to the grid, and how the
// list is grouped. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { saveTemplateBody, templateGrid, groupTemplates, templateNote, pinNeeded } from '../templaterules.js';
import { createGrid, select, setKind, countUnassigned, saved } from '../gridmodel.js';

const grid = () => {
  let g = createGrid({ rows: 2, cols: 3 });
  g = setKind(select(g, 1, 1), 'source');
  g = setKind(select(g, 1, 2), 'destination');
  g = setKind(select(g, 2, 1), 'take');
  return g;
};

test('a save sends the layout and, by default, neither settings nor contents', () => {
  const b = saveTemplateBody({ name: '  Show  ', scope: 'me', grid: grid(), settings: { mode: 'instant' } });
  assert.equal(b.name, 'Show');
  assert.equal(b.scope, 'me');
  assert.deepEqual([b.sources, b.dests, 'contents' in b], [false, false, false]);
  assert.equal('settings' in b, false);
  assert.equal(b.rows, 2);
  assert.equal(b.cells.length, 3);
});

test('settings, Sources and Destinations go only when asked for, each on its own', () => {
  const b = saveTemplateBody({
    name: 'Show',
    scope: 'everyone',
    grid: grid(),
    settings: { mode: 'instant' },
    withSettings: true,
    withSources: true,
    withDests: true,
    pin: '1234',
  });
  assert.deepEqual(b.settings, { mode: 'instant' });
  assert.deepEqual([b.sources, b.dests], [true, true]);
  assert.equal(b.pin, '1234');
});

test('Sources and Destinations are separate flags', () => {
  const only = (o) => saveTemplateBody({ name: 'a', scope: 'me', grid: grid(), ...o });
  assert.deepEqual([only({ withSources: true }).sources, only({ withSources: true }).dests], [true, false]);
  assert.deepEqual([only({ withDests: true }).sources, only({ withDests: true }).dests], [false, true]);
});

test('no PIN is sent for a Me template, nor an empty one', () => {
  assert.equal('pin' in saveTemplateBody({ name: 'a', scope: 'me', grid: grid(), pin: '1234' }), false);
  assert.equal('pin' in saveTemplateBody({ name: 'a', scope: 'everyone', grid: grid(), pin: '' }), false);
});

test('an applied template brings its cells back unassigned', () => {
  const stored = { rows: 2, cols: 3, cells: saved(grid()).cells };
  const g = templateGrid(stored);
  assert.deepEqual([g.rows, g.cols], [2, 3]);
  assert.equal(g.cells.length, 3);
  assert.equal(countUnassigned(g), 2);
  assert.equal(g.sel, null);
});

test('a template with contents keeps them', () => {
  const g = templateGrid({ rows: 1, cols: 2, sources: true, dests: true, cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n: 4 }] });
  assert.equal(g.cells[0].n, 4);
  assert.equal(countUnassigned(g), 0);
});

test('the list groups Me and Everyone, by name', () => {
  const { me, everyone } = groupTemplates([
    { id: 't-1', name: 'b', scope: 'everyone' },
    { id: 't-2', name: 'Z', scope: 'me' },
    { id: 't-3', name: 'a', scope: 'me' },
  ]);
  assert.deepEqual(
    me.map((t) => t.id),
    ['t-3', 't-2'],
  );
  assert.deepEqual(
    everyone.map((t) => t.id),
    ['t-1'],
  );
});

test('the note says what a template carries', () => {
  assert.equal(templateNote({ rows: 4, cols: 8 }), '4 x 8, cells unassigned');
  assert.equal(templateNote({ rows: 2, cols: 3, sources: true, dests: true, settings: {} }), '2 x 3, with settings, with Sources and Destinations');
  assert.equal(templateNote({ rows: 2, cols: 3, sources: true }), '2 x 3, with Sources, Destinations unassigned');
  assert.equal(templateNote({ rows: 2, cols: 3, dests: true }), '2 x 3, with Destinations, Sources unassigned');
  assert.equal(templateNote({ rows: 2, cols: 3, settings: {} }), '2 x 3, with settings, cells unassigned');
});

test('a PIN field is asked for only after the server refuses Everyone', () => {
  assert.equal(pinNeeded(403, 'trusted_required'), true);
  assert.equal(pinNeeded(403, 'scope'), false);
  assert.equal(pinNeeded(409, 'conflict'), false);
});
