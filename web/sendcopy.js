// The Send dialog (Send a copy, GLOSSARY.md): presets, filter sets and panels go to one or more named users as one card
// each. The server reads Me and Everyone items itself; This browser items go whole (inboxrules.js sendBody).

import { el, S } from './core.js';
import { btn } from './formbits.js';
import { modal } from './modal.js';
import { layout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { ask, inboxUser } from './inbox.js';
import { sendBody } from './inboxrules.js';

const people = el('div', { class: 'sode-send__people', role: 'group', 'aria-label': 'Send to' });
const note = el('span', { class: 'sode__hint', id: 'send-note', role: 'status' });
const sendBtn = btn('pb-btn--ok', 'Send', () => send(), 'send-go');
let dlg = null;
const ui = () =>
  (dlg ||= modal({
    titleId: 'send-title',
    title: 'Send a copy',
    cls: 'pb-modal--sm sode-send',
    body: [el('span', 'sode__label', 'Send to'), people],
    foot: [btn('pb-btn--outline', 'Close', () => close()), note, sendBtn],
    onEsc: () => close(),
  }));
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
  ui();
  items = list;
  back = document.activeElement;
  setNote('');
  dlg.title.textContent = `Send a copy of ${name}`;
  const desk = layout() !== 'phone';
  sendBtn.hidden = !desk;
  const names = (S.users || []).map((u) => u.name).filter((n) => n.toLowerCase() !== inboxUser().toLowerCase());
  people.replaceChildren(
    ...(desk
      ? names.map((n) => {
          const b = el('input', { class: 'pb-check__input', type: 'checkbox', value: n });
          b.addEventListener('change', () => (sendBtn.disabled = busy || !picked().length));
          return el('label', 'pb-check', b, el('span', 'pb-check__label', n));
        })
      : [desktopOnly('send a copy')]),
  );
  if (desk && !names.length) people.append(el('p', 'sode__hint', 'No other users'));
  sendBtn.disabled = true;
  dlg.show();
  (people.querySelector('input') || dlg.dialog).focus();
}

function close() {
  items = null;
  dlg?.hide();
  if (back?.isConnected) back.focus();
}
