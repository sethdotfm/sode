import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sessionStep, sessionKeyReq, keyFrom, renewable, withKey, jsonCall, endedCode, retryAfterMs, guardFetch } from '../sessionkeyrules.js';

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

test('an ended key is final: key_revoked and key_expired, never a dropped link or a busy server', () => {
  assert.equal(endedCode(401, 'key_revoked'), true);
  assert.equal(endedCode(401, 'key_expired'), true);
  assert.equal(endedCode(401, 'invalid_key'), false);
  assert.equal(endedCode(429, 'rate_limited'), false);
  assert.equal(renewable(401, 'key_revoked'), true);
});

test('Retry-After is read as seconds, and a missing or odd one is no wait', () => {
  assert.equal(retryAfterMs('7'), 7000);
  assert.equal(retryAfterMs(null), 0);
  assert.equal(retryAfterMs('soon'), 0);
  assert.equal(retryAfterMs('-3'), 0);
});

// A fake clock: sleep moves time on, so a gated call returns at once in the test.
function clock() {
  const c = { t: 0 };
  c.now = () => c.t;
  c.sleep = async (ms) => {
    c.t += ms;
  };
  return c;
}

const reply = (status, code, headers = {}) =>
  new Response(code ? JSON.stringify({ error: { code, message: code } }) : '{}', { status, headers: { 'Content-Type': 'application/json', ...headers } });

test('after a 429 with Retry-After the next call waits that long first', async () => {
  const c = clock();
  const sent = [];
  const answers = [reply(429, 'rate_limited', { 'Retry-After': '5' }), reply(200), reply(200)];
  const f = guardFetch(async (p) => (sent.push([p, c.t]), answers.shift()), c);
  assert.equal((await f('a')).status, 429);
  assert.equal((await f('b')).status, 200);
  assert.deepEqual(sent, [
    ['a', 0],
    ['b', 5000],
  ]);
  await f('c');
  assert.equal(sent[2][1], 5000, 'no wait once it has passed');
});

test('a final answer is kept, and later calls get it without the network', async () => {
  let n = 0;
  const f = guardFetch(async () => (n++, reply(401, 'key_revoked')), { ...clock(), final: endedCode });
  for (let i = 0; i < 5; i++) {
    const r = await f('x');
    assert.equal(r.status, 401);
    assert.equal((await r.json()).error.code, 'key_revoked');
  }
  assert.equal(n, 1);
});

test('other refusals never stop the calls', async () => {
  let n = 0;
  const f = guardFetch(async () => (n++, reply(403, 'scope')), { ...clock(), final: endedCode });
  await f('x');
  await f('x');
  assert.equal(n, 2);
});
