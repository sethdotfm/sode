import test from 'node:test';
import assert from 'node:assert/strict';
import { stepInOffered, stepInUsers, stepsOut, stepActive } from '../stepinrules.js';

const users = [
  { name: 'Ana', len: 4 },
  { name: 'Cy', len: 0 },
];

test('Step in shows beside Sign in, never on the Lock screen, the phone or a second time', () => {
  const base = { locked: false, phone: false, stepped: false, user: '', users };
  assert.equal(stepInOffered(base), true);
  assert.equal(stepInOffered({ ...base, user: 'Cy' }), true);
  assert.equal(stepInOffered({ ...base, user: 'Ana' }), false, 'nobody else has a PIN');
  assert.equal(stepInOffered({ ...base, locked: true }), false);
  assert.equal(stepInOffered({ ...base, phone: true }), false);
  assert.equal(stepInOffered({ ...base, stepped: true }), false);
  assert.equal(stepInOffered({ ...base, users: [{ name: 'Cy', len: 0 }] }), false);
});

test('the Step in pad lists other users with a PIN', () => {
  assert.deepEqual(stepInUsers([...users, { name: 'Bo', len: 6 }], 'Bo'), [{ name: 'Ana', len: 4 }]);
  assert.deepEqual(stepInUsers(users, ''), [{ name: 'Ana', len: 4 }]);
  assert.deepEqual(stepInUsers(undefined, ''), []);
});

test('only a link to another page of Sode steps out', () => {
  const here = 'https://sode.test/';
  assert.equal(stepsOut('https://sode.test/log', here), true);
  assert.equal(stepsOut('https://sode.test/panels/editor', here), true);
  assert.equal(stepsOut('https://sode.test/#settings', here), false);
  assert.equal(stepsOut('https://example.com/log', here), false);
  assert.equal(stepsOut('https://sode.test/log', here, '_blank'), false);
});

test("a step-in's activity is this tab's own input, not the original's other tabs", () => {
  assert.equal(stepActive(true, 900, 100), 100);
  assert.equal(stepActive(false, 900, 100), 900);
});
