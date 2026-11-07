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
  o.owe();
  s.up = true;
  assert.equal(o.arrived(signedIn), true);
  assert.deepEqual(s.sent, ['userLock']);
  assert.equal(o.arrived(signedIn), false);
});

test('a Lock is dropped once the server holds it or the session is gone', () => {
  for (const m of [held, gone]) {
    const s = fakeSend();
    const o = createOwed(s.send);
    o.owe();
    s.up = true;
    assert.equal(o.arrived(m), false);
    assert.equal(o.arrived(signedIn), false);
    assert.deepEqual(s.sent, []);
  }
});

test('a resend that is refused again stays owed', () => {
  const s = fakeSend();
  const o = createOwed(s.send);
  o.owe();
  assert.equal(o.arrived(signedIn), true);
  s.up = true;
  assert.equal(o.arrived(signedIn), true);
  assert.deepEqual(s.sent, ['userLock']);
});

test('a Lock that went out owes nothing', () => {
  const s = fakeSend();
  s.up = true;
  const o = createOwed(s.send);
  o.owe();
  assert.deepEqual(s.sent, ['userLock']);
  assert.equal(o.arrived(signedIn), false);
});
