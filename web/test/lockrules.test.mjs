import test from 'node:test';
import assert from 'node:assert/strict';
import {
  padKey, padCap, sendDelay, sendable, idleDue, idleLock, needsPin, guardedEdited, pageType, liftFrom, shouldFly, brandFrames,
  dots, actorUser, folderActor,
} from '../lockrules.js';

test('padKey adds digits, deletes and clears, capped at the longest PIN', () => {
  assert.equal(padKey('', '1', [4]), '1');
  assert.equal(padKey('12', '3', [4]), '123');
  assert.equal(padKey('123', 'back', [4]), '12');
  assert.equal(padKey('', 'back', [4]), '');
  assert.equal(padKey('123', 'clear', [4]), '');
  assert.equal(padKey('1', 'x', [4]), '1');
  assert.equal(padKey('1', 'Enter', [4]), '1');
  assert.equal(padKey('1234', '5', [4]), '1234');
  assert.equal(padKey('1234', '5', [4, 6]), '12345');
  assert.equal(padKey('123456', '7', [4, 6]), '123456');
});

test('the pad also stops at the configured maxPinLen', () => {
  assert.equal(padCap([4, 10], 8), 8);
  assert.equal(padCap([4, 6], 8), 6);
  assert.equal(padCap([4], 0), 4);
  assert.equal(padKey('12345678', '9', [4, 10], 8), '12345678');
  assert.equal(padKey('1234567', '8', [4, 10], 8), '12345678');
});

test('sendDelay: only the longest length submits by itself', () => {
  assert.equal(sendDelay('123', [4]), null);
  assert.equal(sendDelay('1234', [4]), 0);
  assert.equal(sendDelay('12345', [4]), null);
  assert.equal(sendDelay('1234', [4, 6]), null); // a pause after four never sends
  assert.equal(sendDelay('123456', [4, 6]), 0);
  assert.equal(sendDelay('12345', [4, 6]), null);
  assert.equal(sendDelay('1234', []), null);
});

test('1234 and 12345: the five-digit user can always log in, the four-digit one uses the check key', () => {
  const lens = [4, 5];
  let pin = '';
  for (const k of '1234') pin = padKey(pin, k, lens);
  assert.equal(sendDelay(pin, lens), null); // not sent on its own
  assert.equal(sendable(pin, lens), true); // the check key or Enter sends it
  pin = padKey(pin, '5', lens);
  assert.equal(pin, '12345');
  assert.equal(sendDelay(pin, lens), 0); // the longest goes at once
});

test('sendable: only lengths some user has', () => {
  assert.equal(sendable('123', [4, 6]), false);
  assert.equal(sendable('1234', [4, 6]), true);
  assert.equal(sendable('12345', [4, 6]), false);
  assert.equal(sendable('', [4]), false);
});

test('idleDue', () => {
  assert.equal(idleDue(0, 10 * 60_000, 0), false);
  assert.equal(idleDue(0, 4 * 60_000 + 59_999, 5), false);
  assert.equal(idleDue(0, 5 * 60_000, 5), true);
  assert.equal(idleDue(1000, 5 * 60_000, 5), false);
});

test('idleLock: a logged-in user locks when idle; a persistent one never does', () => {
  const t = 5 * 60_000;
  const user = { enforced: true, locked: false, user: 'Ana', persistent: false };
  assert.equal(idleLock(user, 0, t, 5), true);
  assert.equal(idleLock({ ...user, persistent: true }, 0, t, 5), false);
  assert.equal(idleLock({ ...user, locked: true }, 0, t, 5), false);
  assert.equal(idleLock({ ...user, enforced: false }, 0, t, 5), false);
  assert.equal(idleLock({ enforced: true, locked: false, user: '' }, 0, t, 5), false);
  assert.equal(idleLock(user, 0, t, 0), false);
  assert.equal(idleLock(user, 1000, t, 5), false);
});

test('needsPin: a trusted user skips the prompt until the server demands the PIN', () => {
  assert.equal(needsPin(false, false, false, false), false); // no PIN set
  assert.equal(needsPin(true, false, false, false), true);
  assert.equal(needsPin(true, true, false, false), false); // PIN entered
  assert.equal(needsPin(true, false, true, false), false); // trusted user
  assert.equal(needsPin(true, false, true, true), true); // the server refused: PIN after all
  assert.equal(needsPin(true, true, true, true), false);
});

test('pageType', () => {
  assert.equal(pageType('/'), 'main');
  assert.equal(pageType('/sode/'), 'main');
  assert.equal(pageType('/log'), 'log');
  assert.equal(pageType('/sode/log'), 'log');
  assert.equal(pageType('/log.html'), 'log');
  assert.equal(pageType('/catalog'), 'main');
});

test('liftFrom moves the centred logo back over the page logo', () => {
  const from = { left: 10, top: 20, width: 100, height: 40 };
  const to = { left: 300, top: 400, width: 400, height: 160 };
  const t = liftFrom(from, to);
  assert.equal(t.scale, 0.25);
  assert.equal(t.x, 60 - 500);
  assert.equal(t.y, 40 - 480);
  assert.deepEqual(liftFrom(null, to), { x: 0, y: 0, scale: 1 });
  assert.deepEqual(liftFrom({ left: 0, top: 0, width: 0, height: 0 }, to), { x: 0, y: 0, scale: 1 });
});

test('shouldFly: only an unscrolled page with the whole logo in view', () => {
  const r = { top: 40, bottom: 80, width: 100 };
  assert.equal(shouldFly(r, 0, 800), true);
  assert.equal(shouldFly(r, 1, 800), false);
  assert.equal(shouldFly({ ...r, top: -1 }, 0, 800), false);
  assert.equal(shouldFly({ ...r, bottom: 801 }, 0, 800), false);
  assert.equal(shouldFly({ ...r, bottom: 800 }, 0, 800), true);
  assert.equal(shouldFly(null, 0, 800), false);
  assert.equal(shouldFly({ top: 0, bottom: 0, width: 0 }, 0, 800), false);
});

test('brandFrames: fresh frames every run, a fade when not flying, a slow tail out', () => {
  const t = { x: -10, y: -20, scale: 0.5 };
  const a = brandFrames('in', true, t), b = brandFrames('in', true, t);
  assert.notEqual(a, b);
  assert.notEqual(a[0], b[0]);
  assert.deepEqual(a, b);
  assert.equal(a[0].transform, 'translate(-10px, -20px) scale(0.5)');
  assert.equal(a[1].transform, 'none');
  assert.deepEqual(brandFrames('in', false, t), [{ opacity: 0 }, { opacity: 1 }]);
  assert.deepEqual(brandFrames('out', false, t), [{ opacity: 1 }, { opacity: 0 }]);
  const out = brandFrames('out', true, t);
  assert.equal(out.at(-1).opacity, 0);
  assert.equal(out.at(-1).offset, 1);
  assert.ok(out.some((f) => f.offset > 0 && f.offset < 1 && f.opacity === 1));
});

test('actorUser names configured users only, never an address', () => {
  const users = [{ name: 'Ana' }, { name: 'Bo' }];
  assert.equal(actorUser('Ana', users), 'Ana');
  assert.equal(actorUser('10.0.0.1', users), '');
  assert.equal(actorUser('', users), '');
  assert.equal(actorUser(undefined, users), '');
  assert.equal(actorUser('Ana', []), '');
  assert.equal(actorUser('Ana', undefined), '');
});

test('folderActor: one name only when every take shares it', () => {
  const users = [{ name: 'Ana' }, { name: 'Bo' }];
  assert.equal(folderActor([{ client: 'Ana' }, { client: 'Ana' }], users), 'Ana');
  assert.equal(folderActor([{ client: 'Ana' }, { client: 'Bo' }], users), '');
  assert.equal(folderActor([{ client: 'Ana' }, { client: '10.0.0.1' }], users), '');
  assert.equal(folderActor([{ client: '10.0.0.1' }], users), '');
});

test('dots', () => {
  assert.equal(dots(''), '');
  assert.equal(dots('123').length, 3);
});

test('guardedEdited: router, trusted proxies and the PIN limit need the PIN', () => {
  const cur = { router: 'a:1', pinMaxTries: 10, trustedProxies: ['10.0.0.2'] };
  assert.equal(guardedEdited({ ...cur }, cur), false);
  assert.equal(guardedEdited({ ...cur, router: 'b:1' }, cur), true);
  assert.equal(guardedEdited({ ...cur, pinMaxTries: 0 }, cur), true);
  assert.equal(guardedEdited({ ...cur, trustedProxies: ['0.0.0.0/0'] }, cur), true);
  assert.equal(guardedEdited({ ...cur, trustedProxies: undefined }, { ...cur, trustedProxies: [] }), false);
});
