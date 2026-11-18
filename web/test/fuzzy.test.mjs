// The main page's fuzzy scoring (fuzzy.js), which core.js search and the Grid panel's Find both use.
import test from 'node:test';
import assert from 'node:assert/strict';
import { fuzzyScore } from '../fuzzy.js';

const it = (num, name) => ({ num, lname: name.toLowerCase() });

test('an exact number beats a name; a number prefix, a name prefix and a name part follow in that order', () => {
  assert.equal(fuzzyScore(it('12', 'CAM 1'), '12'), 1000);
  assert.equal(fuzzyScore(it('12', 'cam 1'), 'cam 1'), 900);
  assert.equal(fuzzyScore(it('123', 'CAM 1'), '12'), 797);
  assert.equal(fuzzyScore(it('5', 'CAM 1'), 'cam'), 695);
  assert.equal(fuzzyScore(it('5', 'BIG CAM'), 'cam'), 596);
});

test('letters in order anywhere in "number name" match; letters out of order do not', () => {
  assert.ok(fuzzyScore(it('13', 'Replay A'), 'rpa') > 100);
  assert.equal(fuzzyScore(it('13', 'Replay A'), 'zz'), -1);
});
