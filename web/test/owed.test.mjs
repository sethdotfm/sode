import test from 'node:test';
import assert from 'node:assert/strict';
import { createOwed } from '../owed.js';

const fakeSend = () => {
  const s = { up: false, sent: [] };
  s.send = (m) => {
    if (!s.up) return false;
    s.sent.push(m.t);
    return true;
  };
  return s;
};
const signedIn = { user: 'Ana', held: false };
const held = { user: '', held: true };
const gone = { user: '', held: false };

test('a Lock the server never got is sent again while the session answers unlocked', () => {
  const s = fakeSend();
  const o = createOwed(s.send);
  o.owe('userLock');
  s.up = true;
  assert.equal(o.arrived(signedIn), true);
  assert.deepEqual(s.sent, ['userLock']);
  assert.equal(o.pending(), '');
});

test('a Lock is dropped once the server holds it or the session is gone', () => {
  for (const m of [held, gone]) {
    const s = fakeSend();
    const o = createOwed(s.send);
    o.owe('userLock');
    s.up = true;
    assert.equal(o.arrived(m), false);
    assert.equal(o.pending(), '');
    assert.deepEqual(s.sent, []);
  }
});

test('a refused userSignOut is sent again while the session still answers signed in or held', () => {
  for (const m of [signedIn, held]) {
    const s = fakeSend();
    const o = createOwed(s.send);
    o.owe('userSignOut');
    s.up = true;
    assert.equal(o.arrived(m), true);
    assert.deepEqual(s.sent, ['userSignOut']);
  }
});

test('a userSignOut is dropped once the session is gone', () => {
  const s = fakeSend();
  const o = createOwed(s.send);
  o.owe('userSignOut');
  s.up = true;
  assert.equal(o.arrived(gone), false);
  assert.deepEqual(s.sent, []);
});

test('a resend that is refused again stays owed', () => {
  const s = fakeSend();
  const o = createOwed(s.send);
  o.owe('userSignOut');
  assert.equal(o.arrived(signedIn), true);
  assert.equal(o.pending(), 'userSignOut');
});

test('one slot: a later message replaces an earlier one; a sent one owes nothing', () => {
  const s = fakeSend();
  const o = createOwed(s.send);
  o.owe('userLock');
  o.owe('userSignOut');
  assert.equal(o.pending(), 'userSignOut');
  s.up = true;
  o.owe('userLock');
  assert.equal(o.pending(), '');
  assert.equal(o.arrived(signedIn), false);
});
