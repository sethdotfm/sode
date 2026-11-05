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

import { prefFields, prefsWith, nameIssue, upsertEntry, filterEntry, findByName } from '../profilesrules.js';

test('prefFields: all eight, Not set first, saved value chosen', () => {
  const f = prefFields({ theme: 'dark', flip: true, clock: 'bogus' });
  assert.equal(f.length, 8);
  assert.equal(f.find((x) => x.key === 'theme').value, 'dark');
  assert.equal(f.find((x) => x.key === 'flip').value, 'on');
  assert.equal(f.find((x) => x.key === 'clock').value, '');
  assert.deepEqual(f[0].options[0], ['', 'Not set']);
  assert.ok(f.every((x) => x.options.length > 1));
});

test('prefsWith sets, removes on Not set, ignores bad values', () => {
  assert.deepEqual(prefsWith({ clock: '12' }, 'theme', 'dark'), { clock: '12', theme: 'dark' });
  assert.deepEqual(prefsWith({ clock: '12', theme: 'dark' }, 'theme', ''), { clock: '12' });
  assert.deepEqual(prefsWith({ clock: '12' }, 'theme', 'purple'), { clock: '12' });
  assert.deepEqual(prefsWith(null, 'nope', 'x'), {});
});

test('nameIssue: empty, long, taken ignoring case, own name ok', () => {
  const l = [{ name: 'Cams' }, { name: 'Mon' }];
  assert.match(nameIssue(l, '  '), /required/);
  assert.match(nameIssue(l, 'x'.repeat(33)), /32/);
  assert.match(nameIssue(l, 'cams', '', 'preset'), /preset named/);
  assert.equal(nameIssue(l, 'CAMS', 'cams'), '');
  assert.match(nameIssue(l, 'mon', 'Cams'), /already/);
  assert.equal(nameIssue(l, 'New'), '');
});

test('upsertEntry adds, replaces in place, refuses a clash', () => {
  const l = [{ name: 'A', v: 1 }, { name: 'B', v: 2 }];
  assert.deepEqual(upsertEntry(l, { name: 'C' }).list.map((x) => x.name), ['A', 'B', 'C']);
  const r = upsertEntry(l, { name: 'a2', v: 9 }, 'a').list;
  assert.deepEqual(r.map((x) => x.name), ['a2', 'B']);
  assert.ok(upsertEntry(l, { name: 'b' }, 'A').err);
  assert.deepEqual(l.map((x) => x.name), ['A', 'B']); // not mutated
});

test('filterEntry and findByName', () => {
  assert.deepEqual(filterEntry(' X ', { src: '1', sode: true }), { name: 'X', src: '1', sode: true });
  assert.equal(findByName([{ name: 'Cams' }], 'cams').name, 'Cams');
  assert.equal(findByName(null, 'a'), undefined);
});
