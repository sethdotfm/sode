// Approval in person (GLOSSARY.md): a person beside this browser types their name and PIN, and the action goes again
// with `approval`. The server checks that one message against the approver's Role; nothing stays on this browser.
// send(approval, answer) sends the action and returns false when it could not. The caller then calls answer.fail(text)
// for a refused Approval (the popup stays open) or answer.done() for any other answer.

import { el, S, on, sentence } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { namePicker } from './namepick.js';

let ui = null;
let send = null;

function close() {
  if (!ui) return;
  ui.dlg.hide();
  ui.pin.value = '';
  ui.err.hidden = true;
  ui.go.disabled = false;
  send = null;
}

function fail(text) {
  if (!ui || !send) return;
  ui.go.disabled = false;
  ui.pin.value = '';
  ui.err.textContent = sentence(text);
  ui.err.hidden = false;
  ui.pin.focus();
}

function submit() {
  if (ui.go.disabled || !send) return;
  const user = ui.picker.value().trim();
  if (!user) return fail('Type the name of the person who approves.');
  ui.err.hidden = true;
  if (!send({ user, pin: ui.pin.value }, { fail, done: close })) return fail('Not connected to Sode.');
  ui.go.disabled = true;
}

function button(id, label, cls, fn) {
  const b = el('button', `pb-btn ${cls}`, label);
  b.type = 'button';
  b.id = id;
  b.addEventListener('click', fn);
  return b;
}

function build() {
  if (ui) return;
  const picker = namePicker({ id: 'approval-name', users: () => S.users || [], onInput: () => {}, onPick: () => ui.pin.focus() });
  picker.input.placeholder = 'Name';
  picker.input.setAttribute('aria-label', 'Name of the person who approves');
  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.inputMode = 'numeric';
  pin.id = 'approval-pin';
  pin.autocomplete = 'off';
  pin.placeholder = 'PIN';
  pin.setAttribute('aria-label', 'PIN of the person who approves');
  const why = el('p', 'sode__hint');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const go = button('approval-go', 'Approve', '', submit);

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal sode__filters sode__pinprompt');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'approval-title');
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Approval'), { id: 'approval-title' })),
    el('div', 'pb-modal__body', why, picker.root, pin, err),
    el('div', 'pb-modal__footer', button('approval-cancel', 'Cancel', 'pb-btn--outline', close), go),
  );
  root.append(dialog);
  document.body.append(root);
  picker.input.addEventListener('keydown', (e) => {
    if (picker.key(e)) e.preventDefault();
  });
  pin.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    submit();
  });
  ui = { dlg: trapDialog(root, dialog, close), picker, pin, why, err, go };
}

// what names the action, for example 'Saving these changes'; who names the Role it needs.
export function askApproval(what, who, sendFn) {
  build();
  close();
  send = sendFn;
  ui.why.textContent = `${what} needs Approval from ${who}. That person types their name and PIN here. The Approval covers this one action.`;
  ui.picker.set('');
  ui.dlg.show();
  ui.picker.input.focus();
}

// An Approval needs the Lock dismissed, and a socket drop loses the answer.
on('locking', close);
on('conn', () => {
  if (S.wsState !== 'up') close();
});
