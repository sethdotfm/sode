// My API clients: API clients a signed-in user makes from My user (internal/app/ownclients.go). A client
// saves at once, gets a copy of the user's Rights and stays capped by them, and goes when the user is
// removed. Its keys do not end with the user's PIN; making one asks that PIN.

import { el, on, onMsg, wsSend, sentence } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { expiryFrom, keyNameProblem } from './keyrules.js';
import { keyRow, secretNode } from './keyview.js';
import { btn, field, opt } from './formbits.js';
import { OWN_SCOPES, clientNameProblem, saveReq, keyReq } from './myclientrules.js';

const input = (id, type, auto) => {
  const i = el('input', 'pb-input');
  i.id = id;
  i.type = type;
  i.autocomplete = auto;
  i.spellcheck = false;
  return i;
};

const nameIn = input('myclient-name', 'text', 'off');
nameIn.placeholder = 'Desk app';
const scopeBox = el('div', 'sode__scopes');
for (const s of OWN_SCOPES) {
  const box = el('input', 'pb-check__input');
  box.type = 'checkbox';
  box.dataset.scope = s.id;
  scopeBox.append(el('label', 'pb-check', box, el('span', 'pb-check__label', s.text)));
}
const addBtn = btn('pb-btn--ok', 'Make API client');

const who = el('select', 'pb-select');
who.id = 'myclient-key-for';
const keyName = input('myclient-key-name', 'text', 'off');
keyName.placeholder = 'Wall';
const keyExp = input('myclient-key-expires', 'date', 'off');
const pinIn = input('myclient-pin', 'password', 'current-password');
pinIn.inputMode = 'numeric';
const keyBtn = btn('pb-btn--outline', 'Make key');

const list = el('div', 'sode__key-list');
const shown = el('div');
const note = el('div', 'pb-note pb-note--error');
note.setAttribute('role', 'alert');
note.hidden = true;
const close = btn('pb-btn--outline', 'Close');

const dialog = el(
  'div',
  'pb-modal sode__filters sode__keys-dlg',
  el('div', 'pb-modal__header', el('span', 'pb-modal__title', 'My API clients')),
  el(
    'div',
    'pb-modal__body',
    el(
      'p',
      'sode__hint',
      'An API client is an app with its own name in History. It gets a copy of your Rights and never more than your Rights. Its keys work after you change your PIN. It is removed when your user is removed.',
    ),
    list,
    shown,
    el('h3', 'sode__profiles-h', 'Make an API client'),
    el('div', 'sode__keys-new', field('myclient-name', 'Name', nameIn), el('div', null, addBtn)),
    scopeBox,
    el('h3', 'sode__profiles-h', 'Make a key'),
    el(
      'div',
      'sode__keys-new',
      field('myclient-key-for', 'For', el('span', 'pb-select-wrap', who)),
      field('myclient-key-name', 'Key name', keyName),
      field('myclient-key-expires', 'Expires (optional)', keyExp),
      field('myclient-pin', 'Your PIN', pinIn),
      el('div', null, keyBtn),
    ),
    note,
  ),
  el('div', 'pb-modal__footer', close),
);
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-label', 'My API clients');
dialog.tabIndex = -1;
const root = el('div', 'pb-modal-backdrop sode__scrim', dialog);
root.hidden = true;
document.body.append(root);

let isOpen = false;
let back = null;
let saveSeq = 0;
let keySeq = 0;
let listSeq = 0;
let mine = [];
const say = (text) => {
  note.textContent = sentence(text);
  note.hidden = !text;
};
const send = (m) => wsSend(m) || (say('Not connected to Sode.'), false);
const load = () => send({ t: 'myClients', id: ++listSeq });

function closeClients() {
  isOpen = false;
  shown.replaceChildren(); // the one-time key goes with the dialog
  for (const i of [nameIn, keyName, keyExp, pinIn]) i.value = '';
  say('');
  saveSeq++;
  keySeq++;
  addBtn.disabled = keyBtn.disabled = false;
  dlg.hide();
  if (back?.isConnected) back.focus({ preventScroll: true });
}
const dlg = trapDialog(root, dialog, closeClients);
close.addEventListener('click', closeClients);

const scopeText = (ids) =>
  OWN_SCOPES.filter((s) => ids.includes(s.id))
    .map((s) => s.text)
    .join(', ') || 'No scopes';

onMsg('myClientsResult', (m) => {
  if (m.id !== listSeq || !isOpen) return;
  if (!m.ok) return say(m.err || 'Sign in to see your API clients.');
  mine = m.clients.map((c) => c.name);
  const keep = who.value;
  who.replaceChildren(...mine.map((n) => opt(n, n)));
  if (mine.includes(keep)) who.value = keep;
  const now = Date.now();
  list.replaceChildren(
    ...(m.clients.length
      ? m.clients.flatMap((c) => [
          el(
            'div',
            'sode__keys-owner',
            el('strong', null, c.name),
            el('span', 'sode__hint', scopeText(c.scopes)),
            btn('pb-btn--outline pb-btn--sm', 'Remove', () => send({ t: 'myClientDelete', id: ++listSeq, name: c.name })),
          ),
          ...m.keys
            .filter((k) => k.owner.name.toLowerCase() === c.name.toLowerCase())
            .map((k) => keyRow(k, now, () => send({ t: 'myClientKeyRevoke', keyId: k.id, id: ++listSeq }))),
        ])
      : [el('p', 'sode__hint', 'You have no API clients.')]),
  );
});
for (const t of ['myClientDeleteResult', 'myClientKeyRevokeResult'])
  onMsg(t, (m) => {
    if (!isOpen) return;
    if (!m.ok) say(m.err);
    load();
  });

addBtn.addEventListener('click', () => {
  say('');
  const problem = clientNameProblem(nameIn.value, mine);
  if (problem) return say(problem);
  const scopes = [...scopeBox.querySelectorAll('input:checked')].map((i) => i.dataset.scope);
  if (send(saveReq({ name: nameIn.value, scopes, id: ++saveSeq }))) addBtn.disabled = true;
});
onMsg('myClientSaveResult', (m) => {
  if (m.id !== saveSeq || !isOpen) return;
  addBtn.disabled = false;
  if (!m.ok) return say(m.err);
  nameIn.value = '';
  for (const i of scopeBox.querySelectorAll('input')) i.checked = false;
  load();
});

keyBtn.addEventListener('click', () => {
  say('');
  if (!who.value) return say('Make an API client first, then make its key.');
  const problem = keyNameProblem(keyName.value);
  if (problem) return say(problem);
  if (!pinIn.value) return say('Enter your PIN to make a key.');
  const exp = expiryFrom(keyExp.value, Date.now());
  if (exp.error) return say(exp.error);
  if (send(keyReq({ client: who.value, name: keyName.value, pin: pinIn.value, expires: exp.ms, id: ++keySeq }))) keyBtn.disabled = true;
});
onMsg('myClientKeyResult', (m) => {
  if (m.id !== keySeq || !isOpen) return;
  keyBtn.disabled = false;
  pinIn.value = '';
  if (!m.ok) return say(m.err);
  keyName.value = keyExp.value = '';
  shown.replaceChildren(secretNode(m.key));
  load();
});

on('openClients', () => {
  back = document.activeElement;
  isOpen = true;
  shown.replaceChildren();
  say('');
  load();
  dlg.show();
  dialog.focus();
});
// Signing out or a Lock closes it, forgetting any key on show.
on('locking', () => isOpen && closeClients());
on('access', (L) => isOpen && !L.user && closeClients());
