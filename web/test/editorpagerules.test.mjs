// The editor's Pages bar rules (editorpagerules.js) and undo across Pages (gridundo.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { addPage, movePage, renamePage, enablePage, deletePage, leadsTo, copyPage, openClick, OPEN_MAX } from '../editorpagerules.js';
import { PAGE_MAX } from '../panelpages.js';
import { emptyUndo, record, undo, redo, pageSnap, samePages, restorePages, rekeyUndo, rekeySaved } from '../gridundo.js';

const src = (r, c, n) => ({ r, c, rs: 1, cs: 1, kind: 'source', n });
const goTo = (r, c, page) => ({ r, c, rs: 1, cs: 1, kind: 'goToPage', page });
const three = () => [
  { id: 'a', name: 'Cams', enabled: true, cells: [src(1, 1, 1), goTo(1, 2, 'c')] },
  { id: 'b', enabled: true, cells: [] },
  { id: 'c', enabled: false, cells: [src(2, 2, 3)] },
];
const ids = (pages) => pages.map((p) => p.id);

test('Add page appends an enabled empty Page with a new id and stops at 16', () => {
  const p = addPage(three(), 'd');
  assert.deepEqual(p.at(-1), { id: 'd', enabled: true, cells: [] });
  assert.equal(p.length, 4);
  let full = three();
  for (let i = 0; full.length < PAGE_MAX; i++) full = addPage(full, `x${i}`);
  assert.equal(full.length, 16);
  assert.equal(addPage(full, 'y'), full);
  assert.match(addPage(three()).at(-1).id, /^[0-9a-f]{8}$/);
});

test('a move puts the Page at the new place and keeps every id, so Go to Page targets hold', () => {
  assert.deepEqual(ids(movePage(three(), 0, 2)), ['b', 'c', 'a']);
  assert.deepEqual(ids(movePage(three(), 2, 0)), ['c', 'a', 'b']);
  assert.deepEqual(ids(movePage(three(), 1, 1)), ['a', 'b', 'c']);
  // Move up on the first Page and Move down on the last change nothing.
  assert.deepEqual(ids(movePage(three(), 0, -1)), ['a', 'b', 'c']);
  assert.deepEqual(ids(movePage(three(), 2, 3)), ['a', 'b', 'c']);
  assert.equal(movePage(three(), 0, 2)[2].cells[1].page, 'c');
});

test('a rename keeps at most 40 characters, and an empty name drops the name so the bar shows Page N', () => {
  assert.equal(renamePage(three(), 1, '  Studio B  ')[1].name, 'Studio B');
  assert.equal(renamePage(three(), 1, 'x'.repeat(50))[1].name.length, 40);
  assert.equal('name' in renamePage(three(), 0, '   ')[0], false);
});

test('the last enabled Page cannot be turned off, so a reload never lands on a dead Page', () => {
  const one = three();
  const off = enablePage(one, 0, false);
  assert.equal(enablePage(off, 1, false), off);
  assert.equal(enablePage(off, 1, false)[1].enabled, true);
});

test('enable turns a Page on or off and leaves its cells', () => {
  const p = enablePage(three(), 2, true);
  assert.equal(p[2].enabled, true);
  assert.equal(p[2].cells.length, 1);
  assert.equal(enablePage(three(), 0, false)[0].enabled, false);
});

test('Delete page drops the Page, leaves Go to Page cells to it as they are, and never drops the last Page', () => {
  assert.equal(leadsTo(three(), 'c'), 1);
  assert.equal(leadsTo(three(), 'b'), 0);
  const p = deletePage(three(), 2);
  assert.deepEqual(ids(p), ['a', 'b']);
  assert.equal(p[0].cells[1].page, 'c');
  const one = [three()[0]];
  assert.equal(deletePage(one, 0), one);
});

test('Copy to puts a copy of the Page on the target, says what it replaces, and keeps Go to Page targets', () => {
  const into = copyPage(three(), 0, 1);
  assert.equal(into.replaced, 0);
  assert.deepEqual(into.pages[1].cells, three()[0].cells);
  assert.notEqual(into.pages[1].cells[0], into.pages[0].cells[0]);
  assert.equal(into.pages[1].cells[1].page, 'c');
  assert.deepEqual(ids(into.pages), ['a', 'b', 'c']);
  assert.equal(into.pages[1].enabled, true);
  const over = copyPage(three(), 0, 2);
  assert.equal(over.replaced, 1);
  assert.equal(over.pages[2].enabled, false);
  assert.equal(copyPage(three(), 1, 1), null);
});

test('a plain click opens one Page; Cmd or Ctrl-click adds or removes one, up to four, never none', () => {
  assert.equal(OPEN_MAX, 4);
  assert.deepEqual(openClick(['a', 'b'], 'c', false), ['c']);
  assert.deepEqual(openClick(['a'], 'c', true), ['a', 'c']);
  assert.deepEqual(openClick(['a', 'c'], 'c', true), ['a']);
  assert.deepEqual(openClick(['a'], 'a', true), ['a']);
  assert.deepEqual(openClick(['a', 'b', 'c', 'd'], 'e', true), ['a', 'b', 'c', 'd']);
});

test('undo covers a cell edit on any Page and leaves Page names, order and deletions as they are', () => {
  let pages = three();
  let h = emptyUndo();
  // An edit on Page 2 while Page 1 is in view.
  const before = pageSnap(pages, 4, 8);
  pages = pages.map((p) => (p.id === 'b' ? { ...p, cells: [src(1, 1, 9)] } : p));
  assert.equal(samePages(before, pageSnap(pages, 4, 8)), false);
  h = record(h, { what: 'paste', before });
  // Page ops are outside undo.
  pages = renamePage(pages, 1, 'Studio');
  pages = movePage(pages, 1, 0);
  pages = deletePage(pages, 2);
  const u = undo(h, pageSnap(pages, 4, 8));
  pages = restorePages(pages, u.grid);
  assert.deepEqual(ids(pages), ['b', 'a']);
  assert.equal(pages[0].name, 'Studio');
  assert.deepEqual(pages[0].cells, []);
  const r = redo(u.undo, pageSnap(pages, 4, 8));
  pages = restorePages(pages, r.grid);
  assert.deepEqual(pages[0].cells, [src(1, 1, 9)]);
});

test('a selection move or a Page op changes no cells, so it is not an undo step', () => {
  const a = pageSnap(three(), 4, 8);
  assert.equal(samePages(a, pageSnap(three(), 4, 8)), true);
  assert.equal(samePages(a, pageSnap(three(), 4, 7)), false);
  const reordered = movePage(three(), 0, 2);
  assert.equal(samePages(a, pageSnap(reordered, 4, 8)), true);
  // Key order inside a cell does not matter.
  const shuffled = three().map((p, i) => (i === 0 ? { ...p, cells: [{ n: 1, kind: 'source', cs: 1, rs: 1, c: 1, r: 1 }, goTo(1, 2, 'c')] } : p));
  assert.equal(samePages(a, pageSnap(shuffled, 4, 8)), true);
});

test('a template step keeps the whole Page list, so its undo brings back the Pages it replaced', () => {
  const before = pageSnap(three(), 4, 8, { list: true });
  const tpl = [{ id: 't1', enabled: true, cells: [src(1, 1, 7)] }];
  assert.equal(samePages(before, pageSnap(tpl, 2, 2)), false);
  assert.deepEqual(ids(restorePages(tpl, before)), ['a', 'b', 'c']);
  assert.equal(restorePages(tpl, before)[0].name, 'Cams');
});

test('the first save gives an unsaved Page 1 its id, and undo steps follow it', () => {
  let h = record(emptyUndo(), { what: 'paste', before: pageSnap([{ id: '', enabled: true, cells: [src(1, 1, 1)] }], 4, 8) });
  h = rekeyUndo(h, '', 'n1');
  const u = undo(h, pageSnap([{ id: 'n1', enabled: true, cells: [] }], 4, 8));
  assert.deepEqual(restorePages([{ id: 'n1', enabled: true, cells: [] }], u.grid)[0].cells, [src(1, 1, 1)]);
});

test('undo steps follow Page 1 to its saved id even when another Page had focus at the save', () => {
  const unsaved = [{ id: '', enabled: true, cells: [src(1, 1, 1)] }, { id: 'p2', enabled: true, cells: [] }];
  let h = record(emptyUndo(), { what: 'paste', before: pageSnap([{ ...unsaved[0], cells: [] }, unsaved[1]], 4, 8, { focus: '' }) });
  h = rekeySaved(h, ['', 'p2'], ['n1', 'p2']);
  const saved = [{ id: 'n1', enabled: true, cells: [src(1, 1, 1)] }, unsaved[1]];
  const u = undo(h, pageSnap(saved, 4, 8));
  assert.deepEqual(restorePages(saved, u.grid)[0].cells, []);
  assert.equal(u.grid.focus, 'n1');
  assert.equal(rekeySaved(h, ['n1', 'p2'], ['n1', 'p2']), h);
});
