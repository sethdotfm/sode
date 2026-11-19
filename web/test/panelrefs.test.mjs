// Keep in step with internal/app/panelrefs.go.
import test from 'node:test';
import assert from 'node:assert/strict';
import { envFrom, cellsIn } from '../panelrules.js';
import { createState, pressDestination, pressSource, pressPreset, presetState } from '../panelmodel.js';

const U = '2c5bb739e7cf2f2a';
const tally = { levels: [{ level: 1 }], routes: [{ dst: 2, level: 1, src: 3 }] };
const presets = {
  presets: [{ scope: 'everyone', id: 'p1', name: 'Evening', items: [{ src: { router: U, n: 1, name: 'CAM 1' }, dsts: [{ router: U, n: 2 }], levels: [] }] }],
};
const panel = (cells) => ({ rows: 1, cols: 4, cells });

const staged = (cells) => {
  const e = envFrom(tally, {}, presets);
  const [s, d] = cellsIn(panel(cells));
  return pressSource(pressDestination(createState(), d, e, 0), s, e, 0).staged;
};

test('reference cells stage the same route as bare numbers', () => {
  const bare = staged([
    { r: 1, c: 1, kind: 'source', n: 1 },
    { r: 1, c: 2, kind: 'destination', n: 2 },
  ]);
  const refs = staged([
    { r: 1, c: 1, kind: 'source', router: U, n: 1, name: 'CAM 1' },
    { r: 1, c: 2, kind: 'destination', router: U, n: 2, name: 'MON 2' },
  ]);
  assert.deepEqual(refs, bare);
  assert.deepEqual(refs, [{ dst: 2, src: 1, levels: [1] }]);
});

test('a Preset cell finds its preset by id after a rename', () => {
  const e = envFrom(tally, {}, presets);
  const [cell] = cellsIn(panel([{ r: 1, c: 1, kind: 'preset', scope: 'everyone', id: 'p1', name: 'Show' }]));
  const st = pressPreset(createState(), cell, e, 0);
  assert.equal(st.error, null);
  assert.deepEqual(st.staged, [{ dst: 2, src: 1, levels: [1] }]);
  assert.equal(presetState(st, cell, e), 'pending');
});

test('a Preset cell with no id still finds its preset by name', () => {
  const e = envFrom(tally, {}, presets);
  const st = pressPreset(createState(), { kind: 'preset', scope: 'everyone', name: 'Evening' }, e, 0);
  assert.deepEqual(st.staged, [{ dst: 2, src: 1, levels: [1] }]);
});
