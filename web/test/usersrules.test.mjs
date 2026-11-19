import test from 'node:test';
import assert from 'node:assert/strict';
import { ROLES, GUEST_ROLES, ownerChoices, pinPlaceholder } from '../usersrules.js';

test('the PIN placeholder says what Save does', () => {
  assert.equal(pinPlaceholder({ id: 'u-1', hadPin: true, noPin: false }), 'unchanged');
  assert.equal(pinPlaceholder({ id: 'u-1', hadPin: true, noPin: true }), 'no PIN after Save');
  assert.equal(pinPlaceholder({ id: 'u-2', hadPin: false, pinNeeded: true }), 'PIN needed');
  assert.equal(pinPlaceholder({ id: 'u-2', hadPin: false }), 'no PIN');
  assert.equal(pinPlaceholder({ id: '', noPin: true }), 'no PIN');
  assert.equal(pinPlaceholder({ id: '' }), 'new PIN');
});

test('the Roles and the Guest roles', () => {
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
