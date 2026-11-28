// The lock screen. The server decides everything (`lockState`) and refuses what a locked
// connection asks for. This module shows the pad and locks again when idle or on Ctrl+L. A
// persistent user gets a session token kept here and sent in `hi`, so a reload does not sign them
// out. The Lock keeps the token, and a sign-out revokes it. Off unless the server says this page type is locked.
// With no PIN lock, a Lock with nobody signed in (churning letter, behind the confirm lockout) can still go up. It
// guards against keyboard mashing. It is page-local state, and a reload clears it.

import { el, icon, S, on, emit, onMsg, wsSend, LINK_DOWN } from './core.js';
import { heldUserKept, keepHeldUser, lastUser, keepLastUser, startHandshake, keepSignInToken, forgetTokens, forgetSignIn, onSessionToken } from './signin.js';
import {
  padKey,
  idleLock,
  pingDue,
  pageType,
  OUT_FACTOR,
  dots,
  wrongFeedback,
  WELCOME_MS,
  churnKey,
  localIdleLock,
  signInOffered,
  asksName,
  padLens,
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
  chipSignInShown,
  heldEnded,
  heldWho,
  prefillName,
  focusName,
  pinFreeView,
  padAsked,
  startStepIn,
  stepInKept,
  padOf,
  stepInWon,
} from './lockrules.js';
import { lockout } from './lockout.js';
import { closeShown } from './dialogkeys.js';
import { chord } from './keys.js';
import { dialogAttrs } from './modal.js';
import { stepLock, noteStepState, steppedIn, sendStepIn, stepOut } from './stepin.js';
import { stepActive, stepInOffered } from './stepinrules.js';
import { createOwed } from './owed.js';
import { createIdleShare } from './idleshare.js';
import { buildChip } from './lockchip.js';
import { askSignOut } from './signoutask.js';
import { layout, onLayout } from './layout.js';
import { namePicker } from './namepick.js';
import { listedUser } from './namepickrules.js';
import { setBrand } from './brandicon.js';
import { motionMs, moveBrand, fitBrand } from './lockbrand.js';

let heldUser = heldUserKept();
const page = pageType(location.pathname);

{
  const css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = 'lock.css';
  document.head.append(css);
}

const L = {
  enforced: false,
  locked: false,
  held: false,
  user: '',
  role: '',
  trusted: false,
  persistent: false,
  noAutoLock: false,
  pinNeeded: false,
  lens: [],
};
// Not enumerable, so the {...L} copies that go back through Object.assign(L, ...) do not hit a getter-only prop.
Object.defineProperty(L, 'heldNoPin', { get: () => L.held && listedUser(S.users, heldUser)?.len === 0 });
export const lockState = () => ({ ...L });

startHandshake(page);
onSessionToken(() => !!L.user);

let root = null;
let brand, modeLine, dotsEl, msg, state, chip, checkKey, goWord;
let nameField, nameInput, picker;
let padBody, letterBody, letterKey, letterBtn, letterNote, signInAsBtn, signOutBtn, letterFoot;
const owed = createOwed(wsSend); // a Lock the server has not got yet
let unlocked = false; // the lockState after a userSignIn ends a sign-in
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
let offlineLine;

// Offline wins over the Lock: html[data-conn="down"] (desktop.js, panelrun.js) gives the Lock the offline colors
// (lock.css), and the Lock names both states.
function sayOffline() {
  if (!root) return;
  const off = document.documentElement.dataset.conn === 'down';
  const p = pad();
  offlineLine.hidden = !off;
  offlineLine.textContent = p.offline;
  root.setAttribute('aria-label', off ? p.offLabel : p.label);
}

const pad = () => padOf(V);
const SEND = {
  signIn: (user, pin, id) => wsSend(signInMsg(L, { pin, id, user })),
  stepIn: (user, pin, id) => sendStepIn(user?.name || '', pin, id),
};
const stepping = () => V.mode === MODE.STEP_IN;
const stepOpen = () => stepInOffered({ locked: L.locked, phone: layout() === 'phone', stepped: steppedIn(), user: L.user, users: S.users });

function build() {
  if (root) return;
  brand = el('h1', 'sode-lock__brand', 'Sode');
  offlineLine = el('p', 'sode-lock__offline', 'Offline, locked');
  modeLine = el('p', 'sode-lock__mode');
  modeLine.hidden = true;
  state = el('p', 'sode-lock__state', 'Locked');
  dotsEl = el('div', 'sode-lock__dots');
  dotsEl.setAttribute('aria-hidden', 'true');
  msg = el('p', 'sode-lock__msg');
  msg.setAttribute('role', 'status');
  msg.setAttribute('aria-live', 'polite');

  const digits = el('div', 'sode-lock__pad');
  const key = (label, k, cls = '') => {
    const b = el('button', `pb-btn pb-btn--outline sode-lock__key ${cls}`, label);
    b.type = 'button';
    b.dataset.key = k;
    b.addEventListener('click', () => press(k));
    return b;
  };
  // The word follows the pad (Sign in or Step in, set in showLock).
  goWord = el('span', null, padOf(PAGE_VIEW).go);
  checkKey = key('', 'enter', 'sode-lock__key--word sode-lock__key--go');
  checkKey.append(icon('check'), goWord);
  checkKey.disabled = true;
  // Cancel shares the last row with the Sign in key, so a sign-in adds no row to the Lock.
  cancelBtn = el('button', 'pb-btn pb-btn--outline sode-lock__key sode-lock__key--word sode-lock__cancel', 'Cancel');
  cancelBtn.type = 'button';
  cancelBtn.hidden = true;
  cancelBtn.addEventListener('click', cancelSignIn);
  digits.append(
    ...['1', '2', '3', '4', '5', '6', '7', '8', '9'].map((d) => key(d, d)),
    key('Clear', 'clear', 'sode-lock__key--word'),
    key('0', '0'),
    key('Del', 'back', 'sode-lock__key--word'),
    cancelBtn,
    checkKey,
  );
  picker = namePicker({
    id: 'lock-name',
    users: () => pad().users(L, S.users),
    onInput: () => {
      if (dotsEl.classList.contains('is-text')) clearFeedback();
      say('');
      paint();
      clearTimeout(timer);
      const wait = padDelay(L, pin, who());
      if (wait != null && pin) timer = setTimeout(sendPin, wait);
    },
    onPick: () => {
      if (pad().pickSends(L, who())) {
        pin = '';
        return sendPin();
      }
      if (!pin || !canSend(L, pin, who())) root.focus({ preventScroll: true });
    },
  });
  nameInput = picker.input;
  nameField = el('div', 'sode-lock__namefield', Object.assign(el('label', 'sode-lock__namelabel', 'Name'), { htmlFor: 'lock-name' }), picker.root);
  nameField.hidden = true;
  // Entry order: who, then the PIN (its dots, count and messages right above the keys), then the keys. The name sits
  // on the pad, so name and digits are typed in one place.
  padBody = el('div', 'sode-lock__body', nameField, dotsEl, state, msg, digits);

  letterKey = el('kbd', 'sode__key sode__key--locked');
  letterBtn = el('button', 'pb-btn sode-btn--primary', 'Dismiss');
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
  // The foot is the last row, in flow, so it cannot land on the pad.
  root = el(
    'div',
    { class: 'sode-lock', ...dialogAttrs(), 'aria-label': 'Locked', hidden: true },
    el('div', 'sode-lock__main', brand, modeLine, offlineLine, padBody, letterBody),
    letterFoot,
  );
  document.body.append(root);
  sayOffline();
  new MutationObserver(sayOffline).observe(document.documentElement, { attributes: true, attributeFilter: ['data-conn'] });

  chip = buildChip({
    lock: localLock,
    signIn: () => emit('login'),
    signOut: endSignIn,
    stepIn: () => emit('stepInOpen'),
    stepOut,
    home: page === 'main',
    stepped: steppedIn,
    stepOffered: stepOpen,
  });
  document.body.append(chip.root);
}

function say(text, bad = false) {
  if (!root) return;
  msg.textContent = text;
  msg.className = `sode-lock__msg${bad ? ' pb-note pb-note--error' : ''}`;
}

function render() {
  V = stepInKept(V, stepOpen());
  const covered = viewBody(V, lockCovers(L)) !== '';
  const who = L.held ? heldUser : '';
  const showChip = chipShown({ covered, letter: V.mode === MODE.LOCK, user: L.user, heldUser: who });
  if (covered) showLock();
  else hideLock();
  build();
  chip.update(L, showChip, V.mode === MODE.LOCK || L.held, L.user || who, chipSignInShown(L, V, covered));
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

let scrolled = null;
let keptOpen = false;

function showLock() {
  build();
  const body = viewBody(V, lockCovers(L));
  padBody.hidden = body !== 'pad';
  cancelBtn.hidden = !padAsked(V);
  goWord.textContent = pad().go;
  modeLine.textContent = pad().note;
  modeLine.hidden = !pad().note;
  nameField.hidden = !asksName(L);
  letterBody.hidden = body !== 'letter';
  signOutBtn.hidden = !(L.held || signOutOffered(L));
  signInAsBtn.hidden = L.held || !signInAsOffered(L);
  letterFoot.hidden = !L.held && (body !== 'letter' || (signOutBtn.hidden && signInAsBtn.hidden));
  sayOffline();
  clearTimeout(hideTimer);
  if (!root.hidden && root.classList.contains('is-on')) {
    // A Lock came over a sign-in or Step in pad that kept Settings open.
    if (keptOpen && !padAsked(V)) {
      keptOpen = false;
      closeModals();
    }
    return;
  }
  pin = '';
  picker.set(pad().prefill(L, lastUser(), S.users));
  welcoming = false;
  clearFeedback();
  say('');
  const text = S.cfg?.brandText?.trim() || 'Sode';
  brand.textContent = text;
  const frame = document.querySelector('.sode__brandframe');
  setBrand(root, frame?.style.getPropertyValue('--brand-color').trim() || frame?.dataset.brand || '');
  // The page keeps its scroll; focus moves must not scroll it (desktop.js leaves its
  // focus-scroll alone while this is set).
  if (root.hidden) scrolled = { x: window.scrollX, y: window.scrollY };
  document.documentElement.dataset.sodeLock = 'on';
  setHand(true);
  root.hidden = false;
  fitBrand(root, brand);
  moveBrand(brand, 'in');
  void root.offsetWidth;
  root.classList.add('is-on');
  document.documentElement.dataset.sodeLockOn = 'on';
  chip?.setLocked(true);
  // A sign-in or Step in from Settings keeps it open: Cancel goes back to it.
  keptOpen = padAsked(V);
  if (!keptOpen) closeModals();
  inerted = [];
  for (const c of document.body.children) inert(c);
  watch ||= new MutationObserver((list) => {
    for (const r of list) for (const n of r.addedNodes) inert(n);
  });
  watch.observe(document.body, { childList: true });
  const had = document.activeElement;
  if (had && had !== document.body && !root.contains(had)) before = had;
  had?.blur?.();
  focusPad();
}

function focusPad() {
  if (focusName(L, nameValue()) && V.mode !== MODE.LOCK) nameInput.focus({ preventScroll: true });
  else root.focus({ preventScroll: true });
}

const refit = () => {
  if (root && !root.hidden) fitBrand(root, brand);
};
addEventListener('resize', refit);
onLayout(refit);
// The phone layout offers no Step in.
onLayout(() => {
  if (pad().lapses) render();
});

function setHand(on) {
  if (covering === on) return;
  covering = on;
  emit('lockcover', { on });
}

let watch = null;
let before = null;
function inert(c) {
  if (c.nodeType !== 1 || c === root || c === chip?.root || ['SCRIPT', 'LINK', 'STYLE'].includes(c.tagName) || c.inert) return;
  if (c.previousElementSibling === root && c.classList.contains('sode-bigscroll')) return; // the Lock's own bar
  c.inert = true;
  inerted.push(c);
}

// A module that needs more than Esc on a Lock listens for 'locking' (Settings exits; an unsaved form keeps its
// warning, covered). Every other trapDialog dialog closes as on Esc.
function closeModals() {
  for (const d of document.querySelectorAll('dialog[open]')) d.close();
  emit('locking');
  closeShown();
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
  // The way out is slower (--sode-lock-out in lock.css = OUT_FACTOR); the root is hidden after
  // the longest transition is over.
  const ms = motionMs();
  const out = ms * OUT_FACTOR;
  if (wasOn) moveBrand(brand, 'out');
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
  checkKey.disabled = !canSend(L, pin, who());
}

const nameValue = () => nameInput?.value ?? '';
const who = () => listedUser(S.users, asksName(L) ? nameValue() : heldUser);

function press(k) {
  clearTimeout(timer);
  if (welcoming) return;
  if (dotsEl.classList.contains('is-text')) clearFeedback();
  if (k === 'enter') {
    if (canSend(L, pin, who())) sendPin();
    else if (asksName(L) && !nameValue().trim()) say('Enter your name.', true);
    else if (asksName(L) && !who()) say('Unknown name.', true);
    else say(pin ? 'Wrong number of digits.' : 'Enter your PIN.', true);
    return;
  }
  pin = padKey(pin, k, padLens(L, who()), S.cfg?.maxPinLen || 0);
  paint();
  if (k !== 'clear' && k !== 'back') say('');
  // Only the longest length goes on its own: it is possible that a pause after a shorter PIN is
  // the start of a longer one.
  const wait = padDelay(L, pin, who());
  if (wait == null) return;
  timer = setTimeout(sendPin, wait);
}

function sendPin() {
  const id = ++seq;
  const sent = SEND[pad().send](who(), pin, id);
  if (!sent) return say(LINK_DOWN, true);
  say('Checking…');
}

function refused(err) {
  pin = '';
  const w = wrongFeedback(err);
  clearFeedback();
  feedback(w.text, 'bad', w.ms, true);
  say(w.note, !!w.note);
}

onMsg('userSignInResult', (m) => {
  if (m.id !== seq) return;
  if (m.ok) {
    if (asksName(L)) keepLastUser(who()?.name);
    pin = '';
    unlocked = true;
    say('');
    keepSignInToken(m);
    return;
  }
  refused(m.err);
});

// The step-in's lockState comes before its result and closes the pad (stepInKept), so only a refusal is left here.
on('stepInResult', (m) => {
  if (m.id !== seq || !stepping() || m.ok) return;
  refused(m.err);
});

on('stepInOpen', () => {
  if (!stepOpen()) return;
  V = startStepIn(V);
  if (!stepping()) return;
  pin = '';
  clearTimeout(timer);
  render();
});

onMsg('lockState', (m) => {
  const wasStepped = steppedIn();
  noteStepState(m);
  if (owed.arrived(m)) return;
  if (settle) settle();
  const stepWon = stepInWon(V, wasStepped, steppedIn());
  const was = lockCovers(L) || V.mode === MODE.SIGN_IN || stepWon;
  const ended = heldEnded(L, m);
  heldUser = heldWho(heldUser, L, m);
  keepHeldUser(heldUser);
  lastState = m;
  L.held = !!m.held;
  L.user = m.user || '';
  L.trusted = !!m.trusted;
  L.role = m.role || '';
  L.persistent = !!m.persistent;
  L.noAutoLock = !!m.noAutoLock;
  L.pinNeeded = !!m.pinNeeded;
  L.lens = m.lens || [];
  applyCover(m);
  // The Lock ended without this tab's PIN: it stays covered, as after a sign-out from the Lock with nobody signed in.
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
    if (was && !stepWon && !lockCovers(L)) emit('unlocked');
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

// enforced and locked from m, as a Public panel's page sees them. A lock that covers ends the Lock with nobody signed in.
function applyCover(m) {
  Object.assign(L, pinFreeView(m, pinFree));
  if (lockCovers(L) && letterLock) {
    letterLock.lock.cancel();
    letterLock = null;
  }
  V = lockArrived(V, lockCovers(L));
}

// The panel page says whether it runs as a Public panel; the last lockState is read again.
on('publicPanel', ({ on: free }) => {
  if (pinFree === !!free) return;
  pinFree = !!free;
  if (!lastState) return;
  applyCover(lastState);
  render();
  emit('access', { ...L });
});

on('hello', () => {
  if (!picker) return;
  picker.refresh();
  if (!root.hidden && !nameValue()) picker.set(pad().prefill(L, lastUser(), S.users));
  paint();
  // A hello can leave nobody to step in as.
  if (pad().lapses) render();
});

on('logout', () => signOutNow());

on('stepEnded', (m) => {
  if (!m.note) return;
  build();
  say(m.note, true);
});

on('login', () => {
  if (!signInOffered(L) || V.mode !== MODE.PAGE) return;
  V = startSignIn(V, false);
  render();
});

function cancelSignIn() {
  if (!padAsked(V)) return;
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
  if (!nameValue()) picker.set(prefillName(L, lastUser(), S.users));
  pin = '';
  clearTimeout(timer);
  clearFeedback();
  say('');
  render();
  // The overlay stays up, so showLock skips its focus step. The clicked button is now hidden and took the focus with it,
  // and letters would go nowhere.
  focusPad();
}

// From the held Lock, or under the Lock with nobody signed in, with someone signed in: the session ends.
function signOutFromFoot() {
  if (L.held || (V.mode === MODE.LOCK && signOutOffered(L))) signOutNow();
}

on('localLock', localLock);

// The Lock keeps the session: the server holds it, so a reload or another tab shows the pad
// until this user's PIN resumes it. The token stays for that.
function lockNow() {
  if (!L.user || stepLock()) return;
  pin = '';
  clearTimeout(timer);
  heldUser = L.user;
  keepHeldUser(heldUser);
  Object.assign(L, raiseLock(L));
  render();
  // The access event first: it drops the page's session key and stops its reads on the way (sessionkey.js). Sent
  // after, the Lock can reach Sode before such a read, which then ends in a 401.
  emit('access', { ...L });
  owed.owe();
}

// Sign out ends the session: the server ends the sign-in and revokes the token, so a reload cannot undo it.
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

// Stepped in, Sign out steps out, as the server's userSignOut does; signOutNow would show the tab signed out until the
// lockState came.
function endSignIn() {
  if (steppedIn()) stepOut();
  else signOutNow();
}

// Capture phase so nothing underneath sees it. Only where it does something.
document.addEventListener(
  'keydown',
  (e) => {
    if (V.mode === MODE.LOCK) {
      // The page behind must not see a key (not a take, not Ctrl+Z). A shortcut chord is kept
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
      // Keep the pad's keys out of the name box; Enter in the list moves on to the PIN.
      if (e.target === nameInput && picker.key(e)) {
        e.preventDefault();
        e.stopPropagation();
        return;
      }
      if (padAsked(V) && e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        cancelSignIn();
        return;
      }
      if (e.target === nameInput) return;
      const k = /^[0-9]$/.test(e.key) ? e.key : e.key === 'Backspace' ? 'back' : e.key === 'Escape' ? 'clear' : e.key === 'Enter' ? 'enter' : null;
      if (k === 'enter' && e.target.closest?.('button')) return;
      if (k) {
        e.preventDefault();
        e.stopPropagation();
        press(k);
      }
      return;
    }
    // A kiosk hides the Lock bar, so its sign-out key is off too.
    if (chord(e) === 'ctrl+shift+l' && !e.repeat && L.user && !document.documentElement.hasAttribute('data-sode-kiosk')) {
      e.preventDefault();
      e.stopPropagation();
      chip?.closeMenu();
      askSignOut({ user: L.user, stepped: steppedIn() }, endSignIn);
      return;
    }
    if (chord(e) === 'ctrl+l' && !e.repeat) {
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
let ownActive = Date.now(); // this tab's own input, which alone feeds a step-in
const touch = () => {
  ownActive = Date.now();
  idle.touch();
};
for (const ev of ['pointerdown', 'keydown', 'wheel', 'touchstart']) document.addEventListener(ev, touch, { capture: true, passive: true });
document.addEventListener('pointermove', touch, { passive: true });
let lastPing = 0;
setInterval(() => {
  const minutes = S.cfg?.autoLockMinutes || 0;
  const stepped = steppedIn();
  if (pingDue(L, lastPing, stepActive(stepped, idle.lastActive(), ownActive), Date.now()) && wsSend({ t: 'activity' })) lastPing = Date.now();
  // The server ends an idle step-in and decides whether the original locks (internal/app/stepin.go).
  if (stepped) return;
  if (idleLock(L, idle.lastActive(), Date.now(), minutes)) lockNow();
  else if (S.cfg && localIdleLock(L, !!letterLock, idle.lastActive(), Date.now(), minutes)) localLock();
}, 5000);
