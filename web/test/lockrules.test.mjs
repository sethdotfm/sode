import test from 'node:test';
import assert from 'node:assert/strict';
import {
  padKey,
  padCap,
  sendDelay,
  sendable,
  idleDue,
  idleLock,
  needsPin,
  guardedEdited,
  pageType,
  liftFrom,
  shouldFly,
  brandFrames,
  dots,
  actorUser,
  groupActor,
  wrongFeedback,
  WRONG_MS,
  churnKey,
  localIdleLock,
  signInOffered,
  signInAsOffered,
  signOutOffered,
  MODE,
  PAGE_VIEW,
  viewBody,
  lockView,
  lockCovers,
  raiseLock,
  signedOut,
  tokenStale,
  lockDismissed,
  startSignIn,
  signInCancelled,
  lockArrived,
  signInWon,
  chipShown,
  heldEnded,
  heldWho,
  remembers,
  prefillName,
  focusName,
  wantsTab,
  hiToken,
  tokenHome,
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
  assert.equal(sendDelay(pin, lens), null);
  assert.equal(sendable(pin, lens), true);
  pin = padKey(pin, '5', lens);
  assert.equal(pin, '12345');
  assert.equal(sendDelay(pin, lens), 0);
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

test('idleLock: every signed-in user locks when idle, persistent ones and tab sign-ins included', () => {
  const t = 5 * 60_000;
  const user = { enforced: true, locked: false, user: 'Ana', persistent: false };
  assert.equal(idleLock(user, 0, t, 5), true);
  assert.equal(idleLock({ ...user, persistent: true }, 0, t, 5), true);
  assert.equal(idleLock({ ...user, locked: true }, 0, t, 5), false);
  assert.equal(idleLock({ ...user, enforced: false }, 0, t, 5), true);
  assert.equal(idleLock({ enforced: true, locked: false, user: '' }, 0, t, 5), false);
  assert.equal(idleLock(user, 0, t, 0), false);
  assert.equal(idleLock(user, 1000, t, 5), false);
});

test('needsPin: a trusted user skips the prompt', () => {
  assert.equal(needsPin(false, false, false), false);
  assert.equal(needsPin(true, false, false), true);
  assert.equal(needsPin(true, true, false), false);
  assert.equal(needsPin(true, false, true), false);
  assert.equal(needsPin(true, true, true), false);
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
  const a = brandFrames('in', true, t),
    b = brandFrames('in', true, t);
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
  const users = ['Ana', 'Bo'];
  assert.equal(actorUser('Ana', users), 'Ana');
  assert.equal(actorUser('10.0.0.1', users), '');
  assert.equal(actorUser('', users), '');
  assert.equal(actorUser(undefined, users), '');
  assert.equal(actorUser('Ana', []), '');
  assert.equal(actorUser('Ana', undefined), '');
});

test('groupActor: one name only when every take shares it', () => {
  const users = ['Ana', 'Bo'];
  assert.equal(groupActor([{ client: 'Ana' }, { client: 'Ana' }], users), 'Ana');
  assert.equal(groupActor([{ client: 'Ana' }, { client: 'Bo' }], users), '');
  assert.equal(groupActor([{ client: 'Ana' }, { client: '10.0.0.1' }], users), '');
  assert.equal(groupActor([{ client: '10.0.0.1' }], users), '');
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
  assert.equal(guardedEdited({ ...cur, fireMode: 'native' }, cur), true);
  assert.equal(guardedEdited({ ...cur, matrix: 2 }, cur), true);
  assert.equal(guardedEdited({ ...cur, trustedProxies: ['0.0.0.0/0'] }, cur), true);
  assert.equal(guardedEdited({ ...cur, trustedProxies: undefined }, { ...cur, trustedProxies: [] }), false);
});

test('wrongFeedback: Incorrect for a plain wrong PIN, server waits kept as text', () => {
  assert.deepEqual(wrongFeedback('wrong PIN'), { text: 'Incorrect', ms: WRONG_MS, note: '' });
  assert.deepEqual(wrongFeedback(''), { text: 'Incorrect', ms: WRONG_MS, note: '' });
  assert.deepEqual(wrongFeedback('wrong PIN; try again in 30s'), { text: 'Incorrect', ms: WRONG_MS, note: 'Try again in 30s' });
  const w = wrongFeedback('too many wrong PINs; try again in 25s');
  assert.equal(w.text, 'Too many wrong PINs; try again in 25s');
  assert.equal(w.ms, 0);
  assert.equal(wrongFeedback('another PIN is being checked; try again').ms, 0);
});

test('churnKey: a letter, never the one before, never one that is easy to confuse', () => {
  let last = '';
  const seen = new Set();
  for (let i = 0; i < 2000; i++) {
    const k = churnKey(last);
    assert.match(k, /^[a-z]$/);
    assert.notEqual(k, last);
    assert.ok(!'ilo'.includes(k), k);
    seen.add(k);
    last = k;
  }
  assert.equal(seen.size, 23);
  // Even if the dice keep landing on the last key, it moves on.
  const rolls = [0, 0, 0.5];
  assert.notEqual(
    churnKey('a', () => rolls.shift()),
    'a',
  );
});

test('localIdleLock: idle past the auto-lock minutes, only where no PIN lock is on and not already locked', () => {
  const idle = 5 * 60_000;
  assert.equal(localIdleLock({ enforced: false }, false, 0, idle, 5), true);
  assert.equal(localIdleLock({ enforced: false }, false, 0, idle - 1, 5), false);
  assert.equal(localIdleLock({ enforced: false }, false, 0, idle, 0), false);
  assert.equal(localIdleLock({ enforced: false }, true, 0, idle, 5), false);
  assert.equal(localIdleLock({ enforced: true }, false, 0, idle, 5), false);
  assert.equal(localIdleLock({ enforced: false, locked: true, held: true, user: '' }, false, 0, idle, 5), false); // a held Lock is not a letter Lock
});

test('tab signedIn: offered only with users, no lock and nobody in', () => {
  assert.equal(signInOffered({ enforced: false, user: '', lens: [4] }), true);
  assert.equal(signInOffered({ enforced: false, user: '', lens: [] }), false);
  assert.equal(signInOffered({ enforced: true, user: '', lens: [4] }), false);
  assert.equal(signInOffered({ enforced: false, user: 'Ana', lens: [4] }), false);
  assert.equal(signInOffered({ enforced: false, user: '' }), false);
});

test('tab signedIn: tokens', () => {
  assert.equal(wantsTab({ enforced: false }), true);
  assert.equal(wantsTab({ enforced: true }), false);
  assert.equal(hiToken('t', 'p'), 't');
  assert.equal(hiToken('', 'p'), 'p');
  assert.equal(hiToken(null, null), '');
  assert.equal(tokenHome({ tab: true }), 'tab');
  assert.equal(tokenHome({}), 'browser');
});

test('guardedEdited: the locks and the idle time need the PIN (Go guard:"pin")', () => {
  const cur = { router: 'a:1', pinMaxTries: 10, autoLockMinutes: 0, locks: { main: true, log: false } };
  assert.equal(guardedEdited({ ...cur, locks: { main: true, log: false } }, cur), false);
  assert.equal(guardedEdited({ ...cur, locks: { main: false, log: false } }, cur), true);
  assert.equal(guardedEdited({ ...cur, locks: { main: true, log: true } }, cur), true);
  assert.equal(guardedEdited({ ...cur, autoLockMinutes: 5 }, cur), true);
  assert.equal(guardedEdited({ ...cur, locks: undefined }, { ...cur, locks: {} }), false);
});

test('Lock: Sign in as is offered with users and no lock, signed in or not', () => {
  assert.equal(signInAsOffered({ enforced: false, user: '', lens: [4] }), true);
  assert.equal(signInAsOffered({ enforced: false, user: 'Ana', lens: [4] }), true);
  assert.equal(signInAsOffered({ enforced: false, user: '', lens: [] }), false);
  assert.equal(signInAsOffered({ enforced: true, user: '', lens: [4] }), false);
  assert.equal(signInAsOffered({ enforced: false }), false);
});

test('lock view: which body shows', () => {
  assert.equal(viewBody({ mode: 'lock', back: 'page' }, false), 'letter');
  assert.equal(viewBody({ mode: 'signIn', back: 'lock' }, false), 'pad');
  assert.equal(viewBody(PAGE_VIEW, true), 'pad');
  assert.equal(viewBody(PAGE_VIEW, false), '');
});

test('lock view: transitions', () => {
  const lock = lockView(PAGE_VIEW, false);
  assert.deepEqual(lock, { mode: 'lock', back: 'page' });
  assert.equal(lockView(PAGE_VIEW, true), PAGE_VIEW);
  assert.equal(lockDismissed(lock), PAGE_VIEW);
  assert.deepEqual(startSignIn(PAGE_VIEW, false), { mode: 'signIn', back: 'page' });
  assert.equal(startSignIn(lock, false), lock); // Settings' button never over the Lock
  assert.deepEqual(startSignIn(lock, true), { mode: 'signIn', back: 'lock' });
  assert.equal(startSignIn(PAGE_VIEW, true), PAGE_VIEW);
  const fromLock = startSignIn(lock, true);
  assert.equal(fromLock.back, 'lock'); // the caller reads this before cancelling
  assert.equal(signInCancelled(fromLock), PAGE_VIEW);
  assert.equal(signInCancelled(lock), lock);
  assert.deepEqual(lockView(fromLock, false), { mode: 'lock', back: 'page' }); // idle lock wins over a pad
  assert.equal(lockArrived(lock, true), PAGE_VIEW);
  assert.equal(lockArrived(lock, false), lock);
  assert.equal(lockArrived(fromLock, true), fromLock);
});

test('lock view: a signedIn ends only with a real unlock or an enforced lock', () => {
  const signedIn = { mode: 'signIn', back: 'lock' };
  assert.equal(signInWon(signedIn, { enforced: false, unlocked: false, user: 'Ana' }), signedIn); // someone else's state
  assert.equal(signInWon(signedIn, { enforced: false, unlocked: true, user: '' }), signedIn);
  assert.equal(signInWon(signedIn, { enforced: false, unlocked: true, user: 'Ana' }), PAGE_VIEW);
  assert.equal(signInWon(signedIn, { enforced: true, unlocked: false, user: '' }), PAGE_VIEW);
  assert.equal(signInWon(PAGE_VIEW, { enforced: false, unlocked: true, user: 'Ana' }), PAGE_VIEW);
});

test('Lock: the chip shows for a logged-in user under it only', () => {
  assert.equal(chipShown({ covered: true, letter: true, user: 'Ana' }), true);
  assert.equal(chipShown({ covered: true, letter: true, user: '' }), false);
  assert.equal(chipShown({ covered: false, letter: false, user: '' }), true);
  assert.equal(chipShown({ covered: true, letter: false, user: 'Ana' }), false);
  assert.equal(chipShown({ covered: true, letter: false, user: '', heldUser: 'Ana' }), true); // the held pad says whose it is
  assert.equal(chipShown({ covered: true, letter: false, user: '', heldUser: '' }), false);
});

test('Lock: Sign out offered only with a user and no PIN lock; the view stays Lock', () => {
  assert.equal(signOutOffered({ enforced: false, user: 'Ana', lens: [4] }), true);
  assert.equal(signOutOffered({ enforced: false, user: '', lens: [4] }), false);
  assert.equal(signOutOffered({ enforced: true, user: 'Ana', lens: [4] }), false);
  const v = lockView(PAGE_VIEW, false);
  // logging out changes no view: lockArrived(false) and signInWon leave the Lock alone
  assert.deepEqual(lockArrived(v, false), v);
  assert.deepEqual(signInWon(v, { enforced: false, unlocked: false, user: '' }), v);
  assert.equal(signInAsOffered({ enforced: false, user: '', lens: [4] }), true);
});

test('a signed-in Lock is held by the server: a pad with no Cancel that only a PIN ends', () => {
  const user = { enforced: false, locked: false, held: false, user: 'Ana', trusted: true, persistent: true };
  assert.equal(lockCovers(user), false);
  const held = raiseLock(user);
  assert.deepEqual(held, { enforced: false, locked: true, held: true, user: '', trusted: false, persistent: false });
  assert.equal(lockCovers(held), true);
  assert.equal(viewBody(PAGE_VIEW, lockCovers(held)), 'pad');
  assert.equal(lockCovers({ enforced: true, locked: true }), true);
  assert.equal(lockCovers({ enforced: false, locked: true }), false); // nobody signed in, no PIN lock: the letter Lock's turn
});

test('Sign out from the Lock clears it; an enforced lock then asks for a Sign in', () => {
  const held = { enforced: false, locked: true, held: true, user: '' };
  assert.deepEqual(signedOut(held), { enforced: false, locked: false, held: false, user: '', trusted: false, persistent: false });
  assert.equal(lockCovers(signedOut({ ...held, enforced: true })), true);
});

test('a held Lock keeps the browser session token across a reload', () => {
  assert.equal(tokenStale({ hi: true, held: true }, ''), false);
  assert.equal(tokenStale({ hi: true }, ''), true);
  assert.equal(tokenStale({ hi: true }, 'Ana'), false);
  assert.equal(tokenStale({ held: false }, ''), false); // not the answer to hi
});

test('a held Lock cancels the letter Lock', () => {
  assert.equal(lockArrived({ mode: MODE.LOCK, back: 'page' }, true), PAGE_VIEW);
});

test('username mode asks for a username, except when resuming a held Lock', async () => {
  const { asksUsername, canSend, padDelay, signInMsg } = await import('../lockrules.js');
  const pinOnly = { enforced: true, locked: true, mode: 'pin', lens: [4] };
  const named = { enforced: true, locked: true, mode: 'username', lens: [4], held: false };
  assert.equal(asksUsername(pinOnly), false);
  assert.equal(asksUsername({}), false);
  assert.equal(asksUsername(named), true);
  assert.equal(asksUsername({ ...named, held: true, user: 'Ann' }), false);

  // PIN only: the old rules and message, whatever the field holds
  assert.equal(canSend(pinOnly, '1234', ''), true);
  assert.equal(padDelay(pinOnly, '1234', ''), 0);
  assert.deepEqual(signInMsg(pinOnly, { pin: '1234', id: 3, username: 'ann' }), { t: 'userUnlock', pin: '1234', id: 3, tab: false });

  // Username and PIN: nothing sends, by hand or by itself, without a username
  assert.equal(canSend(named, '1234', ''), false);
  assert.equal(canSend(named, '1234', '  '), false);
  assert.equal(canSend(named, '1234', 'ann'), true);
  assert.equal(canSend(named, '123', 'ann'), false);
  assert.equal(padDelay(named, '1234', ''), null);
  assert.equal(padDelay(named, '1234', 'ann'), 0);
  assert.deepEqual(signInMsg(named, { pin: '1234', id: 4, username: ' ann ' }), { t: 'userUnlock', user: 'ann', pin: '1234', id: 4, tab: false });
  const held = { ...named, held: true, user: 'Ann' };
  assert.deepEqual(signInMsg(held, { pin: '1234', id: 5, username: 'zed' }), { t: 'userUnlock', pin: '1234', id: 5, tab: false });
  assert.equal(canSend(held, '1234', ''), true);
});

test('heldEnded: a held Lock that ends with nobody signed in was ended elsewhere; one that ends signed in was resumed', () => {
  assert.equal(heldEnded({ held: true }, { held: false, user: '' }), true);
  assert.equal(heldEnded({ held: true }, { held: false, user: 'Ana' }), false); // another tab's PIN resumed it
  assert.equal(heldEnded({ held: true }, { held: true, user: '' }), false);
  assert.equal(heldEnded({ held: false }, { held: false, user: '' }), false);
});

test('heldWho: this tab remembers whose Lock it is; a Lock that is gone forgets it', () => {
  assert.equal(heldWho('', { user: 'Ana' }, { held: true }), 'Ana'); // raised from another tab
  assert.equal(heldWho('Ana', { user: '' }, { held: true }), 'Ana'); // reload or a later state
  assert.equal(heldWho('Ana', { user: '' }, { held: false }), '');
  assert.equal(heldWho('', { user: '' }, { held: true }), '');
});

test('remembers: only a sign-in without a browser-wide token keeps the username', () => {
  assert.equal(remembers({ ok: true }), true);
  assert.equal(remembers({ ok: true, token: 't', tab: true }), true);
  assert.equal(remembers({ ok: true, token: 't' }), false); // Never sign out: the token already signs them in
});

test('prefillName and focusName: username mode fills the box, and then the pad takes the keys', () => {
  assert.equal(prefillName({ mode: 'username', held: false }, 'ana'), 'ana');
  assert.equal(prefillName({ mode: 'username', held: true }, 'ana'), '');
  assert.equal(prefillName({ mode: 'pin', held: false }, 'ana'), '');
  assert.equal(focusName({ mode: 'username', held: false }, 'ana'), false);
  assert.equal(focusName({ mode: 'username', held: false }, ''), true);
  assert.equal(focusName({ mode: 'pin', held: false }, ''), false);
});
