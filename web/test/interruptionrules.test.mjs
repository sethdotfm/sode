import test from 'node:test';
import assert from 'node:assert/strict';
import { stopsText, saveStopsLabel, endsSomething } from '../interruptionrules.js';

const lobby = { text: 'Public panel Lobby stops', panels: 1, ends: true };
const streams = { text: 'The keyless API goes off', streams: 2, ends: true };
const mainLock = { text: 'The Main page lock puts 2 browsers on a lock screen', browsers: 2 };
const guest = { text: 'Guest role goes down' };

test('the Save button names what the save stops', () => {
  const three = [lobby, { ...lobby }, { ...lobby }];
  assert.equal(saveStopsLabel(4, 0, three, 'Save 4 changes'), 'Save and stop 3 panels');
  assert.equal(saveStopsLabel(4, 1, [lobby, streams], 'x'), 'Save 4 and stop 1 panel, 2 streams, keep 1 unsaved');
  assert.equal(saveStopsLabel(1, 0, [mainLock], 'x'), 'Save and lock 2 browsers');
});

test('with nothing that stops, or nothing that saves, the plain label stays', () => {
  assert.equal(saveStopsLabel(2, 0, [], 'Save 2 changes'), 'Save 2 changes');
  assert.equal(saveStopsLabel(2, 0, [guest], 'Save 2 changes'), 'Save 2 changes');
  assert.equal(saveStopsLabel(0, 2, [lobby], 'Nothing to save'), 'Nothing to save');
  assert.equal(saveStopsLabel(2, 0, null, 'Save 2 changes'), 'Save 2 changes');
});

test('stops and locks join', () => {
  assert.equal(stopsText([{ sessions: 1, keys: 2 }, mainLock]), 'stop 1 session, 2 keys and lock 2 browsers');
  assert.equal(stopsText([guest]), '');
});

test('the step-ins a save ends count on the Save button', () => {
  const stepIns = { text: 'Step-ins of Bo: this save ends 1 step-in.', stepIns: 1, ends: true };
  assert.equal(saveStopsLabel(1, 0, [stepIns], 'x'), 'Save and stop 1 step-in');
  assert.equal(stopsText([{ sessions: 2 }, { ...stepIns, stepIns: 2 }]), 'stop 2 sessions, 2 step-ins');
});

test('the do not come back line shows only when something ends', () => {
  assert.equal(endsSomething([lobby]), true);
  assert.equal(endsSomething([mainLock, guest]), false);
  assert.equal(endsSomething(null), false);
});
