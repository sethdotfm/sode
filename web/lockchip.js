// The "Logged in as" chip of the lock screen (#151) and what hangs off it: a
// small options menu (change my PIN, never auto-lock this browser, forget
// this browser) and the change-PIN dialog. lock.js owns the state and passes
// the actions in; this builds the DOM with el() only.

import { el, S, onMsg, wsSend, sentence } from './core.js';
import { trapDialog } from './dialogkeys.js';

const field = (id, label, auto) => {
  const input = el('input', 'pb-input');
  input.type = 'password';
  input.id = id;
  input.inputMode = 'numeric';
  input.autocomplete = auto;
  input.spellcheck = false;
  return { input, row: el('div', 'sode-lock__field', Object.assign(el('label', null, label), { htmlFor: id }), input) };
};

// api: { lock(), forget(), never() -> bool, setNever(bool) }.
export function buildChip(api) {
  const name = el('span', 'sode-lock__who-name');
  const lockBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Lock');
  lockBtn.type = 'button';
  lockBtn.title = 'Lock (Ctrl+L)';
  const optBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Options');
  optBtn.type = 'button';
  optBtn.setAttribute('aria-expanded', 'false');
  optBtn.setAttribute('aria-controls', 'sode-lock-menu');

  const pinBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Change my PIN');
  pinBtn.type = 'button';
  const neverBox = el('input', 'pb-check__input');
  neverBox.type = 'checkbox';
  neverBox.id = 'sode-lock-never';
  const neverRow = el('label', 'pb-check', neverBox, el('span', 'pb-check__label', 'Never auto-lock this browser'));
  const forgetBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Forget this browser');
  forgetBtn.type = 'button';
  forgetBtn.title = 'Stop skipping the lock on this browser';
  const menu = el('div', 'sode-lock__menu', pinBtn, neverRow, forgetBtn);
  menu.id = 'sode-lock-menu';
  menu.hidden = true;

  const root = el('div', 'sode-lock__who', el('div', 'sode-lock__who-bar', name, lockBtn, optBtn), menu);
  root.hidden = true;

  const setMenu = (open) => {
    menu.hidden = !open;
    optBtn.setAttribute('aria-expanded', String(open));
  };
  optBtn.addEventListener('click', () => setMenu(menu.hidden));
  root.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !menu.hidden) { e.stopPropagation(); setMenu(false); optBtn.focus(); }
  });
  document.addEventListener('pointerdown', (e) => { if (!menu.hidden && !root.contains(e.target)) setMenu(false); }, true);
  lockBtn.addEventListener('click', () => { setMenu(false); api.lock(); });
  neverBox.addEventListener('change', () => api.setNever(neverBox.checked));
  forgetBtn.addEventListener('click', () => { setMenu(false); api.forget(); });
  pinBtn.addEventListener('click', () => { setMenu(false); openPin(); });

  // ---- change my PIN
  const oldF = field('sode-pin-old', 'Current PIN', 'current-password');
  const newF = field('sode-pin-new', 'New PIN', 'new-password');
  const againF = field('sode-pin-again', 'New PIN again', 'new-password');
  const note = el('p', 'sode-lock__pin-note');
  note.setAttribute('role', 'status');
  const cancel = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancel.type = 'button';
  const save = el('button', 'pb-btn pb-btn--ok', 'Change PIN');
  save.type = 'submit';
  const form = el('form', 'pb-modal__body sode-lock__pin-form', oldF.row, newF.row, againF.row, note,
    el('div', 'pb-modal__footer', cancel, save));
  const dialog = el('div', 'pb-modal', el('div', 'pb-modal__header', el('span', 'pb-modal__title', 'Change my PIN')), form);
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-label', 'Change my PIN');
  dialog.tabIndex = -1;
  const back = el('div', 'pb-modal-backdrop sode-lock__pin', dialog);
  back.hidden = true;
  document.body.append(back);

  let seq = 0;
  let focusBack = null;
  const say = (text, bad) => {
    note.textContent = text;
    note.className = `sode-lock__pin-note${bad ? ' pb-note pb-note--error' : ''}`;
  };
  function closePin() {
    back.hidden = true;
    seq++; // a late answer no longer counts
    for (const f of [oldF, newF, againF]) f.input.value = '';
    save.disabled = false;
    if (focusBack?.isConnected) focusBack.focus({ preventScroll: true });
  }
  function openPin() {
    focusBack = document.activeElement;
    say('');
    back.hidden = false;
    oldF.input.focus({ preventScroll: true });
  }
  trapDialog(back, dialog, closePin);
  cancel.addEventListener('click', closePin);
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    const max = S.cfg?.maxPinLen || 8;
    const [o, n, a] = [oldF.input.value, newF.input.value, againF.input.value];
    if (!/^[0-9]+$/.test(o)) return say('Enter your current PIN.', true);
    if (!/^[0-9]+$/.test(n) || n.length < 4 || n.length > max) return say(`A PIN is 4 to ${max} digits.`, true);
    if (n !== a) return say('The new PINs differ.', true);
    const id = ++seq;
    if (!wsSend({ t: 'userSetPin', old: o, new: n, id })) return say('Not connected to Sode.', true);
    save.disabled = true;
    say('Checking…');
  });
  onMsg('userSetPinResult', (m) => {
    if (m.id !== seq || back.hidden) return;
    save.disabled = false;
    if (!m.ok) return say(sentence(m.err || 'Could not change the PIN.'), true);
    closePin();
  });

  // update: render the chip for lock state L.
  function update(L, chip) {
    root.hidden = !chip;
    if (!chip) { setMenu(false); return; }
    name.textContent = `Logged in as ${L.user || 'admin'}`;
    optBtn.hidden = !(L.user || L.token);
    pinBtn.hidden = !L.user;
    neverRow.hidden = !L.token;
    forgetBtn.hidden = !L.token;
    neverBox.checked = !!api.never();
  }
  return { root, update, menuOpen: () => !menu.hidden };
}
