import test from 'node:test';
import assert from 'node:assert/strict';
import { usersReq, ROLES, GUEST_ROLES, ownerChoices, pinPlaceholder } from '../usersrules.js';

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

test('No PIN removes a saved user PIN only when no new PIN is typed; the placeholder says what Save does', () => {
  const r = usersReq(
    [
      { name: 'Ann', id: 'u-1', pin: '', noPin: true, hadPin: true, role: 'standard' },
      { name: 'Cy', id: 'u-2', pin: '', noPin: true, hadPin: false, role: 'standard' },
      { name: 'Dee', id: '', pin: '', noPin: true, role: 'standard' },
    ],
    8,
  );
  assert.deepEqual(
    r.users.map((u) => u.clearPin),
    [true, undefined, undefined],
  );
  assert.equal(pinPlaceholder({ id: 'u-1', hadPin: true, noPin: false }), 'unchanged');
  assert.equal(pinPlaceholder({ id: 'u-1', hadPin: true, noPin: true }), 'no PIN after Save');
  assert.equal(pinPlaceholder({ id: 'u-2', hadPin: false, pinNeeded: true }), 'PIN needed');
  assert.equal(pinPlaceholder({ id: 'u-2', hadPin: false }), 'no PIN');
  assert.equal(pinPlaceholder({ id: '', noPin: true }), 'no PIN');
  assert.equal(pinPlaceholder({ id: '' }), 'new PIN');
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

test('a removed user who owns panels gets a new owner, by default the acting user', () => {
  const listed = [
    { id: 'u-a', name: 'Ada', panels: 0 },
    { id: 'u-n', name: 'Ana', panels: 2 },
    { id: 'u-b', name: 'Bo', panels: 1 },
    { id: 'u-c', name: 'Cy' },
  ];
  const rows = [
    { id: 'u-a', name: 'Ada ' },
    { id: 'u-c', name: 'Cyd' },
    { id: '', name: 'New' },
  ];
  const kept = [
    { id: 'u-a', name: 'Ada' },
    { id: 'u-c', name: 'Cyd' },
  ];
  assert.deepEqual(ownerChoices(listed, rows, 'u-a'), [
    { id: 'u-n', name: 'Ana', panels: 2, kept, pick: 'u-a' },
    { id: 'u-b', name: 'Bo', panels: 1, kept, pick: 'u-a' },
  ]);
  // The acting user is not kept (or not a user): the first kept user.
  assert.equal(ownerChoices(listed, rows, 'u-n')[0].pick, 'u-a');
  assert.equal(ownerChoices(listed, rows.slice(1), 'u-a')[0].pick, 'u-c');
  assert.deepEqual(ownerChoices(listed, [], 'u-a')[0].kept, []);
  assert.equal(ownerChoices(listed, [], 'u-a')[0].pick, '');
  assert.deepEqual(ownerChoices(listed, listed, 'u-a'), []);
});

test('usersReq sends the chosen new owners', () => {
  const r = usersReq([], 8, 'standard', 'standard', { 'u-n': 'u-a' });
  assert.deepEqual(r.panelOwners, { 'u-n': 'u-a' });
  assert.equal('panelOwners' in usersReq([], 8, 'standard', 'standard', {}), false);
});
