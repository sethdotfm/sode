// Page rules on the panel page (panelnav.js, panelmodel.js): the current Page, Go to Page, Next Page and Previous
// Page with and without wrap, disabled and deleted targets, the plan across Pages and the fallback to the first
// enabled Page. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { landingPage, pageIndex, stepPage, goToState, navTarget, navText, wrapOf, targetChoices, targetWarning } from '../panelnav.js';
import { createState, pressDestination, pressSource, pressPage, showPage, clear, startTake, carryView } from '../panelmodel.js';
import { cellsIn, drawnCells } from '../panelrules.js';

const P = (id, more = {}) => ({ id, enabled: true, cells: [], ...more });
const pages = [P('a', { name: 'Cams' }), P('b', { cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'take' }] }), P('c', { enabled: false }), P('d')];
const go = (page, more = {}) => ({ r: 1, c: 1, rs: 1, cs: 1, kind: 'goToPage', page, ...more });
const next = { r: 1, c: 2, rs: 1, cs: 1, kind: 'nextPage' };
const prev = { r: 1, c: 3, rs: 1, cs: 1, kind: 'previousPage' };

test('the saved Page opens when it is there and enabled; else the first enabled Page', () => {
  assert.equal(landingPage(pages, 'd'), 'd');
  assert.equal(landingPage(pages, 'c'), 'a'); // disabled
  assert.equal(landingPage(pages, 'gone'), 'a'); // deleted
  assert.equal(landingPage(pages, ''), 'a');
  assert.equal(landingPage([P('a', { enabled: false }), P('b')], 'a'), 'b');
  // Every Page disabled: Page 1, so the panel still shows a grid.
  assert.equal(landingPage([P('a', { enabled: false }), P('b', { enabled: false })], 'b'), 'a');
  assert.equal(pageIndex(pages, 'd'), 3);
  assert.equal(pageIndex(pages, 'gone'), 0);
});

test('Next and Previous skip disabled Pages and wrap when the setting is on', () => {
  assert.equal(stepPage(pages, 'b', 1, true), 'd');
  assert.equal(stepPage(pages, 'd', -1, true), 'b');
  assert.equal(stepPage(pages, 'd', 1, true), 'a');
  assert.equal(stepPage(pages, 'a', -1, true), 'd');
});

test('without wrap Next and Previous stop at the ends', () => {
  assert.equal(stepPage(pages, 'd', 1, false), null);
  assert.equal(stepPage(pages, 'a', -1, false), null);
  assert.equal(stepPage(pages, 'a', 1, false), 'b');
  // The last enabled Page is an end even when a disabled one follows it.
  assert.equal(stepPage([P('a'), P('b'), P('c', { enabled: false })], 'b', 1, false), null);
});

test('with one enabled Page Next and Previous have nowhere to go, wrap or not', () => {
  const one = [P('a'), P('b', { enabled: false })];
  assert.equal(stepPage(one, 'a', 1, true), null);
  assert.equal(stepPage(one, 'a', -1, true), null);
});

test('a Go to Page cell is unassigned, missing, disabled, empty or ok', () => {
  assert.equal(goToState(pages, go('')), 'unassigned');
  assert.equal(goToState(pages, { ...go('a'), page: undefined }), 'unassigned');
  assert.equal(goToState(pages, go('gone')), 'missing');
  assert.equal(goToState(pages, go('c')), 'disabled');
  assert.equal(goToState(pages, go('a')), 'empty');
  assert.equal(goToState(pages, go('b')), 'ok');
});

test('a Go to Page cell to an enabled Page with no cells works; to a disabled or deleted one it goes nowhere', () => {
  assert.equal(navTarget(pages, 'b', go('a'), true), 'a');
  assert.equal(navTarget(pages, 'a', go('c'), true), null);
  assert.equal(navTarget(pages, 'a', go('gone'), true), null);
  assert.equal(navTarget(pages, 'a', next, false), 'b');
  assert.equal(navTarget(pages, 'd', next, false), null);
  assert.equal(navTarget(pages, 'd', prev, false), 'b');
});

test('a Go to Page cell shows its caption, else the Page name, else Page N', () => {
  assert.equal(navText(go('a'), pages), 'Cams');
  assert.equal(navText(go('d'), pages), 'Page 4');
  assert.equal(navText(go('d', { caption: 'Studio' }), pages), 'Studio');
  assert.equal(navText(go('gone'), pages), 'Go to Page');
  assert.equal(navText(next, pages), 'Next Page');
  assert.equal(navText(prev, pages), 'Previous Page');
});

test('wrap between Pages is on unless the panel turns it off', () => {
  assert.equal(wrapOf({}), true);
  assert.equal(wrapOf({ settings: {} }), true);
  assert.equal(wrapOf({ settings: { wrapPages: false } }), false);
});

const env = { levels: [1], tally: new Map(), protectedDsts: new Set() };
const D = (n) => ({ kind: 'destination', n });
const S = (n) => ({ kind: 'source', n });

test('the plan is panel-wide: a Page change keeps it, and Clear and Take act on all of it', () => {
  let st = showPage(createState(), 'a');
  st = pressDestination(st, D(3));
  st = pressPage(st, next, pages, true);
  assert.equal(st.page, 'b');
  st = pressSource(st, S(7), env, 0);
  assert.deepEqual(st.staged, [{ dst: 3, src: 7, levels: [1] }]);
  st = pressPage(st, prev, pages, true);
  assert.equal(st.page, 'a');
  assert.deepEqual(startTake(st, env).routes, [{ src: 7, dst: 3, levels: [1] }]);
  const cleared = clear(st);
  assert.deepEqual(cleared.staged, []);
  assert.equal(cleared.page, 'a');
});

test('a press that goes nowhere leaves the state as it was', () => {
  const st = showPage(createState(), 'd');
  assert.equal(pressPage(st, next, pages, false), st);
  assert.equal(pressPage(st, go('c'), pages, true), st);
});

test('a live update keeps a Destination viewed on another Page', () => {
  const panel = { rows: 1, cols: 2, pages: [P('a', { cells: [go('b')] }), P('b', { cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'destination', n: 5 }] })] };
  const every = panel.pages.flatMap((_, i) => cellsIn(panel, i));
  const prevSt = pressDestination(showPage(createState(), 'a'), D(5));
  assert.deepEqual(carryView(createState(), prevSt, every).view, { dst: 5, levels: null });
});

test('the editor lists every Page by number and name, and warns about a target that is gone, off or empty', () => {
  const list = [P('a', { name: 'Cams' }), P('b', { enabled: false }), P('', {})];
  assert.deepEqual(targetChoices(list), [
    { value: 'a', text: 'Page 1, Cams', disabled: false },
    { value: 'b', text: 'Page 2 (disabled)', disabled: false },
    { value: '', text: 'Page 3 (save the panel first)', disabled: true },
  ]);
  const three = [P('a'), P('b'), P('c', { enabled: false })];
  assert.match(targetWarning(three, go('gone'), []), /deleted/);
  assert.match(targetWarning(three, go('c'), []), /^Page 3 is disabled/);
  assert.match(targetWarning(three, go('b'), []), /^Page 2 has no cells yet/);
  // Page 1's cells are the grid's, not the list's.
  assert.match(targetWarning(three, go('a'), []), /^Page 1 has no cells/);
  assert.equal(targetWarning(three, go('a'), [next]), '');
  assert.equal(targetWarning(three, go(''), []), '');
});

test('the panel page draws the current Page; Go to Page cells with a target are drawn, those without are not', () => {
  const panel = { rows: 1, cols: 3, pages: [P('a', { cells: [go('b'), { ...go(''), c: 2 }, { ...go('gone'), c: 3 }] }), P('b', { cells: [next] })] };
  assert.deepEqual(
    drawnCells(panel, 0).map((c) => c.page),
    ['b', 'gone'],
  );
  assert.deepEqual(
    drawnCells(panel, 1).map((c) => c.kind),
    ['nextPage'],
  );
});
