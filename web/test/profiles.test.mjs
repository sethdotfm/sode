import test from 'node:test';
import assert from 'node:assert/strict';
import { prefRows, withoutPref, setSummary, presetSummary, without, canSeeProfiles, profileButtonOff } from '../profilesrules.js';

test('prefRows: labels values, skips unknown ones', () => {
  const r = prefRows({ theme: 'dark', flip: true, clock: 'bogus', nope: 'x' });
  assert.deepEqual(r.map((x) => [x.key, x.value]), [['theme', 'Dark'], ['flip', 'On']]);
  assert.deepEqual(prefRows(null), []);
});

test('withoutPref leaves the others', () => {
  assert.deepEqual(withoutPref({ theme: 'dark', clock: '12' }, 'theme'), { clock: '12' });
});

test('setSummary', () => {
  assert.equal(setSummary({}), 'Shows everything');
  assert.equal(setSummary({ src: 'CAM', ext: false, hidden: true }), 'sources CAM; hides changes made elsewhere; shows hidden destinations');
});

test('presetSummary counts takes and destinations', () => {
  assert.equal(presetSummary({ items: [{ dsts: [1, 2] }, { dsts: [3] }] }), '2 takes, 3 destinations');
  assert.equal(presetSummary({ items: [{ dsts: [1] }] }), '1 take, 1 destination');
  assert.equal(presetSummary({}), '0 takes, 0 destinations');
});

test('without drops one index', () => {
  assert.deepEqual(without(['a', 'b', 'c'], 1), ['a', 'c']);
  assert.deepEqual(without(null, 0), []);
});

test('canSeeProfiles', () => {
  assert.equal(canSeeProfiles([], true, true), false);
  assert.equal(canSeeProfiles(['Ana'], false, false), false);
  assert.equal(canSeeProfiles(['Ana'], true, false), true);
  assert.equal(canSeeProfiles(['Ana'], false, true), true);
});

test('profileButtonOff: only a saved, unrenamed user has a profile to open', () => {
  assert.equal(profileButtonOff('Ana', 'Ana'), false);
  assert.equal(profileButtonOff('Ana', ' Ana '), false);
  assert.equal(profileButtonOff('', 'Ana'), true); // a new row
  assert.equal(profileButtonOff('Ana', 'Ann'), true); // renamed, not saved
  assert.equal(profileButtonOff(undefined, ''), true);
});
