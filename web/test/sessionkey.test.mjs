import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sessionStep, sessionKeyReq, keyFrom, renewable, withKey, jsonCall } from '../sessionkeyrules.js';

const answer = (status, body) => async () => ({
  status,
  ok: status >= 200 && status < 300,
  json: async () => (body === undefined ? Promise.reject(new Error('empty')) : body),
});

test('jsonCall returns the body, and null for 204', async () => {
  assert.deepEqual(await jsonCall(answer(200, { a: 1 }))('p'), { a: 1 });
  assert.equal(await jsonCall(answer(204))('p'), null);
});

test('jsonCall throws the server message with status and code', async () => {
  const err = await jsonCall(answer(403, { error: { code: 'not_owner', message: 'no' } }), { failed: (s) => `It failed (${s})` })('p').catch((e) => e);
  assert.equal(err.message, 'no');
  assert.equal(err.status, 403);
  assert.equal(err.code, 'not_owner');
});

test('jsonCall falls back to the caller text when the body has no message', async () => {
  const err = await jsonCall(answer(500), { failed: (s) => `It failed (${s})` })('p').catch((e) => e);
  assert.equal(err.message, 'It failed (500)');
  assert.equal(err.status, 500);
  assert.equal(err.code, undefined);
});

test('jsonCall words a dead network with the caller text and status 0, or rethrows without one', async () => {
  const dead = async () => {
    throw new TypeError('offline');
  };
  const err = await jsonCall(dead, { down: 'Sode did not answer.' })('p').catch((e) => e);
  assert.equal(err.message, 'Sode did not answer.');
  assert.equal(err.status, 0);
  const raw = await jsonCall(dead)('p').catch((e) => e);
  assert.ok(raw instanceof TypeError);
});

test('jsonCall passes the path and init to the fetcher', async () => {
  let seen;
  await jsonCall(async (...a) => ((seen = a), answer(200, {})()))('x', { method: 'PUT' });
  assert.deepEqual(seen, ['x', { method: 'PUT' }]);
});

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
