// The lock screen (#151). The server decides everything: it tells this page
// whether the page type is locked and who (if anyone) unlocked it
// (`lockState`), and refuses what a locked connection asks for. This module
// shows a full-viewport pad that sends the PIN on its own at the longest
// length in use (shorter ones with the check key or Enter), and locks again
// when idle or on Ctrl+L. A user the server calls persistent ("never log
// out") is given a session token when they unlock; this page keeps it and
// presents it in `hi`, so a reload or a dropped link does not log them out,
// and the idle timer skips them. Locking (Lock, Log out, Ctrl+L) revokes it.
//
// Off unless the server says this page type is locked: with no users and no
// locks nothing here shows or is bound but a few idle listeners.
//
// Where no PIN lock is on (#158) the same screen can still be put up by hand
// (Ctrl+L, or Settings > This device) as a guard against keyboard mashing: it
// opens with a churning letter, like a take's confirm, and the confirm lockout
// has to pass before that letter (or the Unlock button, for a touch screen)
// works. It is this page's own state, not the server's: no PIN, nothing
// enforced, and a reload clears it.

import { el, S, on, emit, onMsg, wsSend, sentence } from './core.js';
import { readStr, writeStr, readTab, writeTab } from './store.js';
import { padKey, sendDelay, sendable, idleLock, pageType, liftFrom, shouldFly, brandFrames, OUT_FACTOR, dots, wrongFeedback, WELCOME_MS, churnKey, softIdleLock, loginOffered, wantsTab, hiToken, tokenHome } from './lockrules.js';
import { lockout } from './lockout.js';
import { buildChip } from './lockchip.js';

// The session token of a persistent user. localStorage rather than a cookie:
// the socket's `hi` carries it, no HTTP endpoint is needed, and a cookie
// would be JS-readable just the same unless HttpOnly (see AGENTS.md).
const TOKEN_KEY = 'sode-session';
// A no-lock login's token: this tab only (sessionStorage), so closing the tab
// logs out while a reload or reconnect stays in.
const TAB_KEY = 'sode-tab-session';
const page = pageType(location.pathname);

{
  const css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = 'lock.css';
  document.head.append(css);
}

// What the server last said about this connection.
const L = { enforced: false, locked: false, user: '', trusted: false, persistent: false, lens: [] };
export const lockState = () => ({ ...L });

let hiFor = null; // the socket `hi` was sent on
on('conn', () => {
  if (S.wsState !== 'up' || S.ws === hiFor) return;
  hiFor = S.ws;
  wsSend({ t: 'hi', page, token: hiToken(readTab(TAB_KEY), readStr(TOKEN_KEY)) });
});

// Keys of the removed admin-browser sign-in: drop what an older page left.
writeStr('sode-lock-token', null);
writeStr('sode-lock-never', null);

// ---- the overlay, built the first time it is needed

let root = null;
let brand, dotsEl, msg, state, chip, checkKey;
let padBody, softBody, softKey, softBtn, softNote;
let login = false; // the pad is up by request (no lock on), dismissable
let cancelBtn;
let soft = null; // { key, lock } while the screen is locked by hand (no PIN lock on)
let lastSoftKey = '';
let pin = '';
let seq = 0; // the userUnlock whose reply counts
let timer = 0;
let inerted = [];
let hideTimer = 0;

function build() {
  if (root) return;
  brand = el('h1', 'sode-lock__brand', 'Sode');
  state = el('p', 'sode-lock__state', 'Locked');
  dotsEl = el('div', 'sode-lock__dots');
  dotsEl.setAttribute('aria-hidden', 'true');
  msg = el('p', 'sode-lock__msg');
  msg.setAttribute('role', 'status');
  msg.setAttribute('aria-live', 'polite');

  const pad = el('div', 'sode-lock__pad');
  const key = (label, k, cls = '') => {
    const b = el('button', `pb-btn pb-btn--outline sode-lock__key ${cls}`, label);
    b.type = 'button';
    b.dataset.key = k;
    b.addEventListener('click', () => press(k));
    return b;
  };
  // Submits what is typed: shorter PINs than the longest don't go on their own.
  checkKey = key('✓', 'enter', 'sode-lock__key--go');
  checkKey.setAttribute('aria-label', 'Submit PIN');
  checkKey.disabled = true;
  pad.append(
    ...['1', '2', '3', '4', '5', '6', '7', '8', '9'].map((d) => key(d, d)),
    key('Clear', 'clear', 'sode-lock__key--word'),
    key('0', '0'),
    key('Del', 'back', 'sode-lock__key--word'),
    checkKey,
  );

  cancelBtn = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancelBtn.type = 'button';
  cancelBtn.hidden = true;
  cancelBtn.addEventListener('click', cancelLogin);
  padBody = el('div', 'sode-lock__body', state, dotsEl, msg, pad, cancelBtn);

  // The hand lock's body: the churning letter and, for a touch screen, a button.
  softKey = el('kbd', 'sode__key sode__key--locked');
  softBtn = el('button', 'pb-btn pb-btn--ok', 'Unlock');
  softBtn.type = 'button';
  softBtn.addEventListener('click', softUnlock);
  softNote = el('p', 'sode-lock__msg');
  softNote.setAttribute('role', 'status');
  softNote.setAttribute('aria-live', 'polite');
  softBody = el('div', 'sode-lock__body', el('p', 'sode-lock__soft-line', 'Press ', softKey, ' to unlock'), softBtn, softNote);
  softBody.hidden = true;
  root = el('div', 'sode-lock', brand, padBody, softBody);
  root.hidden = true;
  root.tabIndex = -1;
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'true');
  root.setAttribute('aria-label', 'Locked');
  document.body.append(root);

  // Who is logged in, with the one lock button.
  chip = buildChip({ lock: () => (L.enforced ? lockNow() : softLock()) });
  document.body.append(chip.root);
}

function say(text, bad = false) {
  if (!root) return;
  msg.textContent = text;
  msg.className = `sode-lock__msg${bad ? ' pb-note pb-note--error' : ''}`;
}

function render() {
  const covered = (L.enforced && L.locked) || !!soft || login;
  const showChip = !covered; // always there, even where no lock is on
  if (covered) showLock();
  else hideLock();
  build();
  chip.update(L, showChip);
  document.documentElement.toggleAttribute('data-sode-chip', !!showChip);
  if (showChip) placeChip();
}

// On the phone the chip sits under the fixed header; say where that ends.
function placeChip() {
  const head = document.querySelector('.sode-m__head');
  const b = head?.getBoundingClientRect();
  if (b?.width) document.documentElement.style.setProperty('--sode-head-h', `${b.bottom}px`);
}
addEventListener('resize', () => { if (chip && !chip.root.hidden) placeChip(); });

const logo = () => [...document.querySelectorAll('.sode__mark, .sode-m__mark, .sode-log__title')].find((e) => e.getBoundingClientRect().width > 0);
const calmMotion = matchMedia('(prefers-reduced-motion: reduce)');
const motionMs = () => (calmMotion.matches ? 0 : parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--sode-lock-motion')) || 0);

// One run of the logo's move (lock = 'in', unlock = 'out'). Every run starts
// from the brand at rest with its own keyframes, so it restarts reliably
// however soon after the last one it is (a CSS transition measured while the
// last was still running did not). Flies only when shouldFly; otherwise the
// logo fades with the overlay and the page stays where it is.
function moveBrand(dir) {
  for (const a of brand.getAnimations()) a.cancel();
  const ms = motionMs();
  if (!ms || !brand.animate) return;
  const from = logo()?.getBoundingClientRect();
  const fly = !logo()?.classList.contains('sode-log__title') && shouldFly(from, window.scrollY, window.innerHeight); // /log's heading is text: it fades, not scales
  const t = fly ? liftFrom(from, brand.getBoundingClientRect()) : { x: 0, y: 0, scale: 1 };
  brand.animate(brandFrames(dir, fly, t), {
    duration: dir === 'in' ? ms : ms * OUT_FACTOR,
    easing: dir === 'in' ? 'cubic-bezier(0.2, 0, 0, 1)' : 'cubic-bezier(0.4, 0, 0.2, 1)',
    fill: dir === 'out' ? 'forwards' : 'none',
  });
}

let scrolled = null; // where the page was when the lock came down

function showLock() {
  build();
  padBody.hidden = !!soft; // the PIN pad, or the hand lock's letter
  cancelBtn.hidden = !login;
  softBody.hidden = !soft;
  root.setAttribute('aria-label', 'Locked');
  clearTimeout(hideTimer);
  if (!root.hidden && root.classList.contains('is-on')) return;
  pin = '';
  welcoming = false;
  clearFeedback();
  say('');
  const text = S.cfg?.brandText?.trim() || 'Sode';
  brand.textContent = text;
  const frame = document.querySelector('.sode__brandframe');
  if (frame?.dataset.brand) root.dataset.brand = frame.dataset.brand;
  else delete root.dataset.brand;
  // The page keeps its scroll through lock and unlock; focus moves must not
  // scroll it (desktop.js leaves its focus-scroll alone while this is set).
  if (root.hidden) scrolled = { x: window.scrollX, y: window.scrollY };
  document.documentElement.dataset.sodeLock = 'on';
  root.hidden = false;
  moveBrand('in');
  void root.offsetWidth; // commit the start before the class changes it
  root.classList.add('is-on');
  if (!login) closeModals(); // a requested login keeps Settings open behind it
  inerted = [];
  for (const c of document.body.children) inert(c);
  // Anything added to the page while locked is covered too.
  watch ||= new MutationObserver((list) => { for (const r of list) for (const n of r.addedNodes) inert(n); });
  watch.observe(document.body, { childList: true });
  // Remember where the keyboard was, to give it back on unlock.
  const had = document.activeElement;
  if (had && had !== document.body && !root.contains(had)) before = had;
  had?.blur?.();
  root.focus({ preventScroll: true });
}

let watch = null;
let before = null; // what had focus when the lock came down
function inert(c) {
  if (c.nodeType !== 1 || c === root || c === chip?.root || ['SCRIPT', 'LINK', 'STYLE'].includes(c.tagName) || c.inert) return;
  c.inert = true;
  inerted.push(c);
}

// Open dialogs go first, so nothing is left half-done behind the overlay.
// Native <dialog>s close here; a module that owns a dialog or a view listens
// for 'locking' and puts it away itself (Settings exits; an unsaved form keeps
// its warning and stays, covered).
function closeModals() {
  for (const d of document.querySelectorAll('dialog[open]')) d.close();
  emit('locking');
  // Fallback for the app's modals, which don't listen yet: take their own
  // Cancel/Close, else hide them. Goes when each has a 'locking' listener.
  for (const m of document.querySelectorAll('.pb-modal-backdrop:not([hidden])')) {
    const b = [...m.querySelectorAll('button')].find((x) => /cancel|close/i.test(x.id));
    if (b) b.click();
    else m.hidden = true;
  }
}

function hideLock() {
  if (!root || root.hidden) return;
  watch?.disconnect();
  clearTimeout(timer);
  for (const c of inerted) c.inert = false;
  inerted = [];
  const wasOn = root.classList.contains('is-on');
  root.classList.remove('is-on');
  pin = '';
  // The way out is slower than the way in (--sode-lock-out in lock.css is
  // the same OUT_FACTOR), so the last of it fades gently; the root is hidden
  // only once the longest of its transitions is over.
  const ms = motionMs();
  const out = ms * OUT_FACTOR;
  if (wasOn) moveBrand('out');
  clearTimeout(hideTimer);
  hideTimer = setTimeout(() => {
    root.hidden = true;
    for (const a of brand.getAnimations()) a.cancel();
    delete document.documentElement.dataset.sodeLock;
  }, out + 100);
  // The element that had focus before locking, if it is still on the page.
  if (before?.isConnected) before.focus?.({ preventScroll: true });
  before = null;
  if (scrolled && (window.scrollX !== scrolled.x || window.scrollY !== scrolled.y)) window.scrollTo({ left: scrolled.x, top: scrolled.y, behavior: 'instant' });
}

// ---- the field: dots, and "Incorrect" / "Welcome, <name>" in their place

let fbTimer = 0;
let welcoming = false; // a correct PIN: keys do nothing until the unlock runs
let settle = null; // the unlock waiting for its welcome to be read

// Puts text in the dots' place, with a short fade (and, for a wrong PIN, a
// shake that restarts every time). Reduced motion: only the text.
function feedback(text, kind, ms = 0, shake = false) {
  clearTimeout(fbTimer);
  dotsEl.textContent = text;
  dotsEl.className = `sode-lock__dots is-text is-${kind}`;
  dotsEl.removeAttribute('aria-hidden');
  dotsEl.setAttribute('role', 'status');
  if (motionMs() && dotsEl.animate) {
    for (const a of dotsEl.getAnimations()) a.cancel();
    dotsEl.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 200 });
    if (shake) {
      dotsEl.animate(
        [{ transform: 'translateX(0)' }, { transform: 'translateX(-0.6rem)' }, { transform: 'translateX(0.6rem)' }, { transform: 'translateX(-0.4rem)' }, { transform: 'translateX(0.4rem)' }, { transform: 'translateX(0)' }],
        { duration: 400, easing: 'ease-in-out' });
    }
  }
  if (ms) fbTimer = setTimeout(() => { clearFeedback(); paint(); }, ms);
}

function clearFeedback() {
  clearTimeout(fbTimer);
  if (!dotsEl) return;
  for (const a of dotsEl.getAnimations()) a.cancel();
  dotsEl.className = 'sode-lock__dots';
  dotsEl.removeAttribute('role');
  dotsEl.setAttribute('aria-hidden', 'true');
  paint();
}

// ---- entering a PIN

function paint() {
  dotsEl.textContent = dots(pin);
  state.textContent = pin ? `${pin.length} ${pin.length === 1 ? 'digit' : 'digits'} entered` : 'Please enter your PIN.';
  checkKey.disabled = !sendable(pin, lens());
}

const lens = () => L.lens;

function press(k) {
  clearTimeout(timer);
  if (welcoming) return;
  if (dotsEl.classList.contains('is-text')) clearFeedback();
  if (k === 'enter') {
    // The check key and Enter send whatever is typed, if a user has that length.
    if (sendable(pin, lens())) sendPin();
    else say(pin ? 'No PIN has that many digits.' : 'Enter your PIN.', true);
    return;
  }
  pin = padKey(pin, k, lens(), S.cfg?.maxPinLen || 0);
  paint();
  if (k !== 'clear' && k !== 'back') say('');
  // Only the longest length goes on its own; a pause after a shorter PIN
  // never sends it (it may be the start of a longer one).
  const wait = sendDelay(pin, lens());
  if (wait == null) return;
  timer = setTimeout(sendPin, wait);
}

function sendPin() {
  const id = ++seq;
  if (!wsSend({ t: 'userUnlock', pin, id, tab: wantsTab(L) })) return say('Not connected to Sode.', true);
  say('Checking…');
}

onMsg('userUnlockResult', (m) => {
  if (m.id !== seq) return;
  if (m.ok) {
    pin = '';
    say('');
    if (m.token) { // keep the session: a tab login in this tab, a persistent user's in the browser
      if (tokenHome(m) === 'tab') writeTab(TAB_KEY, m.token);
      else writeStr(TOKEN_KEY, m.token);
    }
    return; // lockState follows
  }
  pin = '';
  const w = wrongFeedback(m.err);
  clearFeedback();
  feedback(w.text, 'bad', w.ms, true);
  say(w.note, !!w.note);
});

// A token sent outside an unlock: this user was already logged in when Never
// log out was ticked, or just changed their own PIN. Kept only while this page
// is logged in (a token arriving after Log out was revoked with it).
onMsg('session', (m) => {
  if (m.token && L.user) writeStr(TOKEN_KEY, m.token);
});

onMsg('lockState', (m) => {
  if (settle) settle(); // a welcome still showing: finish it first
  const was = (L.enforced && L.locked) || login;
  L.enforced = !!m.enforced;
  L.locked = !!m.locked;
  L.user = m.user || '';
  L.trusted = !!m.trusted;
  L.persistent = !!m.persistent;
  L.lens = m.lens || [];
  S.userNames = m.names || []; // for "by Ana" in history
  if (L.enforced && soft) { // a lock that is enforced now: the PIN replaces the hand lock
    soft.lock.cancel();
    soft = null;
  }
  // The answer to our `hi` says whether the token was honored; a state sent
  // for another reason (someone else saved settings) says nothing about it.
  // A token the server no longer honors (PIN changed, user removed or no
  // longer persistent, or revoked elsewhere) is forgotten.
  if (m.hi && !L.user) {
    if (readStr(TOKEN_KEY)) writeStr(TOKEN_KEY, null);
    if (readTab(TAB_KEY)) writeTab(TAB_KEY, null);
  }
  const done = () => {
    settle = null;
    welcoming = false;
    if (L.user || L.enforced) login = false;
    render();
    emit('access', { ...L });
    // A page that asked for something while locked asks again (log.js).
    if (was && !(L.enforced && L.locked)) emit('unlocked');
  };
  // A correct PIN: the field says who is welcome, then the unlock runs.
  if (was && !L.locked && L.user && root && !root.hidden && root.classList.contains('is-on') && !welcoming) {
    welcoming = true;
    clearTimeout(timer);
    feedback(`Welcome, ${L.user}`, 'ok');
    say('');
    const t = setTimeout(() => done(), WELCOME_MS);
    settle = () => { clearTimeout(t); done(); };
    return;
  }
  done();
});

// Settings' Account block: log out and change PIN.
on('logout', () => lockNow());

// Settings' Log in button (users exist, no lock on): the PIN pad, with Cancel.
on('login', () => {
  if (!loginOffered(L) || login || soft) return;
  login = true;
  render();
});

function cancelLogin() {
  if (!login) return;
  login = false;
  clearTimeout(timer);
  render();
}

// ---- the hand lock (no PIN lock on this page)

function softLock() {
  if (soft || L.enforced || !S.cfg) return;
  build();
  const key = churnKey(lastSoftKey);
  lastSoftKey = key;
  softKey.textContent = key.toUpperCase();
  softKey.classList.add('sode__key--locked');
  softBtn.disabled = true;
  softNote.textContent = 'Wait a moment…';
  soft = { key, lock: lockout(() => {
    softKey.classList.remove('sode__key--locked');
    softBtn.disabled = false;
    softNote.textContent = '';
  }) };
  render();
}

// Only once the lockout has passed.
function softUnlock() {
  if (!soft?.lock.ready()) return;
  soft.lock.cancel();
  soft = null;
  render();
}

on('softLock', softLock);

// ---- locking again

// Lock and Log out both end the session: the server forgets the unlock and
// revokes the token (so a reload cannot undo it), and this page forgets it.
function lockNow() {
  if (!L.user) return;
  writeStr(TOKEN_KEY, null);
  writeTab(TAB_KEY, null);
  wsSend({ t: 'userLock' });
  L.locked = L.enforced;
  L.user = '';
  L.trusted = false;
  L.persistent = false;
  render();
  emit('access', { ...L });
}

// Ctrl+L, in the capture phase so nothing underneath sees it. Only where it
// does something: a page whose lock is on, unlocked as a user.
document.addEventListener('keydown', (e) => {
  if (soft) {
    // The page behind must not see a key: not a take, not Ctrl+Z. A shortcut
    // chord is kept from the page but left to the browser (reload, copy).
    e.stopImmediatePropagation();
    if (e.ctrlKey || e.metaKey || e.altKey || e.repeat) return;
    if (e.key === 'Tab' || e.key === 'Enter' && e.target.closest?.('button')) return; // a focused button presses itself
    e.preventDefault();
    if (e.key.toLowerCase() === soft.key || (S.cfg?.confirmEnter && e.key === 'Enter')) softUnlock();
    return;
  }
  if (root && !root.hidden && root.classList.contains('is-on')) {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (login && e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      cancelLogin();
      return;
    }
    const k = /^[0-9]$/.test(e.key) ? e.key : e.key === 'Backspace' ? 'back' : e.key === 'Escape' ? 'clear' : e.key === 'Enter' ? 'enter' : null;
    if (k === 'enter' && e.target.closest?.('button')) return; // a focused key presses itself
    if (k) {
      e.preventDefault();
      e.stopPropagation();
      press(k);
    }
    return;
  }
  if (e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey && e.code === 'KeyL' && !e.repeat) {
    if (L.enforced && L.user) {
      e.preventDefault();
      e.stopPropagation();
      lockNow();
    } else if (!L.enforced && S.cfg) {
      e.preventDefault();
      e.stopPropagation();
      softLock();
    }
  }
}, true);

// ---- idle

let lastActive = Date.now();
const touch = () => { lastActive = Date.now(); };
for (const ev of ['pointerdown', 'keydown', 'wheel', 'touchstart']) document.addEventListener(ev, touch, { capture: true, passive: true });
document.addEventListener('pointermove', touch, { passive: true });
setInterval(() => {
  const minutes = S.cfg?.autoLockMinutes || 0;
  if (idleLock(L, lastActive, Date.now(), minutes)) lockNow();
  else if (S.cfg && softIdleLock(L, !!soft, lastActive, Date.now(), minutes)) softLock();
}, 5000);
