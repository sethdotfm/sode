// Popup for the settings PIN when something asks mid-session (users, config file, clear history, refused save).
// askPin() resolves true once accepted, false if cancelled or closed.

import { el, S, on } from './core.js';
import { unlock, lostUnlock } from './pin.js';
import { trapDialog } from './dialogkeys.js';

let ui = null;
let done = null; // resolves the askPin in flight

function settle(ok) {
  const d = done;
  done = null;
  d?.(ok);
}

function close(ok) {
  if (!ui) return;
  ui.dlg.hide();
  ui.input.value = '';
  ui.err.hidden = true;
  ui.go.disabled = false;
  settle(ok);
}

async function submit() {
  if (ui.go.disabled) return;
  ui.err.hidden = true;
  ui.go.disabled = true;
  const r = await unlock(ui.input.value);
  if (r.superseded) return;
  ui.go.disabled = false;
  if (r.stale) return;
  if (!r.ok) {
    ui.err.textContent = r.err || 'Wrong PIN.';
    ui.err.hidden = false;
    ui.input.select();
    return;
  }
  close(true);
}

function build() {
  if (ui) return;
  const input = el('input', 'pb-input');
  input.type = 'password';
  input.id = 'pinprompt-pin';
  input.autocomplete = 'current-password';
  input.spellcheck = false;
  input.placeholder = 'Please enter the administrator PIN';
  input.setAttribute('aria-label', 'Advanced Settings PIN');
  const why = el('p', 'sode__hint');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const cancel = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancel.type = 'button';
  cancel.id = 'pinprompt-cancel';
  cancel.addEventListener('click', () => close(false));
  const go = el('button', 'pb-btn', 'Open');
  go.type = 'button';
  go.addEventListener('click', submit);

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal sode__filters sode__pinprompt');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'pinprompt-title');
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Advanced Settings'), { id: 'pinprompt-title' })),
    el('div', 'pb-modal__body', why, input, err),
    el('div', 'pb-modal__footer', cancel, go),
  );
  input.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    submit();
  });
  root.append(dialog);
  document.body.append(root);
  ui = { root, dialog, input, why, err, go, dlg: trapDialog(root, dialog, () => close(false)) };
}

export function askPin(text = '') {
  build();
  settle(false); // a second ask replaces the first
  ui.why.textContent = text;
  ui.why.hidden = !text;
  ui.dlg.show();
  ui.input.focus();
  return new Promise((resolve) => {
    done = resolve;
  });
}

// Refused for want of the PIN: ask again; true once entered so the caller retries.
export function reask(text) {
  lostUnlock();
  return askPin(text);
}

// A lock or socket drop closes the popup; the PIN removed (hello without one) lets the asker go ahead.
on('locking', () => close(false));
on('conn', () => {
  if (S.wsState !== 'up') close(false);
});
on('hello', () => {
  if (!S.pinSet) close(true);
});
