// PIN recheck (GLOSSARY.md): the server refused a Critical action with `recheck: true`. askRecheck() asks the signed-in
// user's own PIN, sends `pinRecheck`, and resolves true when the server accepts it, so the caller sends its action again.
// The recheck is per connection and holds one minute (docs/agents/users-locks.md).

import { el, S, on, onMsg, wsSend, sentence } from './core.js';
import { trapDialog } from './dialogkeys.js';

let ui = null;
let done = null; // resolves the askRecheck in flight
let seq = 0;
let waiting = 0; // id of the pinRecheck in flight

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
  waiting = 0;
  settle(ok);
}

function fail(text) {
  ui.err.textContent = sentence(text);
  ui.err.hidden = false;
  ui.input.select();
}

function submit() {
  if (ui.go.disabled) return;
  ui.err.hidden = true;
  const id = ++seq;
  if (!wsSend({ t: 'pinRecheck', pin: ui.input.value, id })) return fail('Not connected to Sode.');
  waiting = id;
  ui.go.disabled = true;
}

const inline = new Map(); // id -> resolve, for recheckPin

onMsg('pinRecheckResult', (m) => {
  const own = inline.get(m.id);
  if (own) {
    inline.delete(m.id);
    return own({ ok: !!m.ok, err: m.err || (m.ok ? '' : 'Wrong PIN.') });
  }
  if (!ui || m.id !== waiting) return;
  waiting = 0;
  ui.go.disabled = false;
  if (m.ok) close(true);
  else fail(m.err || 'Wrong PIN.');
});

function build() {
  if (ui) return;
  const input = el('input', 'pb-input');
  input.type = 'password';
  input.inputMode = 'numeric';
  input.id = 'recheck-pin';
  input.autocomplete = 'current-password';
  input.spellcheck = false;
  input.placeholder = 'Your PIN';
  input.setAttribute('aria-label', 'Your PIN');
  const why = el('p', 'sode__hint');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const cancel = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancel.type = 'button';
  cancel.id = 'recheck-cancel';
  cancel.addEventListener('click', () => close(false));
  const go = el('button', 'pb-btn', 'Continue');
  go.type = 'button';
  go.id = 'recheck-go';
  go.addEventListener('click', submit);

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal sode__filters sode__pinprompt');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'recheck-title');
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'PIN recheck'), { id: 'recheck-title' })),
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
  ui = { root, input, why, err, go, dlg: trapDialog(root, dialog, () => close(false)) };
}

// what names the action, for example 'Clearing History'.
export function askRecheck(what) {
  build();
  settle(false); // a second ask replaces the first
  ui.why.textContent = `${what} is a Critical action. Type your PIN again.`;
  ui.dlg.show();
  ui.input.focus();
  return new Promise((resolve) => {
    done = resolve;
  });
}

// For a page with its own PIN field, such as the import review: sends the PIN and resolves {ok, err}.
export function recheckPin(pin) {
  const id = ++seq;
  if (!wsSend({ t: 'pinRecheck', pin, id })) return Promise.resolve({ ok: false, err: 'Not connected to Sode.' });
  return new Promise((resolve) => inline.set(id, resolve));
}

// A Lock or a socket drop ends the recheck on the server, so the prompt closes too.
on('locking', () => close(false));
on('conn', () => {
  if (S.wsState === 'up') return;
  close(false);
  for (const done of inline.values()) done({ ok: false, err: 'Not connected to Sode.' });
  inline.clear();
});
