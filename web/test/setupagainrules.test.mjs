// Run setup again: the wizard's answers from the Settings values, and what Finish stages.
import test from 'node:test';
import assert from 'node:assert/strict';
import { againAnswers, againStage } from '../setupagainrules.js';
import { stepProblem } from '../setupwizardrules.js';

const standard = {
  key: 'standard',
  name: 'Standard access',
  fields: {
    locks: { main: false, log: false, panel: false },
    autoLockMinutes: 0,
    guestRole: 'standard',
    pinWaits: true,
    publicPanels: true,
    newPanelActs: 'public',
    newUserDefaults: { persistent: true, noAutoLock: true },
    guestApiScopes: ['names', 'tally', 'protected', 'status'],
    api: { guestEnabled: true },
  },
};
const secured = {
  key: 'secured',
  name: 'Secured access',
  fields: { ...standard.fields, locks: { main: true, log: false, panel: true }, publicPanels: false, newPanelActs: 'session', api: { guestEnabled: false } },
};
const presets = [standard, secured];

const now = () => ({
  brandText: 'Studio B',
  brandBorder: 'teal',
  router: '10.0.0.5:8910',
  levels: 4,
  levelNumbering: 'va',
  numberOffset: 0,
  fireMode: 'native',
  ['salvoGroup']: 12,
  ...structuredClone(standard.fields),
  // The API section's whole object: only guestEnabled is a preset's.
  api: { guestEnabled: true, keyCap: 50 },
  guestApiScopes: ['status', 'names', 'tally', 'protected'],
});

test('the current values fill every step, and the matching preset is picked', () => {
  const d = againAnswers(now(), presets);
  assert.equal(d.again, true);
  assert.equal(d.brandText, 'Studio B');
  assert.equal(d.brandBorder, 'teal');
  assert.equal(d.router, '10.0.0.5:8910');
  assert.equal(d.levels, '4');
  assert.equal(d.levelNumbering, 'va');
  assert.equal(d.numberOffset, '0');
  assert.equal(d.fireMode, 'native');
  assert.equal(d.group, '12');
  assert.equal(d.preset, 'standard');
});

test('access that matches no preset picks none, so Finish keeps it', () => {
  const d = againAnswers({ ...now(), pinWaits: false }, presets);
  assert.equal(d.preset, '');
  assert.deepEqual(againStage(d, { ...now(), pinWaits: false }), { fields: {}, preset: null });
});

test('Welcome asks for no user', () => {
  assert.equal(stepProblem(againAnswers(now(), presets), 0, 8), '');
});

test('Finish with nothing changed stages nothing', () => {
  assert.deepEqual(againStage(againAnswers(now(), presets), now()), { fields: {}, preset: null });
});

test('Finish stages only the changed fields and a newly picked preset', () => {
  const d = { ...againAnswers(now(), presets), brandText: ' Studio C ', router: '10.0.0.6:8910', levels: '4', preset: 'secured' };
  assert.deepEqual(againStage(d, now(), presets), { fields: { brandText: 'Studio C', router: '10.0.0.6:8910' }, preset: secured });
});

test('Sequential keeps the saved Router salvo group', () => {
  const d = { ...againAnswers(now(), presets), fireMode: 'sequential', group: '127' };
  assert.deepEqual(againStage(d, now()).fields, { fireMode: 'sequential' });
  const n = { ...againAnswers(now(), presets), group: '20' };
  assert.deepEqual(againStage(n, now()).fields, { ['salvoGroup']: 20 });
});

test('numbers stage as numbers', () => {
  const d = { ...againAnswers(now(), presets), levels: '8', numberOffset: '1', levelNumbering: '1' };
  assert.deepEqual(againStage(d, now()).fields, { levels: 8, numberOffset: 1, levelNumbering: '1' });
});
