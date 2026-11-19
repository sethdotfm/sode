// A client gets a copy of the user's Rights, stays capped by them and goes with the user. Its keys outlive the user's PIN.

import { el, on, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
import { modal } from './modal.js';
import { expiryFrom, keyNameProblem } from './keyrules.js';
import { keyRow, secretNode } from './keyview.js';
import { btn, field, opt } from './formbits.js';
import { OWN_SCOPES, clientNameProblem, saveReq, keyReq } from './myclientrules.js';

const input = (id, type, auto) => el('input', { class: 'pb-input', id, type, autocomplete: auto, spellcheck: false });

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
const note = el('div', { class: 'pb-note pb-note--error', role: 'alert', hidden: true });

const dlg = modal({
  titleId: 'myclients-title',
  title: 'My API clients',
  cls: 'sode__filters sode__keys-dlg',
  body: [
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
    ,
  ],
  foot: [btn('pb-btn--outline', 'Close', () => closeClients())],
  onEsc: () => closeClients(),
});

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
const send = (m) => wsSend(m) || (say(LINK_DOWN), false);
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
  keyBtn.disabled = !mine.length;
  if (mine.length) delete keyBtn.dataset.pbTip;
  else keyBtn.dataset.pbTip = 'Make an API client first';
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
      : [el('p', 'sode__hint', 'No API clients')]),
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
  if (!who.value) return;
  const problem = keyNameProblem(keyName.value);
  if (problem) return say(problem);
  if (!pinIn.value) return say('Enter your PIN.');
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
  dlg.dialog.focus();
});
on('wsclose', () => {
  if (!isOpen || !(addBtn.disabled || keyBtn.disabled)) return;
  addBtn.disabled = keyBtn.disabled = false;
  say(LINK_DOWN);
});
on('access', (L) => isOpen && !L.user && closeClients());
