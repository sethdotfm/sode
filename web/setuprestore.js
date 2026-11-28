// Restore from a backup on the setup wizard's Welcome (internal/app/setuprestore.go). The server gives the browser
// that runs setup a key for POST api/setup/restore. A file with no PINs is answered pin_needed, and the page asks
// for a new PIN for the file's first Administrator. After the restore the page reloads into sign-in.

import { $, el, request } from './core.js';
import { applyBrowser } from './browserapply.js';
import { restorePinProblem, pinMistake, restoreFailedText } from './setupwizardrules.js';
import { pinShow } from './pinshow.js';

const KEY_HEADER = 'X-Sode-Setup'; // setupKeyHeader in internal/app/setuprestore.go

let busy = false; // a restore is sent: its hello comes before its answer, so the wizard must stay
let hooks = null; // {max(), back()} from setupwizard.js

let page = null; // the restore page while it shows: {next(), back(), check()} for the wizard's footer

export const restoring = () => busy;
// setupwizard.js sends its Back and Next clicks here while the restore page shows.
export const restorePage = () => page;
// The wizard draws a step again (Take control, Run setup again): the old restore handlers must not catch Back and Next.
export const leaveRestore = () => {
  page = null;
};

async function restoreKey() {
  const m = await request('setupRestoreKeyResult', { t: 'setupRestoreKey' });
  if (!m.ok) throw new Error(m.err || 'Sode refused the restore.');
  return m.key;
}

// kind: 'busy' or 'hint' show muted, as the wizard's footer does; '' is an error.
function say(text, kind = '') {
  const n = $('setup-err');
  n.textContent = text;
  n.classList.toggle('is-hint', kind === 'hint');
  n.classList.toggle('is-busy', kind === 'busy');
}

async function send(file, pin = '') {
  busy = true;
  say('Restoring…', 'busy');
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
      page?.check();
      return;
    }
    applyBrowser(out?.browser, true);
    const failed = Object.keys(out?.failed || {});
    if (failed.length) return showFailed(failed.length);
    location.reload();
  } catch (e) {
    busy = false;
    say(e.message);
    page?.check();
  }
}

// The restore page uses the wizard's footer: Back on the left, the main action as Next on the right. A step chip would
// drop the picked file, so the chips go nowhere while it shows.
function showPage(title, body, next, back = null) {
  page = { next: next.go, back: back || (() => {}), check: next.check || (() => {}) };
  $('setup-title').textContent = title;
  $('setup-body').replaceChildren(...body);
  $('setup-back').hidden = !back;
  $('setup-back').textContent = 'Back';
  $('setup-restore').hidden = true;
  $('setup-next').hidden = false;
  $('setup-next').textContent = next.label;
  $('setup-next').disabled = false;
  for (const c of $('setup-steps').querySelectorAll('[data-go]')) {
    c.removeAttribute('data-go');
    c.tabIndex = -1;
  }
  say('');
  page.check();
}

// The restore is written but some stores outside the config are not. Setup cannot run again, so say so before opening Sode.
function showFailed(n) {
  showPage('Restore from a backup', [el('p', 'sode-setup__lead', restoreFailedText(n))], { label: 'Open Sode', go: () => location.reload() });
  $('setup-next').focus();
}

// The file holds no PINs, so the first Administrator gets a new one here.
function askPin(file, text) {
  const field = (id, label) => {
    const input = el('input', 'pb-input');
    Object.assign(input, { id, type: 'password', inputMode: 'numeric', autocomplete: 'off' });
    const lab = el('label', '', label);
    lab.htmlFor = id;
    return [el('div', 'sode__field', lab, pinShow(input)), input];
  };
  const [pinBox, pin] = field('setup-restore-pin', 'New PIN');
  const [pin2Box, pin2] = field('setup-restore-pin2', 'Repeat PIN');
  const problem = () => restorePinProblem(pin.value, pin2.value, hooks.max());
  const check = () => ($('setup-next').disabled = busy || !!problem());
  const go = () => {
    if (busy || problem()) return;
    $('setup-next').disabled = true;
    send(file, pin.value);
  };
  const back = () => {
    page = null;
    hooks.back();
  };
  // #setup-body presses Next on Enter.
  for (const i of [pin, pin2]) {
    i.addEventListener('input', () => {
      check();
      if (!busy) say(problem(), pinMistake(pin.value, pin2.value, hooks.max()) ? '' : 'hint');
    });
  }
  showPage(
    'Restore from a backup',
    [el('p', 'sode-setup__lead', text), el('div', 'sode-setup__section', el('div', 'sode-setup__row', pinBox, pin2Box))],
    { label: 'Restore', go, check },
    back,
  );
  say(problem(), 'hint'); // Restore is off, so the page says why
  pin.focus();
}

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
