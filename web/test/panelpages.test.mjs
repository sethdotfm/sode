// A panel's Pages as the web reads and saves them (panelpages.js), and every place that reads a stored panel's cells.
// Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { pagesOf, pageCells, pageView, pageLabel, newPageId, fitCells, fitPages, storedPages, PAGE_MAX, PAGE_NAME_MAX } from '../panelpages.js';
import { cellsIn, drawnCells } from '../panelrules.js';
import { thumbCells } from '../panelsrules.js';
import { createGrid, select, setKind } from '../gridmodel.js';
import { saveBody, isDirty, newPanel } from '../editorrules.js';
import { saveTemplateBody, templateGrid } from '../templaterules.js';

const take = (r, c, more = {}) => ({ r, c, rs: 1, cs: 1, kind: 'take', ...more });
const stored = {
  id: 'p-1',
  name: 'Wall',
  rows: 2,
  cols: 2,
  settings: {},
  pages: [
    { id: 'aa', name: 'Cams', enabled: true, cells: [take(1, 1)] },
    { id: 'bb', enabled: false, cells: [take(2, 2), { r: 1, c: 1, rs: 1, cs: 1, kind: 'destination', n: 4 }] },
  ],
};

test('a panel from before Pages reads as one enabled Page; one with Pages reads them in order', () => {
  assert.deepEqual(pagesOf({ cells: [take(1, 1)] }), [{ id: '', enabled: true, cells: [take(1, 1)] }]);
  assert.deepEqual(pagesOf({}), [{ id: '', enabled: true, cells: [] }]);
  assert.deepEqual(
    pagesOf(stored).map((p) => p.id),
    ['aa', 'bb'],
  );
  assert.deepEqual(pageCells(stored), [take(1, 1)]);
  assert.equal(pageCells(stored, 1).length, 2);
  assert.deepEqual(pageView(stored).cells, [take(1, 1)]);
  assert.equal(PAGE_MAX, 16);
  assert.equal(PAGE_NAME_MAX, 40);
});

test('a Page with no name reads as Page N', () => {
  assert.equal(pageLabel(stored.pages[0], 0), 'Cams');
  assert.equal(pageLabel(stored.pages[1], 1), 'Page 2');
  assert.equal(pageLabel({ name: '  ' }, 4), 'Page 5');
});

test('a new Page id is 8 hex digits and differs each time', () => {
  const a = newPageId();
  assert.match(a, /^[0-9a-f]{8}$/);
  assert.notEqual(a, newPageId());
});

test('a stored panel with only pages draws Page 1 on the panel page, the editor, the thumbnails and templates', () => {
  assert.deepEqual(
    cellsIn(stored).map((c) => c.kind),
    ['take'],
  );
  assert.equal(drawnCells(stored).length, 1);
  assert.equal(thumbCells(stored).length, 1);
  assert.equal(createGrid(pageView(stored)).cells.length, 1);
  assert.equal(templateGrid({ rows: 2, cols: 2, pages: stored.pages }).cells.length, 1);
});

test('a resize drops cells outside on every Page and clips a span that reaches out', () => {
  assert.deepEqual(fitCells([take(1, 1, { rs: 2, cs: 3 }), take(3, 1), take(1, 2)], 2, 2), [take(1, 1, { rs: 2, cs: 2 }), take(1, 2)]);
  const fit = fitPages(stored.pages, 1, 1);
  assert.equal(fit.lost, 1);
  assert.deepEqual(
    fit.pages.map((p) => [p.id, p.cells.length]),
    [
      ['aa', 1],
      ['bb', 1],
    ],
  );
  assert.equal(fitPages(stored.pages, 2, 2).lost, 0);
  const out = storedPages(stored.pages, [take(1, 1)], 1, 1);
  assert.deepEqual(
    out.map((p) => [p.id, p.name, p.enabled, p.cells.length]),
    [
      ['aa', 'Cams', true, 1],
      ['bb', undefined, false, 1],
    ],
  );
});

test('a save writes pages and no cells: Page 1 from the grid, the other Pages as loaded and fitted', () => {
  let g = createGrid(pageView(stored));
  g = setKind(select(g, 1, 2), 'clear');
  const b = saveBody(stored, 'Wall', g);
  assert.equal('cells' in b, false);
  assert.deepEqual(
    b.pages.map((p) => [p.id, p.cells.map((c) => c.kind).join()]),
    [
      ['aa', 'take,clear'],
      ['bb', 'take,destination'],
    ],
  );
  const small = saveBody(stored, 'Wall', createGrid({ rows: 1, cols: 1, cells: [take(1, 1)] }));
  assert.deepEqual(
    small.pages.map((p) => p.cells.length),
    [1, 1],
  );
  const fresh = saveBody(newPanel(), 'New', createGrid());
  assert.equal(fresh.pages.length, 1);
  assert.equal('cells' in fresh, false);
});

test('the editor reads the pages it is given, so an applied template counts as a change', () => {
  const g = createGrid(pageView(stored));
  assert.equal(isDirty(stored, 'Wall', g), false);
  assert.equal(isDirty(stored, 'Wall', g, {}, stored.pages), false);
  // A Page the editor once showed comes back in reading order with its keys sorted, which is no change.
  const shown = [stored.pages[0], { ...stored.pages[1], cells: [...stored.pages[1].cells].reverse().map((c) => ({ ...c })) }];
  assert.equal(isDirty(stored, 'Wall', g, {}, shown), false);
  const other = [stored.pages[0], { ...stored.pages[1], enabled: true }];
  assert.equal(isDirty(stored, 'Wall', g, {}, other), true);
  assert.deepEqual(
    saveBody(stored, 'Wall', g, {}, other).pages.map((p) => p.enabled),
    [true, true],
  );
});

test('a template carries every Page with its id', () => {
  const g = createGrid(pageView(stored));
  const b = saveTemplateBody({ name: 'Show', scope: 'me', grid: g, pages: stored.pages });
  assert.equal('cells' in b, false);
  assert.deepEqual(
    b.pages.map((p) => p.id),
    ['aa', 'bb'],
  );
  assert.equal(saveTemplateBody({ name: 'Show', scope: 'me', grid: g }).pages.length, 1);
});
