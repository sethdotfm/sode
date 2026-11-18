// Jumps on a Grid panel (gridpaneljump.js): g to the current source, G through the destinations that have a source.
import test from 'node:test';
import assert from 'node:assert/strict';
import { sourceJump, holderJump, jumpText } from '../gridpaneljump.js';
import { envFrom, namesFrom } from '../panelrules.js';
import { namesReply, tallyReply } from './gridpanelfixture.mjs';

const envOf = (routes) => envFrom(tallyReply({ routes }), {});
// Display order differs from router order on purpose.
const axes = { srcs: [2, 0, 1], dsts: [3, 1, 0, 2] };

test('g on a crosspoint goes along its row to the source on the lowest level in the box', () => {
  const env = envOf([
    [1, 1, 2],
    [1, 2, 0],
  ]);
  assert.deepEqual(sourceJump({ i: 1, j: 0 }, axes, env, [3, 2]), { c: { i: 1, j: 1 } });
  assert.deepEqual(sourceJump({ i: 1, j: 2 }, axes, env, [1, 2]), { c: { i: 1, j: 0 } });
});

test('g on the Destination bar goes onto the crosspoint of the current source', () => {
  assert.deepEqual(sourceJump({ i: 2, j: -1 }, axes, envOf([[0, 1, 1]]), [1]), { c: { i: 2, j: 2 } });
});

test('g on the Source bar or the corner does nothing', () => {
  assert.equal(sourceJump({ i: -1, j: 1 }, axes, envOf([]), [1]), null);
  assert.equal(sourceJump({ i: -1, j: -1 }, axes, envOf([]), [1]), null);
});

test('g says why when it cannot go', () => {
  assert.deepEqual(sourceJump({ i: 0, j: 0 }, axes, envOf([]), []), { why: 'levels' });
  assert.deepEqual(sourceJump({ i: 0, j: 0 }, axes, envOf([[3, 2, 0]]), [1]), { why: 'none', dst: 3, level: 1 });
  assert.deepEqual(sourceJump({ i: 0, j: 0 }, axes, envOf([[3, 1, -1]]), [1]), { why: 'none', dst: 3, level: 1 });
  assert.deepEqual(sourceJump({ i: 0, j: 0 }, axes, envOf([[3, 1, 9]]), [1]), { why: 'other', dst: 3, src: 9 });
});

test('g to a source the Filter hides says so, not that it is off the panel', () => {
  const shown = { srcs: [2, 1], dsts: axes.dsts };
  assert.deepEqual(sourceJump({ i: 0, j: 0 }, shown, envOf([[3, 1, 0]]), [1], axes.srcs), { why: 'filtered', dst: 3, src: 0 });
  assert.deepEqual(sourceJump({ i: 0, j: 0 }, shown, envOf([[3, 1, 9]]), [1], axes.srcs), { why: 'other', dst: 3, src: 9 });
  assert.deepEqual(sourceJump({ i: 0, j: 0 }, shown, envOf([[3, 1, 1]]), [1], axes.srcs), { c: { i: 0, j: 1 } });
});

test('G steps down through the destinations that have the source on any level, and wraps', () => {
  const env = envOf([
    [3, 1, 0],
    [0, 2, 0],
    [2, 1, 1],
  ]);
  // Source 0 is column 1; it is on destinations 3 (row 0) and 0 (row 2).
  assert.deepEqual(holderJump({ i: -1, j: 1 }, axes, env), { c: { i: 0, j: 1 } });
  assert.deepEqual(holderJump({ i: 0, j: 1 }, axes, env), { c: { i: 2, j: 1 } });
  assert.deepEqual(holderJump({ i: 2, j: 1 }, axes, env), { c: { i: 0, j: 1 } });
});

test('G on the only destination that has the source stays there', () => {
  assert.deepEqual(holderJump({ i: 3, j: 2 }, axes, envOf([[2, 1, 1]])), { c: { i: 3, j: 2 } });
});

test('G says why when no destination has the source, and does nothing off a source column', () => {
  assert.deepEqual(holderJump({ i: 0, j: 0 }, axes, envOf([[3, 1, 0]])), { why: 'unused', src: 2 });
  assert.equal(holderJump({ i: 0, j: -1 }, axes, envOf([])), null);
});

test('the note for a jump that cannot go', () => {
  const names = namesFrom(
    namesReply({
      sources: [[9, 'SAT 1']],
      destinations: [[3, 'MON D']],
    }),
  );
  const levels = tallyReply().levels;
  assert.equal(jumpText({ why: 'levels' }, names, levels), 'No levels in the Levels box to jump on');
  assert.equal(jumpText({ why: 'none', dst: 3, level: 1 }, names, levels), 'MON D has no source on level 1');
  assert.equal(jumpText({ why: 'other', dst: 3, src: 9 }, names, levels), 'SAT 1 on MON D is not on this panel');
  assert.equal(jumpText({ why: 'unused', src: 9 }, names, levels), 'No destination on this panel has SAT 1');
  assert.equal(jumpText({ why: 'filtered', dst: 3, src: 9 }, names, levels), 'SAT 1 on MON D is hidden by the Filter');
});
