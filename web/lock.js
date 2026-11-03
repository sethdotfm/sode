// The lock screen (#151). The server decides everything: it tells this page
// whether the page type is locked and who (if anyone) unlocked it
// (`lockState`), and refuses what a locked connection asks for. This module
// shows a full-viewport pad that sends the PIN on its own at the longest
// length in use (shorter ones with the check key or Enter), locks again when
// idle or on Ctrl+L, and lets the admin password (the settings PIN) make this
// browser exempt via a server-issued token. An exempt browser can still lock
// itself: the server then lets its token rest on that connection.
//
// Off unless the server says this page type is locked: with no users and no
// locks nothing here shows or is bound but a few idle listeners.

import { el, S, on, emit, onMsg, wsSend, sentence } from './core.js';
import { readStr, writeStr } from './store.js';
import { padKey, sendDelay, sendable, idleLock, pageType, liftFrom, shouldFly, brandFrames, OUT_FACTOR, dots } from './lockrules.js';
import { buildChip } from './lockchip.js';

const TOKEN_KEY = 'sode-lock-token';
// Per browser, set by an admin: skip the idle timer (never a manual lock).
const NEVER_KEY = 'sode-lock-never';
const page = pageType(location.pathname);

{
  const css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = 'lock.css';
  document.head.append(css);
}

// What the server last said about this connection.
const L = { enforced: false, locked: false, user: '', exempt: false, token: false };
export const lockState = () => ({ ...L });

let hiFor = null; // the socket `hi` was sent on
let hiPending = false;
on('conn', () => {
  if (S.wsState !== 'up' || S.ws === hiFor) return;
  hiFor = S.ws;
  hiPending = true;
  wsSend({ t: 'hi', page, token: readStr(TOKEN_KEY) || '' });
});

// ---- the overlay, built the first time it is needed

let root = null;
let brand, dotsEl, msg, state, adminForm, adminInput, adminToggle, welcome, chip, checkKey;
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

  adminToggle = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Admin password');
  adminToggle.type = 'button';
  adminInput = el('input', 'pb-input');
  adminInput.type = 'password';
  adminInput.autocomplete = 'off';
  adminInput.spellcheck = false;
  adminInput.setAttribute('aria-label', 'Admin password');
  const go = el('button', 'pb-btn pb-btn--ok pb-btn--sm', 'Unlock this browser');
  go.type = 'submit';
  adminForm = el('form', 'sode-lock__admin', adminInput, go);
  adminForm.hidden = true;
  adminToggle.addEventListener('click', () => {
    adminForm.hidden = !adminForm.hidden;
    adminToggle.textContent = adminForm.hidden ? 'Admin password' : 'Use a PIN';
    if (!adminForm.hidden) adminInput.focus();
    else root.focus();
    say('');
  });
  adminForm.addEventListener('submit', (e) => {
    e.preventDefault();
    if (!adminInput.value) return;
    const id = ++seq;
    if (!wsSend({ t: 'adminUnlock', pin: adminInput.value, id })) return say('Not connected to Sode.', true);
    say('Checking…');
  });

  const body = el('div', 'sode-lock__body', state, dotsEl, msg, pad, adminToggle, adminForm);
  welcome = el('p', 'sode-lock__welcome');
  welcome.hidden = true;
  root = el('div', 'sode-lock', brand, body, welcome);
  root.hidden = true;
  root.tabIndex = -1;
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'true');
  root.setAttribute('aria-label', 'Locked');
  document.body.append(root);

  // Who is logged in, with the way to lock and a small options menu.
  chip = buildChip({
    lock: lockNow,
    forget: forgetBrowser,
    never: () => !!readStr(NEVER_KEY),
    setNever: (on) => writeStr(NEVER_KEY, on ? 'on' : null),
  });
  document.body.append(chip.root);
}

function say(text, bad = false) {
  if (!root) return;
  msg.textContent = text;
  msg.className = `sode-lock__msg${bad ? ' pb-note pb-note--error' : ''}`;
}

function render() {
  const showChip = L.enforced && !L.locked && (L.user || L.exempt);
  if (!root && !L.locked && !showChip) return; // nothing configured: nothing built
  const welcomeName = L.user || (L.exempt ? 'admin' : '');
  if (L.enforced && L.locked) showLock();
  else hideLock(welcomeName);
  build();
  chip.update(L, showChip);
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
  const fly = shouldFly(from, window.scrollY, window.innerHeight);
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
  clearTimeout(hideTimer);
  if (!root.hidden && root.classList.contains('is-on')) return;
  pin = '';
  paint();
  say('');
  const text = S.cfg?.brandText?.trim() || 'Sode';
  brand.textContent = text;
  const frame = document.querySelector('.sode__brandframe');
  if (frame?.dataset.brand) root.dataset.brand = frame.dataset.brand;
  else delete root.dataset.brand;
  for (const a of welcome.getAnimations()) a.cancel();
  welcome.hidden = true;
  // The page keeps its scroll through lock and unlock; focus moves must not
  // scroll it (desktop.js leaves its focus-scroll alone while this is set).
  if (root.hidden) scrolled = { x: window.scrollX, y: window.scrollY };
  document.documentElement.dataset.sodeLock = 'on';
  root.hidden = false;
  moveBrand('in');
  void root.offsetWidth; // commit the start before the class changes it
  root.classList.add('is-on');
  closeModals();
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

function hideLock(name = '') {
  if (!root || root.hidden) return;
  watch?.disconnect();
  clearTimeout(timer);
  for (const c of inerted) c.inert = false;
  inerted = [];
  const wasOn = root.classList.contains('is-on');
  root.classList.remove('is-on');
  adminForm.hidden = true;
  adminInput.value = '';
  adminToggle.textContent = 'Admin password';
  pin = '';
  // The way out is slower than the way in (--sode-lock-out in lock.css is
  // the same OUT_FACTOR), so the last of it fades gently; the root is hidden
  // only once the longest of its transitions is over.
  const ms = motionMs();
  const out = ms * OUT_FACTOR;
  if (wasOn) {
    moveBrand('out');
    if (ms && name && welcome.animate) {
      welcome.textContent = `Welcome, ${name}`;
      welcome.hidden = false;
      welcome.animate(
        [{ opacity: 0, transform: 'translateY(0.5rem)' }, { opacity: 1, transform: 'none', offset: 0.3 }, { opacity: 1, offset: 0.65 }, { opacity: 0, offset: 1 }],
        { duration: out, easing: 'ease-out', fill: 'forwards' });
    }
  }
  clearTimeout(hideTimer);
  hideTimer = setTimeout(() => {
    root.hidden = true;
    for (const a of brand.getAnimations()) a.cancel();
    for (const a of welcome.getAnimations()) a.cancel();
    welcome.hidden = true;
    delete document.documentElement.dataset.sodeLock;
  }, out + 100);
  // The element that had focus before locking, if it is still on the page.
  if (before?.isConnected) before.focus?.({ preventScroll: true });
  before = null;
  if (scrolled && (window.scrollX !== scrolled.x || window.scrollY !== scrolled.y)) window.scrollTo({ left: scrolled.x, top: scrolled.y, behavior: 'instant' });
}

// ---- entering a PIN

function paint() {
  dotsEl.textContent = dots(pin);
  state.textContent = pin ? `${pin.length} ${pin.length === 1 ? 'digit' : 'digits'} entered` : 'Locked. Enter your PIN.';
  checkKey.disabled = !sendable(pin, lens());
}

const lens = () => [...new Set((S.cfg?.users || []).map((u) => u.len))];

function press(k) {
  clearTimeout(timer);
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
  if (!wsSend({ t: 'userUnlock', pin, id })) return say('Not connected to Sode.', true);
  say('Checking…');
}

onMsg('userUnlockResult', (m) => {
  if (m.id !== seq) return;
  if (m.ok) {
    pin = '';
    say('');
    return; // lockState follows
  }
  pin = '';
  paint();
  say(sentence(m.err), true);
});

onMsg('adminUnlockResult', (m) => {
  if (m.id !== seq) return;
  if (m.ok && m.token) {
    writeStr(TOKEN_KEY, m.token);
    say('');
    return;
  }
  adminInput.select();
  say(sentence(m.err || 'Wrong password.'), true);
});

onMsg('lockState', (m) => {
  const was = L.enforced && L.locked;
  L.enforced = !!m.enforced;
  L.locked = !!m.locked;
  L.user = m.user || '';
  L.exempt = !!m.exempt;
  L.token = !!m.token || L.exempt;
  if (hiPending) {
    hiPending = false;
    // A token the server no longer honors (the settings PIN changed).
    if (!L.token && readStr(TOKEN_KEY)) {
      writeStr(TOKEN_KEY, null);
      writeStr(NEVER_KEY, null);
    }
  }
  render();
  // A page that asked for something while locked asks again (log.js).
  if (was && !(L.enforced && L.locked)) emit('unlocked');
});

// ---- locking again

// An admin browser (token) locks too: `suspend` has the server let its token
// rest on this connection, and the admin password brings it back.
function lockNow() {
  if (!L.enforced || !(L.user || L.exempt)) return;
  wsSend({ t: 'userLock', suspend: L.exempt });
  L.locked = true;
  L.user = '';
  L.exempt = false;
  render();
}

function forgetBrowser() {
  writeStr(TOKEN_KEY, null);
  writeStr(NEVER_KEY, null);
  wsSend({ t: 'userLock', forget: true });
  L.exempt = false;
  L.token = false;
  L.user = '';
  L.locked = L.enforced;
  render();
}

// Ctrl+L, in the capture phase so nothing underneath sees it. Only where it
// does something: a page whose lock is on, unlocked as a user.
document.addEventListener('keydown', (e) => {
  if (root && !root.hidden && root.classList.contains('is-on')) {
    if (e.target === adminInput) {
      if (e.key === 'Escape') { e.preventDefault(); adminToggle.click(); }
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const k = /^[0-9]$/.test(e.key) ? e.key : e.key === 'Backspace' ? 'back' : e.key === 'Escape' ? 'clear' : e.key === 'Enter' ? 'enter' : null;
    if (k === 'enter' && e.target.closest?.('button')) return; // a focused key presses itself
    if (k) {
      e.preventDefault();
      e.stopPropagation();
      press(k);
    }
    return;
  }
  if (e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey && e.code === 'KeyL' && !e.repeat && L.enforced && (L.user || L.exempt)) {
    e.preventDefault();
    e.stopPropagation();
    lockNow();
  }
}, true);

// ---- idle

let lastActive = Date.now();
const touch = () => { lastActive = Date.now(); };
for (const ev of ['pointerdown', 'keydown', 'wheel', 'touchstart']) document.addEventListener(ev, touch, { capture: true, passive: true });
document.addEventListener('pointermove', touch, { passive: true });
setInterval(() => {
  // The browser's own "never auto-lock" counts only while it holds a token.
  const never = L.token && !!readStr(NEVER_KEY);
  if (idleLock(L, never, lastActive, Date.now(), S.cfg?.autoLockMinutes || 0)) lockNow();
}, 5000);
