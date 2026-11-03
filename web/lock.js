// The lock screen (#151). The server decides everything: it tells this page
// whether the page type is locked and who (if anyone) unlocked it
// (`lockState`), and refuses what a locked connection asks for. This module
// shows a full-viewport pad that sends the PIN as it is typed (no Enter),
// locks again when idle or on Ctrl+L, and lets the admin password (the
// settings PIN) make this browser exempt via a server-issued token.
//
// Off unless the server says this page type is locked: with no users and no
// locks nothing here shows or is bound but a few idle listeners.

import { $, el, S, on, emit, onMsg, wsSend, sentence } from './core.js';
import { readStr, writeStr } from './store.js';
import { padKey, sendDelay, idleDue, pageType, liftFrom, dots } from './lockrules.js';

const TOKEN_KEY = 'sode-lock-token';
const page = pageType(location.pathname);

{
  const css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = 'lock.css';
  document.head.append(css);
}

// What the server last said about this connection.
const L = { enforced: false, locked: false, user: '', exempt: false };
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
let brand, dotsEl, msg, state, adminForm, adminInput, adminToggle;
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
  pad.append(
    ...['1', '2', '3', '4', '5', '6', '7', '8', '9'].map((d) => key(d, d)),
    key('Clear', 'clear', 'sode-lock__key--word'),
    key('0', '0'),
    key('Del', 'back', 'sode-lock__key--word'),
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
  root = el('div', 'sode-lock', brand, body);
  root.hidden = true;
  root.tabIndex = -1;
  root.setAttribute('role', 'dialog');
  root.setAttribute('aria-modal', 'true');
  root.setAttribute('aria-label', 'Locked');
  document.body.append(root);

  // A who-is-unlocked chip with the way to lock.
  who = el('div', 'sode-lock__who');
  whoName = el('span', 'sode-lock__who-name');
  whoBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm');
  whoBtn.type = 'button';
  whoBtn.addEventListener('click', () => (L.exempt && !L.user ? forgetBrowser() : lockNow()));
  who.append(whoName, whoBtn);
  who.hidden = true;
  document.body.append(who);
}
let who, whoName, whoBtn;

function say(text, bad = false) {
  if (!root) return;
  msg.textContent = text;
  msg.className = `sode-lock__msg${bad ? ' pb-note pb-note--error' : ''}`;
}

function render() {
  const chip = L.enforced && !L.locked && (L.user || L.exempt);
  if (!root && !L.locked && !chip) return; // nothing configured: nothing built
  if (L.enforced && L.locked) showLock();
  else hideLock();
  build();
  who.hidden = !chip;
  if (chip) {
    whoName.textContent = L.user ? `Unlocked as ${L.user}` : 'Admin browser';
    whoBtn.textContent = L.user ? 'Lock' : 'Forget';
    whoBtn.title = L.user ? 'Lock (Ctrl+L)' : 'Stop skipping the lock on this browser';
  }
}

const logo = () => [...document.querySelectorAll('.sode__mark, .sode-m__mark, .sode-log__title')].find((e) => e.getBoundingClientRect().width > 0);

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
  root.hidden = false;
  // Start the logo over the page's own, then let it fly to the middle.
  const from = logo()?.getBoundingClientRect();
  root.style.setProperty('--lx', '0px');
  root.style.setProperty('--ly', '0px');
  root.style.setProperty('--ls', '1');
  const t = liftFrom(from, brand.getBoundingClientRect());
  root.style.setProperty('--lx', `${t.x}px`);
  root.style.setProperty('--ly', `${t.y}px`);
  root.style.setProperty('--ls', String(t.scale));
  void root.offsetWidth; // commit the start before the class changes it
  root.classList.add('is-on');
  inerted = [...document.body.children].filter((c) => c !== root && c !== who && c.tagName !== 'SCRIPT' && c.tagName !== 'LINK' && !c.inert);
  for (const c of inerted) c.inert = true;
  document.activeElement?.blur?.();
  root.focus();
}

function hideLock() {
  if (!root || root.hidden) return;
  clearTimeout(timer);
  for (const c of inerted) c.inert = false;
  inerted = [];
  root.classList.remove('is-on');
  adminForm.hidden = true;
  adminInput.value = '';
  adminToggle.textContent = 'Admin password';
  pin = '';
  const ms = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--sode-lock-motion')) || 0;
  hideTimer = setTimeout(() => { root.hidden = true; }, ms + 50);
  $('src')?.focus?.();
}

// ---- entering a PIN

function paint() {
  dotsEl.textContent = dots(pin);
  state.textContent = pin ? `${pin.length} ${pin.length === 1 ? 'digit' : 'digits'} entered` : 'Locked. Enter your PIN.';
}

const lens = () => [...new Set((S.cfg?.users || []).map((u) => u.len))];

function press(k) {
  pin = padKey(pin, k);
  paint();
  if (k !== 'clear' && k !== 'back') say('');
  clearTimeout(timer);
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
  if (m.id !== seq || m.partial) return;
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
  if (hiPending) {
    hiPending = false;
    // A token the server no longer honors (the settings PIN changed).
    if (!L.exempt && readStr(TOKEN_KEY)) writeStr(TOKEN_KEY, null);
  }
  render();
  // A page that asked for something while locked asks again (log.js).
  if (was && !(L.enforced && L.locked)) emit('unlocked');
});

// ---- locking again

function lockNow() {
  if (!L.enforced || !L.user || L.exempt) return;
  wsSend({ t: 'userLock' });
  L.locked = true;
  L.user = '';
  render();
}

function forgetBrowser() {
  writeStr(TOKEN_KEY, null);
  wsSend({ t: 'userLock', forget: true });
  L.exempt = false;
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
    const k = /^[0-9]$/.test(e.key) ? e.key : e.key === 'Backspace' ? 'back' : e.key === 'Escape' ? 'clear' : null;
    if (k) {
      e.preventDefault();
      e.stopPropagation();
      press(k);
    }
    return;
  }
  if (e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey && e.code === 'KeyL' && !e.repeat && L.enforced && L.user && !L.exempt) {
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
  if (L.enforced && !L.locked && L.user && !L.exempt && idleDue(lastActive, Date.now(), S.cfg?.autoLockMinutes || 0)) lockNow();
}, 5000);
