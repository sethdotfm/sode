// The Send dialog (Send a copy, GLOSSARY.md): presets, filter sets and panels go to one or more named users as one card
// each. The server reads Me and Everyone items itself; This browser items go whole (inboxrules.js sendBody).

import { el, on, S } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { layout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { ask, inboxUser } from './inbox.js';
import { sendBody } from './inboxrules.js';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal pb-modal--sm sode-send');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'send-title');
dialog.tabIndex = -1;
const what = el('p');
const people = el('div', 'sode-send__people');
people.setAttribute('role', 'group');
people.setAttribute('aria-label', 'Send to');
const body = el('div', 'pb-modal__body', what, el('span', 'sode__label', 'Send to'), people);
const note = el('span', 'sode__hint');
note.id = 'send-note';
note.setAttribute('role', 'status');
const sendBtn = btn('pb-btn--ok', 'Send', () => send());
sendBtn.id = 'send-go';
const closeBtn = btn('pb-btn--outline', 'Close', () => close());
dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Send a copy'), { id: 'send-title' })),
  body,
  el('div', 'pb-modal__footer', closeBtn, note, sendBtn),
);
root.append(dialog);

let dlg = null;
let items = null;
let back = null;
let busy = false;

const picked = () => [...people.querySelectorAll('input:checked')].map((b) => b.value);
const setNote = (t, error = false) => {
  note.textContent = t;
  note.classList.toggle('sode__error-text', error);
};

async function send() {
  const to = picked();
  if (busy || !to.length) return;
  busy = sendBtn.disabled = true;
  setNote('Sending…');
  const r = await ask({ t: 'inboxSend', inbox: sendBody(S.routerUid, to, items) });
  busy = false;
  sendBtn.disabled = !picked().length;
  if (!r.ok) return setNote(r.err, true);
  setNote(`Sent to ${to.join(', ')}.`);
  for (const b of people.querySelectorAll('input')) b.checked = false;
  sendBtn.disabled = true;
}

// list: { presets, filterSets, panels }, each entry with a name; name says what goes.
export function sendCopy(list, name) {
  if (!dlg) {
    document.body.append(root);
    dlg = trapDialog(root, dialog, close);
  }
  items = list;
  back = document.activeElement;
  setNote('');
  what.textContent = `Send a copy of ${name}`;
  const desk = layout() !== 'phone';
  sendBtn.hidden = !desk;
  const names = (S.users || []).map((u) => u.name).filter((n) => n.toLowerCase() !== inboxUser().toLowerCase());
  people.replaceChildren(
    ...(desk
      ? names.map((n) => {
          const b = el('input', 'pb-check__input');
          b.type = 'checkbox';
          b.value = n;
          b.addEventListener('change', () => (sendBtn.disabled = busy || !picked().length));
          return el('label', 'pb-check', b, el('span', 'pb-check__label', n));
        })
      : [desktopOnly('send a copy')]),
  );
  if (desk && !names.length) people.append(el('p', 'sode__hint', 'No other users'));
  sendBtn.disabled = true;
  dlg.show();
  (people.querySelector('input') || dialog).focus();
}

function close() {
  items = null;
  dlg?.hide();
  if (back?.isConnected) back.focus();
}

on('locking', () => {
  if (items) close();
});
