// Page Name and Info cells (pagecells.js): what they show and their defaults.
import test from 'node:test';
import assert from 'node:assert/strict';
import { showOf, pageCellText, SHOW_PARTS } from '../pagecells.js';

const pages = [
  { id: 'a', name: 'Studio', cells: [] },
  { id: 'b', name: '', cells: [] },
];
const ctx = { panelName: 'Desk', instance: 'Left', pages };

test('defaults: Page Name shows the name only, Info all but the Page number', () => {
  assert.deepEqual(showOf({ kind: 'pageName' }), ['name']);
  assert.deepEqual(showOf({ kind: 'info' }), ['panel', 'page', 'instance']);
  assert.deepEqual(SHOW_PARTS.info, ['panel', 'number', 'page', 'instance']);
});

test('a stored show list wins, unknown parts dropped, order fixed', () => {
  assert.deepEqual(showOf({ kind: 'pageName', show: ['name', 'number'] }), ['number', 'name']);
  assert.deepEqual(showOf({ kind: 'info', show: ['bogus'] }), []);
});

test('Page Name shows the Page it sits on', () => {
  assert.equal(pageCellText({ kind: 'pageName' }, { ...ctx, page: 'a' }).value, 'Studio');
  assert.equal(pageCellText({ kind: 'pageName' }, { ...ctx, page: 'b' }).value, 'Page 2');
  assert.equal(pageCellText({ kind: 'pageName', show: ['number', 'name'] }, { ...ctx, page: 'a' }).value, 'Page 1, Studio');
  assert.equal(pageCellText({ kind: 'pageName', show: ['number'] }, { ...ctx, page: 'b' }).value, 'Page 2');
});

test('Info joins its parts, one per line', () => {
  assert.deepEqual(pageCellText({ kind: 'info' }, { ...ctx, page: 'a' }).lines, ['Desk', 'Studio', 'Left']);
  assert.deepEqual(pageCellText({ kind: 'info', show: ['number', 'page'] }, { ...ctx, page: 'b' }).lines, ['Page 2']);
  assert.deepEqual(pageCellText({ kind: 'info', show: ['panel', 'number'] }, { ...ctx, page: 'b' }).lines, ['Desk', 'Page 2']);
});

test('no instance yet shows the kind words; the caption rides above', () => {
  const t = pageCellText({ kind: 'info', caption: 'Here' }, { pages, page: 'a' });
  assert.deepEqual(t.lines, ['Panel Name', 'Studio', 'Instance Name']);
  assert.equal(t.caption, 'Here');
});
