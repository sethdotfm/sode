// My API keys. A key acts as its user and ends with a PIN change (ADR 0003). Sode shows it one time and does not keep it.

import { el, on, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
import { modal } from './modal.js';
import { createReq, expiryFrom, keyNameProblem } from './keyrules.js';
import { keyRow, secretNode } from './keyview.js';
import { btn, field } from './formbits.js';
import { pinShow, hidePin } from './pinshow.js';

const make = (id, type, auto) => el('input', { class: 'pb-input', id, type, autocomplete: auto, spellcheck: false });

const nameIn = make('mykey-name', 'text', 'off');
nameIn.placeholder = 'Panel 21';
const expIn = make('mykey-expires', 'date', 'off');
const pinIn = make('mykey-pin', 'password', 'current-password');
pinIn.inputMode = 'numeric';
const list = el('div', 'sode__key-list');
const shown = el('div');
const note = el('div', { class: 'pb-note pb-note--error', role: 'alert', hidden: true });
const makeBtn = btn('pb-btn--ok', 'Make key');

const dlg = modal({
  titleId: 'mykeys-title',
  title: 'My API keys',
  cls: 'sode__filters sode__keys-dlg',
  body: [
    el(
      'p',
      'sode__hint',
      'A key lets another app act as you, with your rights and your Me presets. It stops working when you change your PIN. If a wall panel must work after PIN changes, give it an API client key (Settings > Manage users, API clients).',
    ),
    list,
    shown,
    el(
      'div',
      'sode__keys-new',
      field('mykey-name', 'Key name', nameIn),
      field('mykey-expires', 'Expires (optional)', expIn),
      field('mykey-pin', 'Your PIN', pinShow(pinIn)),
    ),
    note,
    ,
  ],
  foot: [btn('pb-btn--outline', 'Close', () => closeKeys()), makeBtn],
  onEsc: () => closeKeys(),
});

let isOpen = false;
let back = null;
let seq = 0;
let listSeq = 0;
const say = (text) => {
  note.textContent = sentence(text);
  note.hidden = !text;
};

function load() {
  if (!wsSend({ t: 'keyListOwn', id: ++listSeq })) say(LINK_DOWN);
}

function closeKeys() {
  isOpen = false;
  shown.replaceChildren(); // the one-time key goes with the dialog
  for (const i of [nameIn, expIn, pinIn]) i.value = '';
  hidePin(pinIn);
  say('');
  seq++;
  makeBtn.disabled = false;
  dlg.hide();
  if (back?.isConnected) back.focus({ preventScroll: true });
}

onMsg('keyListOwnResult', (m) => {
  if (m.id !== listSeq || !isOpen) return;
  if (!m.ok) return say('Sign in to see your keys.');
  const now = Date.now();
  list.replaceChildren(
    ...(m.keys.length
      ? m.keys.map((k) =>
          keyRow(k, now, () => {
            if (!wsSend({ t: 'keyRevoke', keyId: k.id, id: ++listSeq })) say(LINK_DOWN);
          }),
        )
      : [el('p', 'sode__hint', 'No API keys')]),
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
  if (!pinIn.value) return say('Enter your PIN.');
  const exp = expiryFrom(expIn.value, Date.now());
  if (exp.error) return say(exp.error);
  const id = ++seq;
  if (!wsSend(createReq({ name: nameIn.value, pin: pinIn.value, expires: exp.ms, id }))) return say(LINK_DOWN);
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
  dlg.dialog.focus();
});
on('wsclose', () => {
  if (isOpen && makeBtn.disabled) {
    makeBtn.disabled = false;
    say(LINK_DOWN);
  }
});
// A sign out forgets any key on show; a Lock closes it as Esc does.
on('access', (L) => isOpen && !L.user && closeKeys());
