import test from 'node:test';
import assert from 'node:assert/strict';
import { usersReq, MODES, ROLES, GUEST_ROLES, pinClashes, needsNewPins, clashNotice } from '../usersrules.js';

const rows = [
  { name: ' Ann ', id: 'u-1', pin: '', role: 'trusted', persistent: false },
  { name: 'Bob', id: '', pin: '4321', role: 'administrator', persistent: true },
];

test('a saved row sends its id so a rename keeps the user; a new row sends none', () => {
  assert.deepEqual(usersReq(rows, 8, 'pin', 'pin'), {
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
  assert.equal(usersReq([{ name: 'Cy', id: '', pin: '1', persistent: false }], 8, 'pin', 'pin').users[0].role, 'standard');
  assert.equal(usersReq(rows, 8, 'pin', 'pin', 'trusted', 'standard').guestRole, 'trusted');
  assert.equal('guestRole' in usersReq(rows, 8, 'pin', 'pin', 'standard', 'standard'), false);
});

test('a changed mode is sent; an unchanged one is not, so the server keeps its own', () => {
  assert.equal(usersReq(rows, 8, 'username', 'pin').signInMode, 'username');
  assert.equal(usersReq(rows, 8, 'pin', 'username').signInMode, 'pin');
  assert.equal('signInMode' in usersReq(rows, 8, 'username', 'username'), false);
  assert.equal('signInMode' in usersReq(rows, 8, 'pin', 'pin'), false);
});

test('no row carries a username: name and PIN mode signs in by the Name', () => {
  for (const u of usersReq(rows, 8, 'username', 'pin').users) assert.equal('username' in u, false);
  assert.deepEqual(
    MODES.map((m) => [m.value, m.label]),
    [
      ['pin', 'PIN only'],
      ['username', 'Name and PIN'],
    ],
  );
});

test('going back to PIN only needs a new, different PIN typed for every user', () => {
  const r = [
    { name: 'Ann', pin: '' },
    { name: ' Bob ', pin: '4321' },
    { name: 'Cy', pin: '4321' },
    { name: 'Di', pin: '5555' },
  ];
  assert.deepEqual(pinClashes(r, 'pin', 'username'), { missing: ['Ann'], same: ['Bob', 'Cy'] });
  assert.deepEqual(pinClashes(r, 'username', 'username'), { missing: [], same: [] });
  assert.deepEqual(pinClashes(r, 'pin', 'pin'), { missing: [], same: [] });
  assert.deepEqual(pinClashes(r, 'username', 'pin'), { missing: [], same: [] });
  const ok = [
    { name: 'Ann', pin: '1111' },
    { name: 'Bob', pin: '2222' },
  ];
  assert.deepEqual(pinClashes(ok, 'pin', 'username'), { missing: [], same: [] });
});

test('needsNewPins is true only when switching from name and PIN to PIN only', () => {
  assert.equal(needsNewPins('pin', 'username'), true);
  assert.equal(needsNewPins('pin', 'pin'), false);
  assert.equal(needsNewPins('username', 'username'), false);
  assert.equal(needsNewPins('username', 'pin'), false);
});

test('clashNotice names the users and what to do', () => {
  assert.equal(clashNotice({ missing: [], same: [] }), '');
  assert.equal(clashNotice({ missing: ['Ann'], same: [] }), 'Give Ann a new PIN to switch to PIN only. Sode cannot tell whether saved PINs match.');
  assert.equal(
    clashNotice({ missing: ['Ann', 'Bob', 'Cy'], same: [] }),
    'Give Ann, Bob and Cy a new PIN to switch to PIN only. Sode cannot tell whether saved PINs match.',
  );
  assert.equal(clashNotice({ missing: [], same: ['Ann', 'Bob'] }), 'Ann and Bob have the same new PIN. Each needs a different one.');
  assert.match(clashNotice({ missing: ['Cy'], same: ['Ann', 'Bob'] }), /^Give Cy a new PIN.* Ann and Bob have the same new PIN/);
});
