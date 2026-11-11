// The lock screen. The server decides everything (`lockState`) and refuses what a locked
// connection asks for; this module shows the pad and locks again when idle or on Ctrl+L. A
// persistent user gets a session token kept here and sent in `hi`, so a reload doesn't log them
// out; the Lock keeps it, signing out revokes it. Off unless the server says this page type is locked. With no PIN
// lock a Lock with nobody signed in (churning letter, behind the confirm lockout) can still be put up as a guard
// against keyboard mashing: page-local state, a reload clears it.

import { el, S, on, emit, onMsg, wsSend, sentence } from './core.js';
import { heldUserKept, keepHeldUser, lastUser, keepLastUser, startHandshake, keepSignInToken, forgetTokens, forgetSignIn, onSessionToken } from './signin.js';
import {
  padKey,
  idleLock,
  pageType,
  liftFrom,
  shouldFly,
  brandFrames,
  OUT_FACTOR,
  dots,
  wrongFeedback,
  WELCOME_MS,
  churnKey,
  localIdleLock,
  signInOffered,
  asksUsername,
  canSend,
  padDelay,
  signInMsg,
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
  pinFreeView,
} from './lockrules.js';
import { lockout } from './lockout.js';
import { createOwed } from './owed.js';
import { createIdleShare } from './idleshare.js';
import { buildChip } from './lockchip.js';
import { layout, onLayout } from './layout.js';

let heldUser = heldUserKept();
const page = pageType(location.pathname);

{
  const css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = 'lock.css';
  document.head.append(css);
}

const L = { enforced: false, locked: false, held: false, user: '', trusted: false, persistent: false, noAutoLock: false, lens: [], mode: 'pin' };
export const lockState = () => ({ ...L });

startHandshake(page);
onSessionToken(() => !!L.user);

let root = null;
let brand, dotsEl, msg, state, chip, checkKey;
let nameField, nameInput;
let padBody, letterBody, letterKey, letterBtn, letterNote, signInAsBtn, signOutBtn, letterFoot;
const owed = createOwed(wsSend); // a Lock the server has not got yet
let unlocked = false; // a userUnlock just succeeded: the lockState that follows ends a sign-in
let V = PAGE_VIEW;
let covering = false;
let cancelBtn;
let letterLock = null;
let lastLetter = '';
let pin = '';
let seq = 0;
let timer = 0;
let inerted = [];
let hideTimer = 0;
let pinFree = false; // a Public panel's page, told by panelrun.js
let lastState = null;

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
  cancelBtn.addEventListener('click', cancelSignIn);
  nameInput = el('input', 'pb-input sode-lock__name');
  nameInput.id = 'lock-username';
  nameInput.type = 'text';
  nameInput.autocomplete = 'username';
  nameInput.autocapitalize = 'none';
  nameInput.spellcheck = false;
  nameInput.maxLength = 24;
  nameInput.addEventListener('input', () => {
    if (dotsEl.classList.contains('is-text')) clearFeedback();
    paint();
    clearTimeout(timer);
    const wait = padDelay(L, pin, nameInput.value);
    if (wait != null && pin) timer = setTimeout(sendPin, wait);
  });
  nameField = el('div', 'sode-lock__namefield', Object.assign(el('label', 'sode-lock__namelabel', 'Username'), { htmlFor: 'lock-username' }), nameInput);
  nameField.hidden = true;
  padBody = el('div', 'sode-lock__body', nameField, state, dotsEl, msg, pad, cancelBtn);

  letterKey = el('kbd', 'sode__key sode__key--locked');
  letterBtn = el('button', 'pb-btn pb-btn--ok', 'Dismiss');
  letterBtn.type = 'button';
  letterBtn.addEventListener('click', dismissLocalLock);
  letterNote = el('p', 'sode-lock__msg');
  letterNote.setAttribute('role', 'status');
  letterNote.setAttribute('aria-live', 'polite');
  signInAsBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Sign in as…');
  signInAsBtn.type = 'button';
  signInAsBtn.hidden = true;
  signInAsBtn.addEventListener('click', signInFromLock);
  signOutBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Sign out');
  signOutBtn.type = 'button';
  signOutBtn.hidden = true;
  signOutBtn.addEventListener('click', signOutFromFoot);
  letterBody = el('div', 'sode-lock__body', el('p', 'sode-lock__letter-line', 'Press ', letterKey, ' to dismiss'), letterBtn, letterNote);
  letterBody.hidden = true;
  letterFoot = el('div', 'sode-lock__foot', signOutBtn, signInAsBtn);
  letterFoot.hidden = true;
  // The foot is the last row, in flow, so it can't land on the pad.
  root = el('div', 'sode-lock', el('div', 'sode-lock__main', brand, padBody, letterBody), letterFoot);
  root.hidden = true;
  root.tabIndex = -1;
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'true');
  root.setAttribute('aria-label', 'Locked');
  document.body.append(root);

  chip = buildChip({ lock: localLock });
  document.body.append(chip.root);
}

function say(text, bad = false) {
  if (!root) return;
  msg.textContent = text;
  msg.className = `sode-lock__msg${bad ? ' pb-note pb-note--error' : ''}`;
}

function render() {
  const covered = viewBody(V, lockCovers(L)) !== '';
  const who = L.held ? heldUser : '';
  const showChip = chipShown({ covered, letter: V.mode === MODE.LOCK, user: L.user, heldUser: who });
  if (covered) showLock();
  else hideLock();
  build();
  chip.update(L, showChip, V.mode === MODE.LOCK || L.held, L.user || who);
  document.documentElement.toggleAttribute('data-sode-chip', !!showChip);
  if (showChip) placeChip();
}

function placeChip() {
  const head = document.querySelector('.sode-m__head');
  const b = head?.getBoundingClientRect();
  if (b?.width) document.documentElement.style.setProperty('--sode-head-h', `${b.bottom}px`);
}
addEventListener('resize', () => {
  if (chip && !chip.root.hidden) placeChip();
});

const logo = () =>
  [...document.querySelectorAll('.sode__mark, .sode-m__mark, .sode-log__title, .sode-panels__title')].find((e) => e.getBoundingClientRect().width > 0);
const calmMotion = matchMedia('(prefers-reduced-motion: reduce)');
const motionMs = () => (calmMotion.matches ? 0 : parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--sode-lock-motion')) || 0);

// Every run starts from the brand at rest with its own keyframes, so it restarts reliably (a
// CSS transition measured while the last run was going did not). Flies only when shouldFly,
// else the logo fades with the overlay.
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

let scrolled = null;

function showLock() {
  build();
  const body = viewBody(V, lockCovers(L));
  padBody.hidden = body !== 'pad';
  cancelBtn.hidden = V.mode !== MODE.SIGN_IN;
  nameField.hidden = !asksUsername(L);
  letterBody.hidden = body !== 'letter';
  signOutBtn.hidden = !(L.held || signOutOffered(L));
  signInAsBtn.hidden = L.held || !signInAsOffered(L);
  letterFoot.hidden = !L.held && (body !== 'letter' || (signOutBtn.hidden && signInAsBtn.hidden));
  root.setAttribute('aria-label', 'Locked');
  clearTimeout(hideTimer);
  if (!root.hidden && root.classList.contains('is-on')) return;
  pin = '';
  nameInput.value = prefillName(L, lastUser());
  welcoming = false;
  clearFeedback();
  say('');
  const text = S.cfg?.brandText?.trim() || 'Sode';
  brand.textContent = text;
  const frame = document.querySelector('.sode__brandframe');
  if (frame?.dataset.brand) root.dataset.brand = frame.dataset.brand;
  else delete root.dataset.brand;
  // The page keeps its scroll; focus moves must not scroll it (desktop.js leaves its
  // focus-scroll alone while this is set).
  if (root.hidden) scrolled = { x: window.scrollX, y: window.scrollY };
  document.documentElement.dataset.sodeLock = 'on';
  setHand(true);
  root.hidden = false;
  fitBrand();
  moveBrand('in');
  void root.offsetWidth;
  root.classList.add('is-on');
  document.documentElement.dataset.sodeLockOn = 'on';
  chip?.setLocked(true);
  if (V.mode !== MODE.SIGN_IN) closeModals();
  inerted = [];
  for (const c of document.body.children) inert(c);
  watch ||= new MutationObserver((list) => {
    for (const r of list) for (const n of r.addedNodes) inert(n);
  });
  watch.observe(document.body, { childList: true });
  const had = document.activeElement;
  if (had && had !== document.body && !root.contains(had)) before = had;
  had?.blur?.();
  if (focusName(L, nameValue()) && V.mode !== MODE.LOCK) nameInput.focus({ preventScroll: true });
  else root.focus({ preventScroll: true });
}

// A site name wider than the screen scales down to one line (under 1.25rem it wraps instead).
// Phone only.
function fitBrand() {
  brand.style.fontSize = '';
  brand.style.whiteSpace = '';
  if (layout() !== 'phone') return;
  const cs = getComputedStyle(root);
  const room = root.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
  brand.style.whiteSpace = 'nowrap';
  const w = brand.getBoundingClientRect().width;
  if (w <= room) return;
  const size = (parseFloat(getComputedStyle(brand).fontSize) * (room - 1)) / w;
  const min = 1.25 * parseFloat(getComputedStyle(document.documentElement).fontSize);
  if (size >= min) brand.style.fontSize = `${size}px`;
  else {
    brand.style.whiteSpace = '';
    brand.style.fontSize = `${min}px`;
  }
}
const refit = () => {
  if (root && !root.hidden) fitBrand();
};
addEventListener('resize', refit);
onLayout(refit);

function setHand(on) {
  if (covering === on) return;
  covering = on;
  emit('lockcover', { on });
}

let watch = null;
let before = null;
function inert(c) {
  if (c.nodeType !== 1 || c === root || c === chip?.root || ['SCRIPT', 'LINK', 'STYLE'].includes(c.tagName) || c.inert) return;
  c.inert = true;
  inerted.push(c);
}

// Native <dialog>s close here; a module that owns a dialog or view listens for 'locking'
// (Settings exits; an unsaved form keeps its warning, covered).
function closeModals() {
  for (const d of document.querySelectorAll('dialog[open]')) d.close();
  emit('locking');
  // Fallback for modals with no 'locking' listener yet: take their own Cancel/Close, else hide
  // them.
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
  delete document.documentElement.dataset.sodeLockOn;
  chip?.setLocked(false);
  pin = '';
  // The way out is slower (--sode-lock-out in lock.css = OUT_FACTOR); the root is hidden once
  // the longest transition is over.
  const ms = motionMs();
  const out = ms * OUT_FACTOR;
  if (wasOn) moveBrand('out');
  clearTimeout(hideTimer);
  hideTimer = setTimeout(() => {
    root.hidden = true;
    for (const a of brand.getAnimations()) a.cancel();
    delete document.documentElement.dataset.sodeLock;
    setHand(false);
  }, out + 100);
  if (before?.isConnected) before.focus?.({ preventScroll: true });
  before = null;
  if (scrolled && (window.scrollX !== scrolled.x || window.scrollY !== scrolled.y)) window.scrollTo({ left: scrolled.x, top: scrolled.y, behavior: 'instant' });
}

let fbTimer = 0;
let welcoming = false;
let settle = null;

// Fade in; a wrong PIN also shakes (restarts each time). Reduced motion: only the text.
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
        [
          { transform: 'translateX(0)' },
          { transform: 'translateX(-0.6rem)' },
          { transform: 'translateX(0.6rem)' },
          { transform: 'translateX(-0.4rem)' },
          { transform: 'translateX(0.4rem)' },
          { transform: 'translateX(0)' },
        ],
        { duration: 400, easing: 'ease-in-out' },
      );
    }
  }
  if (ms)
    fbTimer = setTimeout(() => {
      clearFeedback();
      paint();
    }, ms);
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

function paint() {
  dotsEl.textContent = dots(pin);
  state.textContent = pin ? `${pin.length} ${pin.length === 1 ? 'digit' : 'digits'} entered` : '';
  checkKey.disabled = !canSend(L, pin, nameValue());
}

const lens = () => L.lens;
const nameValue = () => nameInput?.value ?? '';

function press(k) {
  clearTimeout(timer);
  if (welcoming) return;
  if (dotsEl.classList.contains('is-text')) clearFeedback();
  if (k === 'enter') {
    if (canSend(L, pin, nameValue())) sendPin();
    else if (asksUsername(L) && !nameValue().trim()) say('Enter your username.', true);
    else say(pin ? 'No PIN has that many digits.' : 'Enter your PIN.', true);
    return;
  }
  pin = padKey(pin, k, lens(), S.cfg?.maxPinLen || 0);
  paint();
  if (k !== 'clear' && k !== 'back') say('');
  // Only the longest length goes on its own: a pause after a shorter PIN may be the start of a
  // longer one.
  const wait = padDelay(L, pin, nameValue());
  if (wait == null) return;
  timer = setTimeout(sendPin, wait);
}

function sendPin() {
  const id = ++seq;
  if (!wsSend(signInMsg(L, { pin, id, username: nameValue() }))) return say('Not connected to Sode.', true);
  say('Checking…');
}

onMsg('userUnlockResult', (m) => {
  if (m.id !== seq) return;
  if (m.ok) {
    if (asksUsername(L) && remembers(m)) keepLastUser(nameValue().trim());
    pin = '';
    unlocked = true;
    say('');
    keepSignInToken(m);
    return;
  }
  pin = '';
  const w = wrongFeedback(m.err);
  clearFeedback();
  feedback(w.text, 'bad', w.ms, true);
  say(w.note, !!w.note);
});

onMsg('lockState', (m) => {
  if (owed.arrived(m)) return;
  if (settle) settle();
  const was = lockCovers(L) || V.mode === MODE.SIGN_IN;
  const ended = heldEnded(L, m);
  heldUser = heldWho(heldUser, L, m);
  keepHeldUser(heldUser);
  if (ended) keepLastUser(null);
  lastState = m;
  Object.assign(L, pinFreeView(m, pinFree));
  L.held = !!m.held;
  L.user = m.user || '';
  L.trusted = !!m.trusted;
  L.persistent = !!m.persistent;
  L.noAutoLock = !!m.noAutoLock;
  L.lens = m.lens || [];
  L.mode = m.mode === 'username' ? 'username' : 'pin';
  S.userNames = m.names || [];
  if (lockCovers(L) && letterLock) {
    letterLock.lock.cancel();
    letterLock = null;
  }
  V = lockArrived(V, lockCovers(L));
  // The Lock ended without this tab's PIN: it stays covered, as when signing out of the letter Lock.
  if (ended && !L.enforced) localLock();
  // Only the answer to our `hi` says whether the token was honored; a state sent for another
  // reason says nothing. A token the server no longer honors is forgotten.
  if (tokenStale(m, L.user)) forgetTokens();
  const done = () => {
    settle = null;
    welcoming = false;
    // A sign-in that worked ends the Lock with nobody signed in for good; other states say nothing about this pad.
    V = signInWon(V, { enforced: L.enforced, unlocked, user: L.user });
    unlocked = false;
    render();
    emit('access', { ...L });
    if (was && !lockCovers(L)) emit('unlocked');
  };
  if (was && !L.locked && L.user && root && !root.hidden && root.classList.contains('is-on') && !welcoming) {
    welcoming = true;
    clearTimeout(timer);
    feedback(`Welcome, ${L.user}`, 'ok');
    say('');
    const t = setTimeout(() => done(), WELCOME_MS);
    settle = () => {
      clearTimeout(t);
      done();
    };
    return;
  }
  done();
});

// The panel page says whether it runs as a Public panel; the last lockState is read again.
on('publicPanel', ({ on: free }) => {
  if (pinFree === !!free) return;
  pinFree = !!free;
  if (!lastState) return;
  Object.assign(L, pinFreeView(lastState, pinFree));
  if (lockCovers(L) && letterLock) {
    letterLock.lock.cancel();
    letterLock = null;
  }
  V = lockArrived(V, lockCovers(L));
  render();
  emit('access', { ...L });
});

on('logout', () => signOutNow());

on('login', () => {
  if (!signInOffered(L) || V.mode !== MODE.PAGE) return;
  V = startSignIn(V, false);
  render();
});

function cancelSignIn() {
  if (V.mode !== MODE.SIGN_IN) return;
  const back = V.back === 'lock';
  V = signInCancelled(V);
  clearTimeout(timer);
  clearFeedback();
  if (back && !L.enforced) localLock();
  else render();
}

function localLock() {
  if (L.user) return lockNow();
  if (letterLock || L.enforced || lockCovers(L) || !S.cfg) return;
  build();
  V = lockView(V, false);
  const key = churnKey(lastLetter);
  lastLetter = key;
  letterKey.textContent = key.toUpperCase();
  letterKey.classList.add('sode__key--locked');
  letterBtn.disabled = true;
  letterNote.textContent = 'Wait a moment…';
  letterLock = {
    key,
    lock: lockout(() => {
      letterKey.classList.remove('sode__key--locked');
      letterBtn.disabled = false;
      letterNote.textContent = '';
    }),
  };
  render();
}

function dismissLocalLock() {
  if (!letterLock?.lock.ready()) return;
  letterLock.lock.cancel();
  letterLock = null;
  V = lockDismissed();
  render();
}

// "Sign in as…": ends the Lock with nobody signed in and begins a sign-in. Cancel returns to
// it. That Lock sends nothing to the server, so a tab sign-in stays in until this one succeeds.
function signInFromLock() {
  if (!letterLock || !signInAsOffered(L)) return;
  letterLock.lock.cancel();
  letterLock = null;
  V = startSignIn(V, true);
  nameInput.value ||= prefillName(L, lastUser());
  pin = '';
  clearTimeout(timer);
  clearFeedback();
  say('');
  render();
}

// From the held Lock, or under the Lock with nobody signed in, with someone signed in: the session ends.
function signOutFromFoot() {
  if (L.held || (V.mode === MODE.LOCK && signOutOffered(L))) signOutNow();
}

on('localLock', localLock);

// The Lock keeps the session: the server holds it, so a reload or another tab shows the pad
// until this user's PIN resumes it. The token stays for that.
function lockNow() {
  if (!L.user) return;
  owed.owe();
  pin = '';
  clearTimeout(timer);
  heldUser = L.user;
  keepHeldUser(heldUser);
  Object.assign(L, raiseLock(L));
  render();
  emit('access', { ...L });
}

// Sign out ends the session: the server forgets the unlock and revokes the token (so a
// reload cannot undo it).
function signOutNow() {
  if (!L.user && !L.held) return;
  forgetSignIn();
  heldUser = '';
  wsSend({ t: 'userSignOut' });
  pin = '';
  clearTimeout(timer);
  clearFeedback();
  const wasHeld = L.held;
  Object.assign(L, signedOut(L));
  if (wasHeld && !L.enforced) localLock();
  else render();
  emit('access', { ...L });
}

// Capture phase so nothing underneath sees it. Only where it does something.
document.addEventListener(
  'keydown',
  (e) => {
    if (V.mode === MODE.LOCK) {
      // The page behind must not see a key (not a take, not Ctrl+Z); a shortcut chord is kept
      // from the page but left to the browser.
      e.stopImmediatePropagation();
      if (e.ctrlKey || e.metaKey || e.altKey || e.repeat) return;
      if (e.key === 'Tab' || ((e.key === 'Enter' || e.key === ' ') && e.target.closest?.('button'))) return;
      e.preventDefault();
      if (e.key.toLowerCase() === letterLock.key || (S.cfg?.confirmEnter && e.key === 'Enter')) dismissLocalLock();
      return;
    }
    if (root && !root.hidden && root.classList.contains('is-on')) {
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      if (V.mode === MODE.SIGN_IN && e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        cancelSignIn();
        return;
      }
      if (e.target === nameInput) {
        // The pad's keys must not eat what is typed into the username; Enter moves on to the PIN.
        if (e.key === 'Enter') {
          e.preventDefault();
          e.stopPropagation();
          if (pin && canSend(L, pin, nameValue())) sendPin();
          else root.focus({ preventScroll: true });
        }
        return;
      }
      const k = /^[0-9]$/.test(e.key) ? e.key : e.key === 'Backspace' ? 'back' : e.key === 'Escape' ? 'clear' : e.key === 'Enter' ? 'enter' : null;
      if (k === 'enter' && e.target.closest?.('button')) return;
      if (k) {
        e.preventDefault();
        e.stopPropagation();
        press(k);
      }
      return;
    }
    if (e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey && e.code === 'KeyL' && !e.repeat) {
      if (L.user) {
        e.preventDefault();
        e.stopPropagation();
        lockNow();
      } else if (!L.enforced && S.cfg) {
        e.preventDefault();
        e.stopPropagation();
        localLock();
      }
    }
  },
  true,
);

const idle = createIdleShare(typeof BroadcastChannel === 'function' ? new BroadcastChannel('sode-activity') : null);
const touch = idle.touch;
for (const ev of ['pointerdown', 'keydown', 'wheel', 'touchstart']) document.addEventListener(ev, touch, { capture: true, passive: true });
document.addEventListener('pointermove', touch, { passive: true });
setInterval(() => {
  const minutes = S.cfg?.autoLockMinutes || 0;
  if (idleLock(L, idle.lastActive(), Date.now(), minutes)) lockNow();
  else if (S.cfg && localIdleLock(L, !!letterLock, idle.lastActive(), Date.now(), minutes)) localLock();
}, 5000);
