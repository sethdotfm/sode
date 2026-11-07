// The lock screen. The server decides everything (`lockState`) and refuses what a locked
// connection asks for; this module shows the pad and locks again when idle or on Ctrl+L. A
// persistent user gets a session token kept here and sent in `hi`, so a reload doesn't log them
// out; locking revokes it. Off unless the server says this page type is locked. With no PIN
// lock a hand lock (churning letter, behind the confirm lockout) can still be put up as a guard
// against keyboard mashing: page-local state, a reload clears it.

import { el, S, on, emit, onMsg, wsSend, sentence } from './core.js';
import { readStr, writeStr, readTab, writeTab } from './store.js';
import {
  padKey,
  sendDelay,
  sendable,
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
  signInAsOffered,
  signOutOffered,
  MODE,
  PAGE_VIEW,
  viewBody,
  localLock as softView,
  userLock as userLockView,
  dismissLocalLock as softUnview,
  startSignIn,
  cancelSignIn as cancelView,
  lockArrived,
  signInWon,
  chipShown,
  wantsTab,
  hiToken,
  tokenHome,
} from './lockrules.js';
import { lockout } from './lockout.js';
import { buildChip } from './lockchip.js';
import { layout, onLayout } from './layout.js';

// localStorage rather than a cookie: the socket's `hi` carries it, no HTTP endpoint is needed,
// and a cookie would be JS-readable anyway (see docs/agents/users-locks.md).
const TOKEN_KEY = 'sode-session';
// A no-lock login's token: this tab only (sessionStorage), so closing the tab logs out while a
// reload stays in.
const TAB_KEY = 'sode-tab-session';
const page = pageType(location.pathname);

{
  const css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = 'lock.css';
  document.head.append(css);
}

const L = { enforced: false, locked: false, user: '', trusted: false, persistent: false, lens: [] };
export const lockState = () => ({ ...L });

let hiFor = null;
on('conn', () => {
  if (S.wsState !== 'up' || S.ws === hiFor) return;
  hiFor = S.ws;
  wsSend({ t: 'hi', page, token: hiToken(readTab(TAB_KEY), readStr(TOKEN_KEY)) });
});

// Drop keys of the removed admin-browser sign-in.
writeStr('sode-lock-token', null);
writeStr('sode-lock-never', null);

let root = null;
let brand, dotsEl, msg, state, chip, checkKey;
let padBody, softBody, softKey, softBtn, softNote, signInAsBtn, signOutBtn, softFoot;
let unlocked = false; // a userUnlock just succeeded: the lockState that follows ends a login
let V = PAGE_VIEW;
let handOn = false;
let cancelBtn;
let soft = null;
let lastSoftKey = '';
let pin = '';
let seq = 0;
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
  padBody = el('div', 'sode-lock__body', state, dotsEl, msg, pad, cancelBtn);

  softKey = el('kbd', 'sode__key sode__key--locked');
  softBtn = el('button', 'pb-btn pb-btn--ok', 'Dismiss');
  softBtn.type = 'button';
  softBtn.addEventListener('click', dismissLocalLock);
  softNote = el('p', 'sode-lock__msg');
  softNote.setAttribute('role', 'status');
  softNote.setAttribute('aria-live', 'polite');
  signInAsBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Sign in as…');
  signInAsBtn.type = 'button';
  signInAsBtn.hidden = true;
  signInAsBtn.addEventListener('click', signInFromLock);
  signOutBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Sign out');
  signOutBtn.type = 'button';
  signOutBtn.hidden = true;
  signOutBtn.addEventListener('click', signOutFromFoot);
  softBody = el('div', 'sode-lock__body', el('p', 'sode-lock__soft-line', 'Press ', softKey, ' to dismiss'), softBtn, softNote);
  softBody.hidden = true;
  softFoot = el('div', 'sode-lock__foot', signOutBtn, signInAsBtn);
  softFoot.hidden = true;
  // The foot is the last row, in flow, so it can't land on the pad.
  root = el('div', 'sode-lock', el('div', 'sode-lock__main', brand, padBody, softBody), softFoot);
  root.hidden = true;
  root.tabIndex = -1;
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'true');
  root.setAttribute('aria-label', 'Locked');
  document.body.append(root);

  chip = buildChip({ lock: () => (L.enforced ? lockNow() : localLock()) });
  document.body.append(chip.root);
}

function say(text, bad = false) {
  if (!root) return;
  msg.textContent = text;
  msg.className = `sode-lock__msg${bad ? ' pb-note pb-note--error' : ''}`;
}

function render() {
  const covered = viewBody(V, L.enforced && L.locked) !== '';
  const showChip = chipShown({ covered, soft: V.mode === MODE.SOFT, user: L.user });
  if (covered) showLock();
  else hideLock();
  build();
  chip.update(L, showChip, V.mode === MODE.SOFT);
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

const logo = () => [...document.querySelectorAll('.sode__mark, .sode-m__mark, .sode-log__title')].find((e) => e.getBoundingClientRect().width > 0);
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
  const body = viewBody(V, L.enforced && L.locked);
  padBody.hidden = body !== 'pad';
  cancelBtn.hidden = V.mode !== MODE.SIGN_IN;
  const ownLock = V.mode === MODE.OWN;
  softBody.hidden = body !== 'soft';
  signOutBtn.hidden = !(ownLock || signOutOffered(L));
  signInAsBtn.hidden = ownLock || !signInAsOffered(L);
  softFoot.hidden = !ownLock && (body !== 'soft' || (signOutBtn.hidden && signInAsBtn.hidden));
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
  root.focus({ preventScroll: true });
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
  if (handOn === on) return;
  handOn = on;
  emit('handlock', { on });
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
  state.textContent = pin ? `${pin.length} ${pin.length === 1 ? 'digit' : 'digits'} entered` : 'Please enter your PIN.';
  checkKey.disabled = !sendable(pin, lens());
}

const lens = () => L.lens;

function press(k) {
  clearTimeout(timer);
  if (welcoming) return;
  if (dotsEl.classList.contains('is-text')) clearFeedback();
  if (k === 'enter') {
    if (sendable(pin, lens())) sendPin();
    else say(pin ? 'No PIN has that many digits.' : 'Enter your PIN.', true);
    return;
  }
  pin = padKey(pin, k, lens(), S.cfg?.maxPinLen || 0);
  paint();
  if (k !== 'clear' && k !== 'back') say('');
  // Only the longest length goes on its own: a pause after a shorter PIN may be the start of a
  // longer one.
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
    unlocked = true;
    say('');
    if (m.token) {
      if (tokenHome(m) === 'tab') writeTab(TAB_KEY, m.token);
      else writeStr(TOKEN_KEY, m.token);
    }
    return;
  }
  pin = '';
  const w = wrongFeedback(m.err);
  clearFeedback();
  feedback(w.text, 'bad', w.ms, true);
  say(w.note, !!w.note);
});

// A token sent outside an unlock (user already logged in when Never log out was ticked, or
// changed their own PIN). Kept only while this page is logged in.
onMsg('session', (m) => {
  if (m.token && L.user) writeStr(TOKEN_KEY, m.token);
});

onMsg('lockState', (m) => {
  if (settle) settle();
  const was = (L.enforced && L.locked) || V.mode === MODE.SIGN_IN || V.mode === MODE.OWN;
  L.enforced = !!m.enforced;
  L.locked = !!m.locked;
  L.user = m.user || '';
  L.trusted = !!m.trusted;
  L.persistent = !!m.persistent;
  L.lens = m.lens || [];
  S.userNames = m.names || [];
  if (L.enforced && soft) {
    soft.lock.cancel();
    soft = null;
  }
  V = lockArrived(V, L.enforced);
  // Only the answer to our `hi` says whether the token was honored; a state sent for another
  // reason says nothing. A token the server no longer honors is forgotten.
  if (m.hi && !L.user) {
    if (readStr(TOKEN_KEY)) writeStr(TOKEN_KEY, null);
    if (readTab(TAB_KEY)) writeTab(TAB_KEY, null);
  }
  const done = () => {
    settle = null;
    welcoming = false;
    // A login that worked ends the hand lock for good; other states say nothing about this pad.
    V = signInWon(V, { enforced: L.enforced, unlocked, user: L.user });
    unlocked = false;
    render();
    emit('access', { ...L });
    if (was && !(L.enforced && L.locked)) emit('unlocked');
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

on('logout', () => lockNow());

on('login', () => {
  if (!signInOffered(L) || V.mode !== MODE.PAGE) return;
  V = startSignIn(V, false);
  render();
});

function cancelSignIn() {
  if (V.mode !== MODE.SIGN_IN) return;
  const back = V.back === 'soft';
  V = cancelView(V);
  clearTimeout(timer);
  clearFeedback();
  if (back && !L.enforced) localLock();
  else render();
}

function localLock() {
  if (soft || L.enforced || !S.cfg || V.mode === MODE.OWN) return;
  if (L.user) return pinLock();
  build();
  V = softView(V, false);
  const key = churnKey(lastSoftKey);
  lastSoftKey = key;
  softKey.textContent = key.toUpperCase();
  softKey.classList.add('sode__key--locked');
  softBtn.disabled = true;
  softNote.textContent = 'Wait a moment…';
  soft = {
    key,
    lock: lockout(() => {
      softKey.classList.remove('sode__key--locked');
      softBtn.disabled = false;
      softNote.textContent = '';
    }),
  };
  render();
}

// Lock while logged in with no PIN lock: the session ends on the server (as Log out) and the
// PIN pad covers the page with no Cancel. A reload shows nobody logged in.
function pinLock() {
  build();
  V = userLockView(V, false);
  pin = '';
  clearTimeout(timer);
  clearFeedback();
  lockNow();
}

function dismissLocalLock() {
  if (!soft?.lock.ready()) return;
  soft.lock.cancel();
  soft = null;
  V = softUnview();
  render();
}

// "Log in as...": ends the hand lock and begins a login. Cancel returns to the hand lock. The
// hand lock never sends userLock, so a tab login stays in until this one succeeds.
function signInFromLock() {
  if (!soft || !signInAsOffered(L)) return;
  soft.lock.cancel();
  soft = null;
  V = startSignIn(V, true);
  pin = '';
  clearTimeout(timer);
  clearFeedback();
  say('');
  render();
}

// Under the hand lock the session ends (token revoked) and the hand lock stays up. On a
// logged-in user's own lock the session already ended: the pad just goes away.
function signOutFromFoot() {
  if (V.mode === MODE.OWN) {
    V = PAGE_VIEW;
    clearTimeout(timer);
    clearFeedback();
    render();
  } else if (V.mode === MODE.SOFT && signOutOffered(L)) lockNow();
}

on('localLock', localLock);

// Lock and Log out both end the session: the server forgets the unlock and revokes the token
// (so a reload cannot undo it).
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

// Capture phase so nothing underneath sees it. Only where it does something.
document.addEventListener(
  'keydown',
  (e) => {
    if (V.mode === MODE.SOFT) {
      // The page behind must not see a key (not a take, not Ctrl+Z); a shortcut chord is kept
      // from the page but left to the browser.
      e.stopImmediatePropagation();
      if (e.ctrlKey || e.metaKey || e.altKey || e.repeat) return;
      if (e.key === 'Tab' || ((e.key === 'Enter' || e.key === ' ') && e.target.closest?.('button'))) return;
      e.preventDefault();
      if (e.key.toLowerCase() === soft.key || (S.cfg?.confirmEnter && e.key === 'Enter')) dismissLocalLock();
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
      if (L.enforced && L.user) {
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

let lastActive = Date.now();
const touch = () => {
  lastActive = Date.now();
};
for (const ev of ['pointerdown', 'keydown', 'wheel', 'touchstart']) document.addEventListener(ev, touch, { capture: true, passive: true });
document.addEventListener('pointermove', touch, { passive: true });
setInterval(() => {
  const minutes = S.cfg?.autoLockMinutes || 0;
  if (idleLock(L, lastActive, Date.now(), minutes)) lockNow();
  else if (S.cfg && localIdleLock(L, !!soft, lastActive, Date.now(), minutes)) localLock();
}, 5000);
