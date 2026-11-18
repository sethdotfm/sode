import { test } from 'node:test';
import assert from 'node:assert/strict';
import { nameMatches, listedUser, pickName } from '../namepickrules.js';
import { asksName, padLens, canSend, padDelay, signInMsg, prefillName, focusName } from '../lockrules.js';

// The hello's sign-in list.
const users = [
  { name: 'Ana Kis', len: 4 },
  { name: 'Bo', len: 6 },
  { name: 'Ada', len: 4 },
  { name: 'Dana', len: 5 },
];

test('nameMatches: an empty box lists every user in order; typing finds by fuzzy match, best first', () => {
  assert.deepEqual(
    nameMatches(users, '').map((u) => u.name),
    ['Ana Kis', 'Bo', 'Ada', 'Dana'],
  );
  assert.deepEqual(
    nameMatches(users, '  ').map((u) => u.name),
    ['Ana Kis', 'Bo', 'Ada', 'Dana'],
  );
  assert.deepEqual(
    nameMatches(users, 'ad').map((u) => u.name),
    ['Ada'],
  );
  // A start beats a middle; a subsequence across words still matches.
  assert.deepEqual(
    nameMatches(users, 'an').map((u) => u.name),
    ['Ana Kis', 'Dana'],
  );
  assert.deepEqual(
    nameMatches(users, 'ak').map((u) => u.name),
    ['Ana Kis'],
  );
  assert.deepEqual(nameMatches(users, 'zz'), []);
  assert.deepEqual(nameMatches(undefined, 'a'), []);
});

test('listedUser and pickName: the exact name ignoring case, else the best match; never a name that is not listed', () => {
  assert.deepEqual(listedUser(users, ' bo '), { name: 'Bo', len: 6 });
  assert.equal(listedUser(users, 'b'), null);
  assert.equal(pickName(users, 'BO'), 'Bo');
  assert.equal(pickName(users, 'kis'), 'Ana Kis');
  assert.equal(pickName(users, 'zz'), '');
  assert.equal(pickName(users, ''), '');
});

test('the pad asks for a name, except where a held Lock resumes', () => {
  assert.equal(asksName({ enforced: true, locked: true }), true);
  assert.equal(asksName({ held: true }), false);
});

test("a picked user's own PIN length drives the pad; with none known, every length in use", () => {
  const s = { lens: [4, 5, 6] };
  assert.deepEqual(padLens(s, { name: 'Bo', len: 6 }), [6]);
  assert.deepEqual(padLens(s, null), [4, 5, 6]);
  assert.deepEqual(padLens(s, { name: 'Cy', len: 0 }), [4, 5, 6]);
});

test('nothing sends, by hand or by itself, without a listed name; at its own length it sends at once', () => {
  const s = { enforced: true, locked: true, lens: [4, 6] };
  const bo = { name: 'Bo', len: 6 };
  const ada = { name: 'Ada', len: 4 };
  assert.equal(canSend(s, '1234', null), false);
  assert.equal(padDelay(s, '123456', null), null);
  assert.equal(canSend(s, '1234', bo), false);
  assert.equal(canSend(s, '123456', bo), true);
  assert.equal(padDelay(s, '123456', bo), 0);
  // Ada's PIN is the shortest in use, but her own: no wait for a longer one.
  assert.equal(padDelay(s, '1234', ada), 0);
  assert.deepEqual(signInMsg(s, { pin: '1234', id: 4, user: ada }), { t: 'userSignIn', user: 'Ada', pin: '1234', id: 4, tab: false });
});

test('a held Lock resumes with the PIN alone at the holder length', () => {
  const held = { enforced: false, held: true, lens: [4, 6] };
  const ana = { name: 'Ana', len: 4 };
  assert.equal(canSend(held, '1234', ana), true);
  assert.equal(canSend(held, '1234', null), true);
  assert.equal(padDelay(held, '1234', ana), 0);
  assert.deepEqual(signInMsg(held, { pin: '1234', id: 5, user: ana }), { t: 'userSignIn', pin: '1234', id: 5, tab: true });
});

test('prefillName: the remembered name, only while it is still listed; then the pad takes the keys', () => {
  const s = { held: false };
  assert.equal(prefillName(s, 'ada', users), 'Ada');
  assert.equal(prefillName(s, 'Cy', users), '');
  assert.equal(prefillName(s, '', users), '');
  assert.equal(prefillName({ held: true }, 'Ada', users), '');
  assert.equal(focusName(s, 'Ada'), false);
  assert.equal(focusName(s, ''), true);
  assert.equal(focusName({ held: true }, ''), false);
});
