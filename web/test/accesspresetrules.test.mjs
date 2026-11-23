import test from 'node:test';
import assert from 'node:assert/strict';
import { presetTarget, presetMarks, presetGaps, gapText } from '../accesspresetrules.js';
import { diffRows } from '../savereviewrules.js';

// The usersList answer's Secured access (accessPresetView in internal/app/accesspreset.go).
const secured = {
  key: 'secured',
  name: 'Secured access',
  fields: {
    locks: { main: true, log: false, panel: true },
    autoLockMinutes: 0,
    guestRole: 'standard',
    pinWaits: true,
    publicPanels: false,
    newPanelActs: 'session',
    newUserDefaults: { persistent: false, noAutoLock: false },
    guestApiScopes: ['names', 'tally', 'protected', 'status'],
    api: { guestEnabled: false },
  },
  users: { persistent: false, noAutoLock: false },
};

const live = {
  locks: { main: false, log: false, panel: false },
  autoLockMinutes: 0,
  guestRole: 'trusted',
  pinWaits: false,
  publicPanels: true,
  newPanelActs: 'public',
  newUserDefaults: { persistent: true, noAutoLock: true },
  guestApiScopes: ['names', 'tally', 'protected', 'status', 'presets', 'take'],
  api: { rateLimit: 20, takeCap: 0, guestEnabled: true, logReads: false },
  'user/u1/persistent': true,
  'user/u1/noAutoLock': true,
  'user/u2/persistent': true,
  'user/u2/noAutoLock': true,
  'user/u2/role': 'standard',
};

function staged(target) {
  const now = structuredClone(live);
  for (const [k, v] of Object.entries(target)) now[k] = v && typeof v === 'object' && !Array.isArray(v) ? { ...now[k], ...v } : v;
  return now;
}

test('ticked users get the preset Persistent and No auto-lock; unticked users get nothing', () => {
  const t = presetTarget(secured, ['u1']);
  assert.equal(t['user/u1/persistent'], false);
  assert.equal(t['user/u1/noAutoLock'], false);
  assert.ok(!Object.keys(t).some((k) => k.startsWith('user/u2/')));
});

test('a preset stages no remove and no other user attribute', () => {
  const t = presetTarget(secured, ['u1', 'u2']);
  assert.deepEqual(
    Object.keys(t)
      .filter((k) => k.startsWith('user/'))
      .sort(),
    ['user/u1/noAutoLock', 'user/u1/persistent', 'user/u2/noAutoLock', 'user/u2/persistent'],
  );
});

test('applying Secured access marks every change it staged with its name', () => {
  const t = presetTarget(secured, ['u1']);
  const rows = diffRows(live, staged(t));
  const marks = presetMarks(new Map(), rows, secured.name, t);
  assert.equal(rows.length, 12); // auto-lock and the Log lock are already the preset's
  for (const r of rows) assert.equal(marks.get(`${r.key}/${r.sub}`)?.preset, 'Secured access', `${r.key}/${r.sub}`);
});

test('a hand edit on a staged change drops its mark; a hand edit elsewhere gets none', () => {
  const t = presetTarget(secured, []);
  let marks = presetMarks(new Map(), diffRows(live, staged(t)), secured.name, t);
  const now = { ...staged(t), autoLockMinutes: 5, 'user/u2/role': 'trusted' };
  marks = presetMarks(marks, diffRows(live, now), null, null);
  assert.equal(marks.get('autoLockMinutes/null'), undefined);
  assert.equal(marks.get('user/u2/role/null'), undefined);
  assert.equal(marks.get('locks/main')?.preset, 'Secured access');
});

test('scopes in another order still match the preset', () => {
  const t = presetTarget(secured, []);
  const now = { ...staged(t), guestApiScopes: ['status', 'names', 'protected', 'tally'] };
  const marks = presetMarks(new Map(), diffRows(live, now), secured.name, t);
  assert.equal(marks.get('guestApiScopes/null')?.preset, 'Secured access');
  assert.deepEqual(presetGaps(t, now), []);
});

test('after a partly refused save, the fields that still differ from the preset are listed with what they do live', () => {
  const t = presetTarget(secured, ['u1']);
  // The save wrote everything but the Main page lock and the keyless API (someone else saved them).
  const after = staged(t);
  after.locks = { ...after.locks, main: false };
  after.api = { ...after.api, guestEnabled: true };
  const gaps = presetGaps(t, after);
  assert.deepEqual(
    gaps.map((g) => [g.key, g.sub, g.live]),
    [
      ['locks', 'main', false],
      ['api', 'guestEnabled', true],
    ],
  );
  assert.match(gapText(gaps[0]), /Main page opens with no PIN/);
  assert.match(gapText(gaps[1]), /^Keyless API: on$/);
});

test('nothing differs after a whole save', () => {
  const t = presetTarget(secured, ['u1']);
  assert.deepEqual(presetGaps(t, staged(t)), []);
});

test("the Guest's Use tie lines right is part of a preset: a Guest that still has it is a gap", () => {
  const t = presetTarget({ ...secured, guestTieLines: false }, []);
  const saved = staged(presetTarget(secured, []));
  // Unset Guest Rights is on (config.Rights.TieLines).
  for (const guestRights of [{}, { tieLines: true }]) {
    const gaps = presetGaps(t, { ...saved, guestRights });
    assert.deepEqual(
      gaps.map((g) => [g.key, g.sub, g.live]),
      [['guestRights', 'tieLines', true]],
    );
    assert.equal(gapText(gaps[0]), 'Guest Use tie lines: on');
  }
  assert.deepEqual(presetGaps(t, { ...saved, guestRights: { tieLines: false } }), []);
  // Guest Rights that did not load are not known, so they are no gap.
  assert.deepEqual(presetGaps(t, saved), []);
});
