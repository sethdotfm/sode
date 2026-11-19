import { test } from 'node:test';
import assert from 'node:assert/strict';
import { remapRows, newChoices, namesReq, remapSummary, stillBroken, routerOptions, rowLabel, usedBy } from '../remaprules.js';

const target = (sources, destinations = []) => ({
  uid: 'live',
  title: 'Studio A',
  sources: Object.entries(sources).map(([n, name]) => ({ n: +n, name })),
  destinations: Object.entries(destinations).map(([n, name]) => ({ n: +n, name })),
});
const ref = (kind, n, name, presets = 1) => ({ key: `f:${kind}:${n}`, kind, n, name, presets });
const router = (...refs) => ({ uid: 'f', title: 'Truck', target: 'live', default: 'live', refs });
const one = (r, t) => remapRows(router(r), t)[0];

test('number and name agree: as is, nothing to choose', () => {
  const row = one(ref('source', 23, 'CAM 3'), target({ 23: 'CAM 3' }));
  assert.equal(row.state, 'as-is');
  assert.deepEqual(namesReq([row], newChoices([row])), {});
});

test('exact match ignores case and trailing spaces and moves automatically', () => {
  for (const name of ['CAM 3', 'cam 3', 'CAM 3  ', 'Cam 3 ']) {
    const row = one(ref('source', 23, name), target({ 23: 'VT 1', 40: 'CAM 3' }));
    assert.equal(row.state, 'moved', name);
    assert.deepEqual(row.preset, [40]);
  }
  const rows = remapRows(router(ref('source', 23, 'CAM 3')), target({ 23: 'VT 1', 40: 'CAM 3' }));
  assert.deepEqual(namesReq(rows, newChoices(rows)), { 'f:source:23': [40] });
});

test('a name the target lacks is Skip, with a fuzzy suggestion never preselected', () => {
  const row = one(ref('source', 23, 'CAM 3'), target({ 23: 'VT 1', 40: 'CAM 03' }));
  assert.equal(row.state, 'missing');
  assert.deepEqual(row.preset, []);
  assert.equal(row.suggestion, 40);
  assert.deepEqual(namesReq([row], newChoices([row])), {});
});

test('duplicated on the target: the file number wins, else Skip with the candidates', () => {
  assert.equal(one(ref('destination', 6, 'MON'), target({}, { 6: 'MON', 7: 'MON' })).state, 'as-is');
  const dst = one(ref('destination', 4, 'MON'), target({}, { 6: 'MON', 7: 'mon ' }));
  assert.equal(dst.state, 'dup-target');
  assert.deepEqual(dst.candidates, [6, 7]);
  assert.deepEqual(dst.preset, []);
  assert.ok(dst.addBoth);
  const src = one(ref('source', 4, 'CAM'), target({ 6: 'CAM', 7: 'CAM' }));
  assert.equal(src.state, 'dup-target');
  assert.ok(!src.addBoth);
});

test('duplicated in the file and one on the target: both move, noted', () => {
  const rows = remapRows(router(ref('source', 1, 'CAM 3'), ref('source', 2, 'cam 3')), target({ 40: 'CAM 3' }));
  for (const r of rows) {
    assert.equal(r.state, 'moved');
    assert.deepEqual(r.preset, [40]);
    assert.match(r.note, /twice in the file/);
  }
});

test('blank file name: as is on a blank target number, else Keep number offered and Skip preselected', () => {
  assert.equal(one(ref('source', 5, ''), target({ 5: '  ' })).state, 'as-is');
  const row = one(ref('source', 5, ' '), target({ 5: 'CAM 5' }));
  assert.equal(row.state, 'blank');
  assert.deepEqual(row.preset, []);
});

test('rows that need a choice sort first, then moved, then as is', () => {
  const rows = remapRows(router(ref('source', 1, 'A'), ref('source', 2, 'B'), ref('source', 3, 'Gone')), target({ 1: 'A', 9: 'B', 3: 'Other' }));
  assert.deepEqual(
    rows.map((r) => r.state),
    ['missing', 'moved', 'as-is'],
  );
});

test('summary counts as is, moved automatically, chosen by hand and skipped', () => {
  const rows = remapRows(
    router(ref('source', 1, 'A'), ref('source', 2, 'B'), ref('source', 3, 'Gone'), ref('source', 4, 'Lost')),
    target({ 1: 'A', 9: 'B', 3: 'Other', 4: 'Other 2' }),
  );
  const ch = newChoices(rows);
  ch['f:source:3'] = [3];
  assert.deepEqual(remapSummary(rows, ch), { asIs: 1, auto: 1, hand: 1, skipped: 1 });
});

test('a mapped reference is no longer Broken in the list', () => {
  const rows = remapRows(router(ref('source', 23, 'CAM 3'), ref('destination', 4, 'MON')), target({ 40: 'CAM 3' }, { 4: 'X' }));
  const broken = [
    { kind: 'source', ref: { router: 'live', n: 23, name: 'CAM 3' } },
    { kind: 'destination', ref: { router: 'live', n: 4, name: 'MON' } },
    { kind: 'level', ref: { router: 'live', n: 9 } },
  ];
  assert.deepEqual(
    stillBroken(broken, { f: rows }, { f: 'live' }, newChoices(rows)).map((m) => m.kind),
    ['destination', 'level'],
  );
});

test('router options show names found, and a target another file router holds is taken', () => {
  const rs = [
    { uid: 'a', title: 'A', candidates: [{ uid: 'live', title: 'Studio A', found: 41, total: 50 }] },
    { uid: 'b', title: 'B', candidates: [{ uid: 'live', title: 'Studio A', found: 3, total: 50 }] },
  ];
  const opts = routerOptions(rs[1], { a: 'live', b: '' });
  assert.deepEqual(opts, [
    { value: '', text: 'None: its items arrive Broken', disabled: false },
    { value: 'live', text: 'Studio A (3 of 50 names found)', disabled: true },
  ]);
  assert.equal(routerOptions(rs[0], { a: 'live', b: '' })[1].disabled, false);
});

test('row text', () => {
  const r = ref('source', 22, 'CAM 3', 50);
  r.panels = 3;
  assert.equal(rowLabel(r, 1), 'CAM 3 (23)');
  assert.equal(rowLabel({ ...r, name: '' }, 1), '23');
  assert.equal(usedBy(r), 'used by 50 presets, 3 panels');
  assert.equal(usedBy({ presets: 1, users: 2 }), 'used by 1 preset, 2 users');
});
