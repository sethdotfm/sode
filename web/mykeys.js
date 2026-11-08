// My API keys: a signed-in user's own keys, opened from Settings' My user section. Making a key asks
// the user's own PIN (the server checks it in the change-PIN pool). The key is shown once and kept nowhere.
// A user's key acts as that user and ends with a PIN change; ADR 0003.

import { el, on, onMsg, wsSend, sentence } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { createReq, expiryFrom, keyNameProblem } from './keyrules.js';
import { keyRow, secretNode } from './keyview.js';
import { btn, field } from './formbits.js';

const make = (id, type, auto) => {
  const i = el('input', 'pb-input');
  i.id = id;
  i.type = type;
  i.autocomplete = auto;
  i.spellcheck = false;
  return i;
};

const nameIn = make('mykey-name', 'text', 'off');
nameIn.placeholder = 'Panel 21';
const expIn = make('mykey-expires', 'date', 'off');
const pinIn = make('mykey-pin', 'password', 'current-password');
pinIn.inputMode = 'numeric';
const list = el('div', 'sode__key-list');
const shown = el('div');
const note = el('div', 'pb-note pb-note--error');
note.setAttribute('role', 'alert');
note.hidden = true;
const makeBtn = btn('pb-btn--ok', 'Make key');
const close = btn('pb-btn--outline', 'Close');

const dialog = el(
  'div',
  'pb-modal sode__filters sode__keys-dlg',
  el('div', 'pb-modal__header', el('span', 'pb-modal__title', 'My API keys')),
  el(
    'div',
    'pb-modal__body',
    el(
      'p',
      'sode__hint',
      'A key lets another app act as you, with your rights and your Me presets. It stops working when you change your PIN, so a wall panel that must outlive PIN changes should use an API client key (Settings > Manage users, API clients).',
    ),
    list,
    shown,
    el(
      'div',
      'sode__keys-new',
      field('mykey-name', 'Key name', nameIn, 'Shown beside your name in History, such as "Seth (Panel 21)".'),
      field('mykey-expires', 'Expires (optional)', expIn),
      field('mykey-pin', 'Your PIN', pinIn),
    ),
    note,
  ),
  el('div', 'pb-modal__footer', close, makeBtn),
);
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-label', 'My API keys');
dialog.tabIndex = -1;
const root = el('div', 'pb-modal-backdrop sode__scrim', dialog);
root.hidden = true;
document.body.append(root);

let isOpen = false;
let back = null;
let seq = 0;
let listSeq = 0;
const say = (text) => {
  note.textContent = sentence(text);
  note.hidden = !text;
};

function load() {
  if (!wsSend({ t: 'keyListOwn', id: ++listSeq })) say('Not connected to Sode.');
}

function closeKeys() {
  isOpen = false;
  shown.replaceChildren(); // the one-time key goes with the dialog
  for (const i of [nameIn, expIn, pinIn]) i.value = '';
  say('');
  seq++;
  makeBtn.disabled = false;
  dlg.hide();
  if (back?.isConnected) back.focus({ preventScroll: true });
}
const dlg = trapDialog(root, dialog, closeKeys);
close.addEventListener('click', closeKeys);

onMsg('keyListOwnResult', (m) => {
  if (m.id !== listSeq || !isOpen) return;
  if (!m.ok) return say('Sign in to see your keys.');
  const now = Date.now();
  list.replaceChildren(
    ...(m.keys.length
      ? m.keys.map((k) =>
          keyRow(k, now, () => {
            if (!wsSend({ t: 'keyRevoke', keyId: k.id, id: ++listSeq })) say('Not connected to Sode.');
          }),
        )
      : [el('p', 'sode__hint', 'You have no API keys.')]),
  );
});
onMsg('keyRevokeResult', (m) => {
  if (!isOpen) return;
  if (!m.ok) say(m.err);
  load();
});

makeBtn.addEventListener('click', () => {
  say('');
  const problem = keyNameProblem(nameIn.value);
  if (problem) return say(problem);
  if (!pinIn.value) return say('Enter your PIN to make a key.');
  const exp = expiryFrom(expIn.value, Date.now());
  if (exp.error) return say(exp.error);
  const id = ++seq;
  if (!wsSend(createReq({ name: nameIn.value, pin: pinIn.value, expires: exp.ms, id }))) return say('Not connected to Sode.');
  makeBtn.disabled = true;
});
onMsg('keyCreateResult', (m) => {
  if (m.id !== seq || !isOpen) return;
  makeBtn.disabled = false;
  pinIn.value = '';
  if (!m.ok) return say(m.err);
  nameIn.value = expIn.value = '';
  shown.replaceChildren(secretNode(m.key));
  load();
});

on('openKeys', () => {
  back = document.activeElement;
  isOpen = true;
  shown.replaceChildren();
  say('');
  load();
  dlg.show();
  dialog.focus();
});
// Signing out or a Lock closes it, forgetting any key on show.
on('locking', () => isOpen && closeKeys());
on('access', (L) => isOpen && !L.user && closeKeys());
