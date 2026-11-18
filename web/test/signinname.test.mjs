import { test } from 'node:test';
import assert from 'node:assert/strict';
import { nameMatches, listedUser, pickName } from '../namepickrules.js';
import { asksName, padLens, canSend, padDelay, signInMsg, prefillName, focusName, sendsOnPick, pinNeededAsk, pinForm } from '../lockrules.js';

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
  // A user without a PIN takes no digits.
  assert.deepEqual(padLens(s, { name: 'Cy', len: 0 }), [0]);
});

test('a user without a PIN signs in when their name is picked, with no PIN', () => {
  const s = { enforced: false, locked: false, lens: [4, 6] };
  const cy = { name: 'Cy', len: 0 };
  assert.equal(sendsOnPick(s, cy), true);
  assert.equal(sendsOnPick(s, { name: 'Bo', len: 6 }), false);
  assert.equal(canSend(s, '', cy), true);
  assert.equal(canSend(s, '', null), false);
  assert.deepEqual(signInMsg(s, { pin: '', id: 6, user: cy }), { t: 'userUnlock', user: 'Cy', pin: '', id: 6, tab: true });
});

test("a held Lock of a user without a PIN asks for the name again; the holder's own name resumes it", () => {
  const held = { held: true, heldNoPin: true, lens: [4] };
  const cy = { name: 'Cy', len: 0 };
  assert.equal(asksName(held), true);
  assert.equal(canSend(held, '', null), false);
  assert.equal(sendsOnPick(held, cy), true);
  assert.deepEqual(signInMsg(held, { pin: '', id: 7, user: cy }), { t: 'userUnlock', user: 'Cy', pin: '', id: 7, tab: true });
});

test('pinNeededAsk: a signed-in user who must set a PIN is asked once per sign-in', () => {
  assert.equal(pinNeededAsk({ user: 'Cy', pinNeeded: true }, ''), 'Cy');
  assert.equal(pinNeededAsk({ user: 'Cy', pinNeeded: true }, 'Cy'), '');
  assert.equal(pinNeededAsk({ user: 'Cy', pinNeeded: false }, ''), '');
  assert.equal(pinNeededAsk({ user: '', pinNeeded: true }, ''), '');
});

test('pinForm: Change my PIN asks the current PIN only of a user with one; only a Standard user removes it', () => {
  assert.deepEqual(pinForm({ name: 'Ana', len: 4 }, 'standard'), { current: true, remove: true, save: 'Change PIN' });
  assert.deepEqual(pinForm({ name: 'Bo', len: 6 }, 'trusted'), { current: true, remove: false, save: 'Change PIN' });
  assert.deepEqual(pinForm({ name: 'Cy', len: 0 }, 'standard'), { current: false, remove: false, save: 'Set PIN' });
  assert.deepEqual(pinForm(null, 'standard'), { current: true, remove: false, save: 'Change PIN' });
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
  assert.deepEqual(signInMsg(s, { pin: '1234', id: 4, user: ada }), { t: 'userUnlock', user: 'Ada', pin: '1234', id: 4, tab: false });
});

test('a held Lock resumes with the PIN alone at the holder length', () => {
  const held = { enforced: false, held: true, lens: [4, 6] };
  const ana = { name: 'Ana', len: 4 };
  assert.equal(canSend(held, '1234', ana), true);
  assert.equal(canSend(held, '1234', null), true);
  assert.equal(padDelay(held, '1234', ana), 0);
  assert.deepEqual(signInMsg(held, { pin: '1234', id: 5, user: ana }), { t: 'userUnlock', pin: '1234', id: 5, tab: true });
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
