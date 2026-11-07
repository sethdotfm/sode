import test from 'node:test';
import assert from 'node:assert/strict';
import { usersReq, usernameColumn, MODES, pinClashes, needsNewPins, clashNotice, renameNotice, prefillUsernames } from '../usersrules.js';

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

test('needsNewPins is true only when switching from Username and PIN to PIN only', () => {
  assert.equal(needsNewPins('pin', 'username'), true);
  assert.equal(needsNewPins('pin', 'pin'), false);
  assert.equal(needsNewPins('username', 'username'), false);
  assert.equal(needsNewPins('username', 'pin'), false);
});

test('clashNotice names the users and what to do', () => {
  assert.equal(clashNotice({ missing: [], same: [] }), '');
  assert.equal(clashNotice({ missing: ['Ann'], same: [] }), "Give Ann a new PIN to switch to PIN only. Sode can't tell whether saved PINs match.");
  assert.equal(
    clashNotice({ missing: ['Ann', 'Bob', 'Cy'], same: [] }),
    "Give Ann, Bob and Cy a new PIN to switch to PIN only. Sode can't tell whether saved PINs match.",
  );
  assert.equal(clashNotice({ missing: [], same: ['Ann', 'Bob'] }), 'Ann and Bob have the same new PIN. Each needs a different one.');
  assert.match(clashNotice({ missing: ['Cy'], same: ['Ann', 'Bob'] }), /^Give Cy a new PIN.* Ann and Bob have the same new PIN/);
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

const pre = (...pairs) => prefillUsernames(pairs.map(([name, username = '']) => ({ name, username })));

test('prefill drops spaces from the Name', () => {
  assert.deepEqual(pre(['Ann Lee'], ['Bob']), ['AnnLee', 'Bob']);
});

test('prefill never overwrites a username and counts it as taken', () => {
  assert.deepEqual(pre(['Ann Lee', 'custom'], ['Bob', 'Bobby'], ['Cy', 'bob']), ['custom', 'Bobby', 'bob']);
  assert.deepEqual(pre(['Ann', 'Bob'], ['Bob']), ['Bob', 'Bob2']);
});

test('prefill numbers names that collapse to the same username, ignoring case', () => {
  assert.deepEqual(pre(['A B'], ['AB'], ['a b'], ['ab']), ['AB', 'AB2', 'ab3', 'ab4']);
});

test('prefill keeps within 24 characters when it numbers', () => {
  const long = 'x'.repeat(30);
  const out = pre([long], [long]);
  assert.equal(out[0], 'x'.repeat(24));
  assert.equal(out[1], 'x'.repeat(23) + '2');
});

test('prefill leaves a nameless row empty and trims the typed username', () => {
  assert.deepEqual(pre(['  '], ['Zed', ' zed ']), ['', 'zed']);
});
