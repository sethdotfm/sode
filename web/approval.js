// The server checks the one message with `approval` against the approver's Role; nothing stays on this browser.
// send returns false when it could not send; the caller then calls answer.fail or answer.done.
// req: req.action() is the message the server holds for Request approval (approvalcards.js); req.onEnd hears the end.

import { el, S, on, sentence, LINK_DOWN } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { namePicker } from './namepick.js';
import { requestApproval, canApprove } from './approvalcards.js';
import { approverKey } from './approvalcardrules.js';

let ui = null;
let send = null;
let ask = null; // { what, who, sendFn, req } of the open popup

function close() {
  if (!ui) return;
  ui.dlg.hide();
  ui.pin.value = '';
  ui.err.hidden = true;
  ui.go.disabled = false;
  send = null;
  ask = null;
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
  if (!user) return fail("Enter the approver's name.");
  ui.err.hidden = true;
  if (!send({ user, pin: ui.pin.value }, { fail, done: close })) return fail(LINK_DOWN);
  ui.go.disabled = true;
}

function paintRequest() {
  if (!ui || !ask?.req) return;
  const ok = canApprove(approverKey(ask.who));
  ui.request.disabled = !ok;
  if (ok) delete ui.request.dataset.pbTip;
  else ui.request.dataset.pbTip = 'No approver signed in';
}

async function sendRequest() {
  if (!ask?.req || ui.request.disabled) return;
  const { what, who, sendFn, req } = ask;
  ui.request.disabled = true;
  const err = await requestApproval(req.action(), { what, onEnd: req.onEnd, onTimeout: () => askApproval(what, who, sendFn, req) });
  if (ask?.req !== req) return;
  if (!err) return close();
  paintRequest();
  ui.err.textContent = err;
  ui.err.hidden = false;
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
  const request = button('approval-request', 'Request approval', 'pb-btn--outline', sendRequest);
  const requestRow = el('div', '', request);

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal sode__filters sode__pinprompt');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'approval-title');
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Approval'), { id: 'approval-title' })),
    el('div', 'pb-modal__body', why, picker.root, pin, err, requestRow),
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
  ui = { dlg: trapDialog(root, dialog, close), picker, pin, why, err, go, request, requestRow };
}

// what names the action, for example 'Saving these changes'; who names the Role it needs.
export function askApproval(what, who, sendFn, req = null) {
  build();
  close();
  send = sendFn;
  ask = { what, who, sendFn, req };
  ui.requestRow.hidden = !req;
  paintRequest();
  ui.why.textContent = `${what}: Approval from ${who}.`;
  ui.picker.set('');
  ui.dlg.show();
  ui.picker.input.focus();
}

// An Approval needs the Lock dismissed, and a socket drop loses the answer.
on('locking', close);
on('approvers', paintRequest);
on('conn', () => {
  if (S.wsState !== 'up') close();
});
