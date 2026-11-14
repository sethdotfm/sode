import test from 'node:test';
import assert from 'node:assert/strict';
import { prefRows, withoutPref, setSummary, presetSummary, without, canSeeProfiles, profileButtonOff, PROFILE_NOTICE } from '../profilesrules.js';

test('prefRows: labels values, skips unknown ones', () => {
  const r = prefRows({ theme: 'dark', flip: true, clock: 'bogus', nope: 'x' });
  assert.deepEqual(
    r.map((x) => [x.key, x.value]),
    [
      ['theme', 'Dark'],
      ['flip', 'Yes'],
    ],
  );
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
  assert.equal(profileButtonOff('', 'Ana'), true);
  assert.equal(profileButtonOff('Ana', 'Ann'), true);
  assert.equal(profileButtonOff(undefined, ''), true);
});

import { prefFields, prefsWith, withParts, upsertEntry, filterEntry, findByName } from '../profilesrules.js';

test('prefFields: one per pref, Not set first, saved value chosen', () => {
  const f = prefFields({ theme: 'dark', flip: true, clock: 'bogus' });
  assert.equal(f.length, 9);
  assert.equal(f.find((x) => x.key === 'compact').label, 'Compact view');
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

test('upsertEntry adds or replaces in place, ignoring case, without mutating', () => {
  const l = [
    { name: 'A', v: 1 },
    { name: 'B', v: 2 },
  ];
  assert.deepEqual(
    upsertEntry(l, { name: 'C' }).map((x) => x.name),
    ['A', 'B', 'C'],
  );
  assert.deepEqual(
    upsertEntry(l, { name: 'a2', v: 9 }, 'a').map((x) => x.name),
    ['a2', 'B'],
  );
  assert.deepEqual(
    l.map((x) => x.name),
    ['A', 'B'],
  );
});

test('withParts: two quick pref changes accumulate (the second builds on the first)', () => {
  let prof = { prefs: { clock: '12' }, filterSets: [], presets: [] };
  const sent = [];
  for (const [k, v] of [
    ['theme', 'dark'],
    ['layout', 'phone'],
  ]) {
    const parts = { prefs: prefsWith(prof.prefs, k, v) };
    prof = withParts(prof, parts); // at change time, before any answer
    sent.push(parts);
  }
  assert.deepEqual(sent[1].prefs, { clock: '12', theme: 'dark', layout: 'phone' });
  assert.deepEqual(prof.prefs, sent[1].prefs);
  assert.deepEqual(withParts(null, { presets: [] }), { presets: [] });
});

test('filterEntry and findByName', () => {
  assert.deepEqual(filterEntry(' X ', { src: '1', sode: true }), { name: 'X', src: '1', sode: true });
  assert.equal(findByName([{ name: 'Cams' }], 'cams').name, 'Cams');
  assert.equal(findByName(null, 'a'), undefined);
});

test('PROFILE_NOTICE says a Profile is not private and who can see it', () => {
  assert.match(PROFILE_NOTICE, /not private/);
  assert.match(PROFILE_NOTICE, /trusted users/i);
  assert.match(PROFILE_NOTICE, /Advanced Settings PIN/);
});
