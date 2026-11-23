import { test } from 'node:test';
import assert from 'node:assert/strict';
import { termRows, termChoices, filtersReq, termChanges } from '../remaptermrules.js';

const items = (o) => Object.entries(o).map(([n, name]) => ({ n: +n, name }));
// Shown numbers start at 1 on both routers.
const target = (sources, destinations = {}) => ({ uid: 'live', offset: 1, sources: items(sources), destinations: items(destinations) });
const router = (sets, sources, destinations = {}) => ({
  uid: 'f',
  offset: 1,
  sets,
  names: { sources: items(sources), destinations: items(destinations) },
});
const set = (name, src, dst = '') => ({ key: `/${name}`, name, src, dst });

const file = { 0: 'CAM 1', 1: 'CAM 2', 2: 'CAM 3', 3: 'VT 1' };

test('a range whose matches differ asks once, with Convert to names preselected', () => {
  const r = router([set('Cams', '1-3'), set('More', '1-3, VT')], file);
  const rows = termRows(r, target({ 0: 'CAM 1', 1: 'VT 9', 2: 'CAM 3', 3: 'VT 1' }));
  assert.equal(rows.length, 1);
  assert.equal(rows[0].label, '1-3');
  assert.equal(rows[0].side, 'src');
  assert.deepEqual(rows[0].sets, ['/Cams', '/More']);
  assert.equal(termChoices(rows)[rows[0].key], 'names');
  assert.deepEqual(filtersReq(r, rows, termChoices(rows)), {
    '/Cams': { src: '"CAM 1", "CAM 2", "CAM 3"' },
    '/More': { src: '"CAM 1", "CAM 2", "CAM 3", VT' },
  });
});

test('a term whose matches agree gets no row; case and trailing spaces are the same name', () => {
  const r = router([set('Cams', '1-3, 4')], file);
  assert.deepEqual(termRows(r, target({ 0: 'cam 1 ', 1: 'CAM 2', 2: 'CAM 3', 3: 'VT 1' })), []);
});

test('name, exact and regex terms import as they are', () => {
  const r = router([set('Cams', 'CAM, "VT 1", /^CAM/')], file);
  assert.deepEqual(termRows(r, target({})), []);
});

test('a number term on destinations, Keep numbers and Skip term', () => {
  const r = router([set('Mons', '', '2, !3')], {}, { 1: 'MON 2', 2: 'MON 3' });
  const rows = termRows(r, target({}, { 1: 'MON B', 2: 'MON C' }));
  assert.deepEqual(
    rows.map((x) => [x.side, x.label]),
    [
      ['dst', '2'],
      ['dst', '3'],
    ],
  );
  const ch = termChoices(rows);
  assert.deepEqual(filtersReq(r, rows, ch), { '/Mons': { dst: '"MON 2", !"MON 3"' } });
  ch[rows[0].key] = 'keep';
  ch[rows[1].key] = 'skip';
  assert.deepEqual(filtersReq(r, rows, ch), { '/Mons': { dst: '2' } });
  ch[rows[1].key] = 'keep';
  assert.deepEqual(filtersReq(r, rows, ch), {});
});

test('the file and target offsets both count: a shown number is n plus the offset', () => {
  const r = { ...router([set('Cams', '1')], { 1: 'CAM 1' }), offset: 0 };
  assert.deepEqual(termRows(r, target({ 0: 'CAM 1' })), []);
  assert.equal(termRows(r, target({ 0: 'VT 1' })).length, 1);
});

test('a term with only blank names in the file cannot convert and keeps its numbers', () => {
  const r = router([set('Cams', '5')], {});
  const rows = termRows(r, target({ 4: 'CAM 5' }));
  assert.equal(rows.length, 1);
  assert.equal(rows[0].canConvert, false);
  assert.equal(termChoices(rows)[rows[0].key], 'keep');
});

test('the review names each changed term and a set that skipping leaves open to everything', () => {
  const r = router([set('Cams', '1-2'), set('Both', '1-2, VT')], file);
  const rows = termRows(r, target({ 0: 'X', 1: 'Y' }));
  const ch = termChoices(rows);
  ch[rows[0].key] = 'skip';
  const out = termChanges(r, rows, ch);
  assert.deepEqual(out.terms, [{ key: rows[0].key, side: 'src', label: '1-2', choice: 'skip', sets: ['Cams', 'Both'] }]);
  assert.deepEqual(out.open, ['Cams']);
});

test('a set whose text does not parse is left alone', () => {
  const r = router([set('Bad', '"CAM')], file);
  assert.deepEqual(termRows(r, target({})), []);
});
