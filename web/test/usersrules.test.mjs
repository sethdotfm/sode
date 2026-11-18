import test from 'node:test';
import assert from 'node:assert/strict';
import { usersReq, ROLES, GUEST_ROLES } from '../usersrules.js';

const rows = [
  { name: ' Ann ', id: 'u-1', pin: '', role: 'trusted', persistent: false },
  { name: 'Bob', id: '', pin: '4321', role: 'administrator', persistent: true },
];

test('a saved row sends its id so a rename keeps the user; a new row sends none', () => {
  assert.deepEqual(usersReq(rows, 8), {
    users: [
      { id: 'u-1', name: 'Ann', pin: '', role: 'trusted', persistent: false },
      { id: '', name: 'Bob', pin: '4321', role: 'administrator', persistent: true },
    ],
    maxPinLen: 8,
  });
});

test('every row sends its Role; Guest role goes only when changed', () => {
  assert.deepEqual(
    ROLES.map((r) => [r.value, r.label]),
    [
      ['standard', 'Standard user'],
      ['trusted', 'Trusted user'],
      ['administrator', 'Administrator'],
    ],
  );
  assert.deepEqual(
    GUEST_ROLES.map((r) => r.value),
    ['standard', 'trusted'],
  );
  assert.equal(usersReq([{ name: 'Cy', id: '', pin: '1', persistent: false }], 8).users[0].role, 'standard');
  assert.equal(usersReq(rows, 8, 'trusted', 'standard').guestRole, 'trusted');
  assert.equal('guestRole' in usersReq(rows, 8, 'standard', 'standard'), false);
});

test('no row and no request carries a username or a sign-in mode: every user signs in by the Name', () => {
  const req = usersReq(rows, 8);
  assert.equal('signInMode' in req, false);
  for (const u of req.users) assert.equal('username' in u, false);
});
