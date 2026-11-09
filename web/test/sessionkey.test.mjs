import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sessionStep, sessionKeyReq, keyFrom, renewable, withKey } from '../sessionkeyrules.js';

const IN = { user: 'Ana', locked: false, enforced: false, lens: [4] };

test('signed in with no key asks the server for one, with a key it is ready', () => {
  assert.equal(sessionStep(IN, ''), 'request');
  assert.equal(sessionStep(IN, 'sode_x'), 'ready');
});

test('signed out and not locked asks for the PIN, as long as users exist', () => {
  assert.equal(sessionStep({ user: '', locked: false, enforced: false, lens: [4] }, ''), 'sign-in');
  assert.equal(sessionStep({ user: '', locked: false, enforced: false, lens: [] }, ''), 'no-users');
});

test('a page lock shows its own pad, so the panel waits for it', () => {
  assert.equal(sessionStep({ user: '', locked: true, enforced: true, lens: [4] }, ''), 'wait');
  assert.equal(sessionStep({ user: '', locked: false, enforced: true, lens: [4] }, ''), 'wait');
  assert.equal(sessionStep({ ...IN, locked: true }, 'sode_x'), 'wait');
});

test('signing out drops a key the page still holds', () => {
  assert.equal(sessionStep({ user: '', locked: false, enforced: false, lens: [4] }, 'sode_x'), 'sign-in');
});

test('the request and the reply carry only what the server needs', () => {
  assert.deepEqual(sessionKeyReq(7), { t: 'sessionKey', id: 7 });
  assert.equal(keyFrom({ ok: true, key: 'sode_x' }), 'sode_x');
  assert.equal(keyFrom({ ok: false, err: 'unlock with your PIN first' }), '');
});

test('a key the server refused is renewed once, for the reasons a session ends', () => {
  assert.equal(renewable(401, 'invalid_key'), true);
  assert.equal(renewable(401, 'key_expired'), true);
  assert.equal(renewable(403, 'forbidden'), false);
  assert.equal(renewable(429, 'rate_limited'), false);
});

test('withKey sets the Authorization header and keeps the rest of the request', () => {
  const init = withKey({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }, 'sode_x');
  assert.equal(init.method, 'POST');
  assert.equal(init.body, '{}');
  assert.equal(init.headers['Content-Type'], 'application/json');
  assert.equal(init.headers.Authorization, 'Bearer sode_x');
});
