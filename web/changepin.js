// Opens by itself one time per sign-in for a PIN needed user. Rules in lockrules.js (pinForm).

import { el, S, on, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { listedUser } from './namepickrules.js';
import { pinForm, pinNeededAsk } from './lockrules.js';

const field = (id, label, auto) => {
  const input = el('input', 'pb-input');
  input.type = 'password';
  input.id = id;
  input.inputMode = 'numeric';
  input.autocomplete = auto;
  input.spellcheck = false;
  return { input, row: el('div', 'sode-lock__field', Object.assign(el('label', null, label), { htmlFor: id }), input) };
};

export function buildChangePin() {
  const oldF = field('sode-pin-old', 'Current PIN', 'current-password');
  const newF = field('sode-pin-new', 'New PIN', 'new-password');
  const againF = field('sode-pin-again', 'New PIN again', 'new-password');
  const note = el('p', 'sode-lock__pin-note');
  note.setAttribute('role', 'status');
  const cancel = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancel.type = 'button';
  // Remove reads the current PIN from the same field.
  const remove = el('button', 'pb-btn pb-btn--ghost', 'Remove my PIN');
  remove.type = 'button';
  const save = el('button', 'pb-btn pb-btn--ok', 'Change PIN');
  save.type = 'submit';
  const lead = el('p', 'sode-lock__pin-note');
  const form = el('form', 'pb-modal__body sode-lock__pin-form', lead, oldF.row, newF.row, againF.row, note);
  form.id = 'sode-pin-form';
  save.setAttribute('form', form.id); // the footer is the form's sibling
  const dialog = el(
    'div',
    'pb-modal sode-lock__pin-dlg',
    el('div', 'pb-modal__header', el('span', 'pb-modal__title', 'Change my PIN')),
    form,
    el('div', 'pb-modal__footer', remove, cancel, save),
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
  let mode = pinForm(null, '');
  let me = { user: '', role: '', pinNeeded: false };
  let asked = '';
  const say = (text, bad) => {
    note.textContent = text;
    note.className = `sode-lock__pin-note${bad ? ' pb-note pb-note--error' : ''}`;
  };
  function closePin() {
    dlg.hide();
    seq++; // a late answer no longer counts
    for (const f of [oldF, newF, againF]) f.input.value = '';
    save.disabled = remove.disabled = false;
    if (focusBack?.isConnected) focusBack.focus({ preventScroll: true });
  }
  function openPin() {
    mode = pinForm(listedUser(S.users, me.user), me.role);
    oldF.row.hidden = !mode.current;
    remove.hidden = !mode.remove;
    save.textContent = mode.save;
    lead.textContent = me.pinNeeded ? 'No PIN: you act with Guest Rights.' : '';
    lead.hidden = !me.pinNeeded;
    focusBack = document.activeElement;
    say('');
    dlg.show();
    (mode.current ? oldF : newF).input.focus({ preventScroll: true });
  }
  const dlg = trapDialog(back, dialog, closePin);
  on('changePin', openPin);
  on('access', (L) => {
    me = { user: L.user || '', role: L.role || '', pinNeeded: !!L.pinNeeded };
    if (!me.user) asked = '';
    const ask = pinNeededAsk(me, asked);
    if (ask && back.hidden) {
      asked = ask;
      openPin();
    }
  });
  cancel.addEventListener('click', closePin);
  const sendSet = (msg) => {
    const id = ++seq;
    if (!wsSend({ t: 'userSetPin', id, ...msg })) return say(LINK_DOWN, true);
    save.disabled = remove.disabled = true;
    say('Checking…');
  };
  remove.addEventListener('click', () => {
    const o = oldF.input.value;
    if (!/^[0-9]+$/.test(o)) return say('Enter your current PIN.', true);
    sendSet({ old: o, new: '', remove: true });
  });
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    const max = S.cfg?.maxPinLen || 8;
    const [o, n, a] = [oldF.input.value, newF.input.value, againF.input.value];
    if (mode.current && !/^[0-9]+$/.test(o)) return say('Enter your current PIN.', true);
    if (!/^[0-9]+$/.test(n) || n.length < 4 || n.length > max) return say(`A PIN is 4 to ${max} digits.`, true);
    if (n !== a) return say('The PINs differ.', true);
    sendSet({ old: mode.current ? o : '', new: n });
  });
  onMsg('userSetPinResult', (m) => {
    if (m.id !== seq || back.hidden) return;
    save.disabled = remove.disabled = false;
    if (!m.ok) return say(sentence(m.err || 'Could not change the PIN.'), true);
    closePin();
  });
}
