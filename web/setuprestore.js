// Restore from a backup on the setup wizard's Welcome (internal/app/setuprestore.go). The server gives the browser
// that runs setup a key for POST api/setup/restore. A file with no PINs is answered pin_needed, and the page asks
// for a new PIN for the file's first Administrator. After the restore the page reloads into sign-in.

import { $, el, onMsg, wsSend, LINK_DOWN } from './core.js';
import { applyBrowser } from './browserapply.js';
import { restorePinProblem } from './setupwizardrules.js';

const KEY_HEADER = 'X-Sode-Setup'; // setupKeyHeader in internal/app/setuprestore.go

let busy = false; // a restore is sent: its hello comes before its answer, so the wizard must stay
let seq = 0;
let hooks = null; // {max(), back()} from setupwizard.js
const asks = new Map();

export const restoring = () => busy;

onMsg('setupRestoreKeyResult', (m) => {
  const done = asks.get(m.id);
  asks.delete(m.id);
  done?.(m);
});

function restoreKey() {
  const id = ++seq;
  return new Promise((resolve, reject) => {
    if (!wsSend({ t: 'setupRestoreKey', id })) return reject(new Error(LINK_DOWN));
    asks.set(id, (m) => (m.ok ? resolve(m.key) : reject(new Error(m.err || 'Sode refused the restore.'))));
  });
}

const say = (text) => ($('setup-err').textContent = text);

async function send(file, pin = '') {
  busy = true;
  say('Restoring…');
  try {
    const key = await restoreKey();
    const body = new FormData();
    body.append('plan', new Blob([JSON.stringify({ pin })], { type: 'application/json' }));
    body.append('file', file);
    const res = await fetch('api/setup/restore', { method: 'POST', headers: { [KEY_HEADER]: key }, body, cache: 'no-store' });
    const out = await res.json().catch(() => null);
    if (!res.ok) {
      busy = false;
      const code = out?.error?.code || '';
      const text = out?.error?.message || `Sode answered ${res.status}.`;
      if (code === 'pin_needed') return askPin(file, text);
      say(text);
      return;
    }
    applyBrowser(out?.browser, true);
    const failed = Object.keys(out?.failed || {});
    if (failed.length) return showFailed(failed.length);
    location.reload();
  } catch (e) {
    busy = false;
    say(e.message);
  }
}

// showFailed: the restore is written but some stores outside the config are not. Setup cannot run again, so the
// page says so before it opens Sode.
function showFailed(n) {
  const open = el('button', 'pb-btn pb-btn--ok', 'Open Sode');
  open.type = 'button';
  open.addEventListener('click', () => location.reload());
  $('setup-title').textContent = 'Restore from a backup';
  $('setup-body').replaceChildren(
    el('p', 'sode-setup__lead', `Restored. ${n} ${n === 1 ? 'item' : 'items'} not written: check panels and artwork.`),
    el('div', 'sode-setup__restore', open),
  );
  for (const id of ['setup-back', 'setup-restore', 'setup-next']) $(id).hidden = true;
  say('');
  open.focus();
}

// askPin: the file holds no PINs, so the first Administrator gets a new one here.
function askPin(file, text) {
  const field = (id, label) => {
    const input = el('input', 'pb-input');
    Object.assign(input, { id, type: 'password', inputMode: 'numeric', autocomplete: 'off' });
    const lab = el('label', '', label);
    lab.htmlFor = id;
    return [el('div', 'sode__field', lab, input), input];
  };
  const [pinBox, pin] = field('setup-restore-pin', 'New PIN');
  const [pin2Box, pin2] = field('setup-restore-pin2', 'PIN again');
  const go = el('button', 'pb-btn pb-btn--ok', 'Restore');
  const back = el('button', 'pb-btn pb-btn--outline', 'Back');
  go.type = back.type = 'button';
  const problem = () => restorePinProblem(pin.value, pin2.value, hooks.max());
  const check = () => (go.disabled = busy || !!problem());
  go.addEventListener('click', () => {
    if (problem()) return;
    go.disabled = true;
    send(file, pin.value);
  });
  back.addEventListener('click', () => hooks.back());
  for (const i of [pin, pin2]) {
    i.addEventListener('input', () => {
      check();
      if (!busy) say(problem());
    });
    i.addEventListener('keydown', (e) => {
      if (e.key !== 'Enter') return;
      // #setup-body would also press the hidden Next.
      e.preventDefault();
      e.stopPropagation();
      if (!go.disabled) go.click();
    });
  }
  $('setup-title').textContent = 'Restore from a backup';
  $('setup-body').replaceChildren(
    el('p', 'sode-setup__lead', text),
    el('div', 'sode-setup__section', pinBox, pin2Box, el('div', 'sode-setup__restore', back, go)),
  );
  for (const id of ['setup-back', 'setup-restore', 'setup-next']) $(id).hidden = true;
  say('');
  check();
  pin.focus();
}

// wireRestore: the Restore from a backup button picks a file and sends it.
export function wireRestore(h) {
  hooks = h;
  const input = el('input');
  Object.assign(input, { type: 'file', accept: '.zip,application/zip', hidden: true, id: 'setup-restore-file' });
  $('setup-restore').after(input);
  $('setup-restore').addEventListener('click', () => {
    if (!busy) input.click();
  });
  input.addEventListener('change', () => {
    const file = input.files[0];
    input.value = '';
    if (file) send(file);
  });
}
