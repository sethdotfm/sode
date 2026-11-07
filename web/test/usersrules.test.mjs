import test from 'node:test';
import assert from 'node:assert/strict';
import { usersReq, usernameColumn, MODES, modeOneWay, switchNeedsWarning, renameNotice } from '../usersrules.js';

const rows = [
  { name: ' Ann ', was: 'Ann', pin: '', trusted: true, persistent: false, username: ' ann ' },
  { name: 'Bob', was: '', pin: '4321', trusted: false, persistent: true, username: '' },
];

test('PIN only saves no username and no mode change', () => {
  assert.deepEqual(usersReq(rows, 8, 'pin', 'pin'), {
    users: [
      { name: 'Ann', was: 'Ann', pin: '', trusted: true, persistent: false },
      { name: 'Bob', was: '', pin: '4321', trusted: false, persistent: true },
    ],
    maxPinLen: 8,
  });
});

test('Username and PIN saves trimmed usernames and the mode', () => {
  const r = usersReq(rows, 8, 'username', 'pin');
  assert.equal(r.signInMode, 'username');
  assert.deepEqual(
    r.users.map((u) => u.username),
    ['ann', ''],
  );
});

test('an unchanged mode is not sent, so the server keeps its own', () => {
  assert.equal('signInMode' in usersReq(rows, 8, 'username', 'username'), false);
  assert.equal('signInMode' in usersReq(rows, 8, 'pin', 'pin'), false);
});

test('switching back to PIN only sends the mode and leaves usernames alone', () => {
  const r = usersReq(rows, 8, 'pin', 'username');
  assert.equal(r.signInMode, 'pin');
  assert.equal('username' in r.users[0], false);
});

test('the username column shows in Username and PIN mode only', () => {
  assert.equal(usernameColumn('username'), true);
  assert.equal(usernameColumn('pin'), false);
  assert.equal(usernameColumn(undefined), false);
  assert.deepEqual(
    MODES.map((m) => [m.value, m.label]),
    [
      ['pin', 'PIN only'],
      ['username', 'Username and PIN'],
    ],
  );
});

test('Username and PIN is one-way: no way back once saved, a warning before the first switch', () => {
  assert.equal(modeOneWay('username'), true);
  assert.equal(modeOneWay('pin'), false);
  assert.equal(modeOneWay(undefined), false);
  assert.equal(switchNeedsWarning('username', 'pin'), true);
  assert.equal(switchNeedsWarning('username', 'username'), false);
  assert.equal(switchNeedsWarning('pin', 'pin'), false);
});

test('renameNotice names each renamed saved user and stays quiet otherwise', () => {
  assert.equal(renameNotice(rows), '');
  assert.equal(
    renameNotice([
      { name: ' Anna ', was: 'Ann' },
      { name: 'Bob', was: '' },
    ]),
    'Renaming Ann to Anna signs that user out everywhere. Their Profile and PIN stay.',
  );
  assert.match(
    renameNotice([
      { name: 'A2', was: 'A' },
      { name: 'B2', was: 'B' },
    ]),
    /A to A2, B to B2 signs/,
  );
});
