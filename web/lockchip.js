// Lock screen's "Logged in as" chip: name and one lock button. Change PIN and logout live in Settings' Account block (bus 'changePin', 'logout');
// this owns the change-PIN dialog. lock.js owns the state.

import { el, S, on, onMsg, icon, wsSend, sentence } from './core.js';
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

export function buildChip(api) {
  const name = el('span', 'sode-lock__who-name');
  const lockBtn = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm pb-tip--start', icon('lock-open'));
  lockBtn.type = 'button';
  lockBtn.dataset.pbTip = 'Lock (Ctrl+L)';
  lockBtn.setAttribute('aria-label', 'Lock');
  lockBtn.addEventListener('click', () => api.lock());
  // Status with words, cross-faded with the button (is-hand) so the icon doesn't snap against the lock screen's border.
  const ico = el('span', 'sode-lock__who-ico', icon('lock'));
  const slot = el('div', 'sode-lock__who-slot', lockBtn, ico);
  const root = el('div', 'sode-lock__who', el('div', 'sode-lock__who-bar', slot, name));
  root.hidden = true;

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
  const form = el('form', 'pb-modal__body sode-lock__pin-form', oldF.row, newF.row, againF.row, note);
  form.id = 'sode-pin-form';
  save.setAttribute('form', form.id); // the footer is the form's sibling
  const dialog = el(
    'div',
    'pb-modal sode-lock__pin-dlg',
    el('div', 'pb-modal__header', el('span', 'pb-modal__title', 'Change my PIN')),
    form,
    el('div', 'pb-modal__footer', cancel, save),
  );
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-label', 'Change my PIN');
  dialog.tabIndex = -1;
  const back = el('div', 'pb-modal-backdrop sode__scrim sode-lock__pin', dialog);
  back.hidden = true;
  document.body.append(back);

  let seq = 0;
  let focusBack = null;
  const say = (text, bad) => {
    note.textContent = text;
    note.className = `sode-lock__pin-note${bad ? ' pb-note pb-note--error' : ''}`;
  };
  function closePin() {
    dlg.hide();
    seq++; // a late answer no longer counts
    for (const f of [oldF, newF, againF]) f.input.value = '';
    save.disabled = false;
    if (focusBack?.isConnected) focusBack.focus({ preventScroll: true });
  }
  function openPin() {
    focusBack = document.activeElement;
    say('');
    dlg.show();
    oldF.input.focus({ preventScroll: true });
  }
  const dlg = trapDialog(back, dialog, closePin);
  on('changePin', openPin);
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

  // pageLocked: the lock button does nothing, so it goes.
  function update(L, chip, pageLocked) {
    root.hidden = !chip;
    root.classList.toggle('is-hand', !!pageLocked);
    lockBtn.inert = !!pageLocked; // out of the tab order and unclickable while faded
    lockBtn.setAttribute('aria-hidden', String(!!pageLocked));
    ico.setAttribute('aria-hidden', String(!pageLocked));
    name.textContent = L.user ? `Signed in as ${L.user}` : '';
  }
  // Swaps the icon when the overlay's border starts to move.
  function setLocked(on) {
    lockBtn.replaceChildren(icon(on ? 'lock' : 'lock-open'));
  }
  return { root, update, setLocked };
}
