// Opens by itself one time per sign-in for a PIN needed user. Rules in lockrules.js (pinForm).

import { el, S, on, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
import { btn } from './formbits.js';
import { modal } from './modal.js';
import { listedUser } from './namepickrules.js';
import { pinForm, pinNeededAsk } from './lockrules.js';

const field = (id, label, auto) => {
  const input = el('input', { class: 'pb-input', type: 'password', id, inputMode: 'numeric', autocomplete: auto, spellcheck: false });
  return { input, row: el('div', 'sode-lock__field', Object.assign(el('label', null, label), { htmlFor: id }), input) };
};

export function buildChangePin() {
  const oldF = field('sode-pin-old', 'Current PIN', 'current-password');
  const newF = field('sode-pin-new', 'New PIN', 'new-password');
  const againF = field('sode-pin-again', 'New PIN again', 'new-password');
  const note = el('p', { class: 'sode-lock__pin-note', role: 'status' });
  const cancel = btn('pb-btn--outline', 'Cancel', () => closePin());
  // Remove reads the current PIN from the same field.
  const remove = btn('pb-btn--ghost', 'Remove my PIN');
  const lead = el('p', 'sode-lock__pin-note');
  const form = el('form', { class: 'pb-modal__body sode-lock__pin-form', id: 'sode-pin-form' }, lead, oldF.row, newF.row, againF.row, note);
  const save = el('button', { class: 'pb-btn pb-btn--ok', type: 'submit' }, 'Change PIN');
  save.setAttribute('form', form.id); // the footer is the form's sibling
  const dlg = modal({
    titleId: 'sode-pin-title',
    title: 'Change my PIN',
    cls: 'sode-lock__pin-dlg',
    rootCls: 'sode-lock__pin',
    body: form,
    foot: [remove, cancel, save],
    onEsc: () => closePin(),
  });
  const back = dlg.root;

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
  on('wsclose', () => {
    if (back.hidden || !save.disabled) return;
    save.disabled = remove.disabled = false;
    say(LINK_DOWN, true);
  });
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
