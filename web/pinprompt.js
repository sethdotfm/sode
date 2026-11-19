// The PIN prompts (Step in, Approval, PIN recheck): an optional name, a PIN and an error line. Enter in the PIN sends.
import { el, S, on, sentence, LINK_DOWN } from './core.js';
import { btn } from './formbits.js';
import { namePicker } from './namepick.js';
import { modal } from './modal.js';
import { pinShow, hidePin } from './pinshow.js';

// name: {placeholder, label, missing} adds a name picker. send(pin, name) returns false when it could not send; the
// caller then calls fail or close. onClose(sent) runs on every close. dropCloses: a socket drop loses the answer.
export function pinPrompt({ id, title, go: goText, name, pin: pinProps, extra = [], send, onClose = () => {}, dropCloses = false }) {
  let back = null;
  let open = false;
  const picker = name && namePicker({ id: `${id}-name`, users: () => S.users || [], onInput: () => {}, onPick: () => pin.focus() });
  if (picker) {
    picker.input.placeholder = name.placeholder;
    picker.input.setAttribute('aria-label', name.label);
    // Esc closes the name list, not the popup.
    picker.input.addEventListener('keydown', (e) => {
      if (!picker.key(e)) return;
      e.preventDefault();
      e.stopPropagation();
    });
  }
  const pin = el('input', { class: 'pb-input', type: 'password', inputMode: 'numeric', id: `${id}-pin`, autocomplete: 'off', ...pinProps });
  const why = el('p', { class: 'sode__hint', hidden: true });
  const err = el('div', { class: 'pb-note pb-note--error', role: 'alert', hidden: true });
  const go = btn('', goText, submit, `${id}-go`);
  const m = modal({
    titleId: `${id}-title`,
    title,
    cls: 'sode__filters sode__pinprompt',
    body: [why, ...(picker ? [picker.root] : []), pinShow(pin), err, ...extra],
    foot: [btn('pb-btn--outline', 'Cancel', () => close(false), `${id}-cancel`), go],
    onEsc: () => close(false),
  });
  pin.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    submit();
  });

  function close(ok) {
    if (!open) return;
    open = false;
    m.hide();
    pin.value = '';
    hidePin(pin);
    err.hidden = true;
    go.disabled = false;
    if (back?.isConnected) back.focus();
    back = null;
    onClose(ok);
  }

  // Selected, not cleared: the next key replaces a wrong PIN, and a missing name keeps the typed PIN.
  function fail(text, field = pin) {
    if (!open) return;
    go.disabled = false;
    err.textContent = sentence(text);
    err.hidden = false;
    field.focus();
    if (field === pin) pin.select();
  }

  function submit() {
    if (go.disabled) return;
    const who = picker ? picker.value().trim() : '';
    if (picker && !who) return fail(name.missing, picker.input);
    err.hidden = true;
    if (!send(pin.value, who)) return fail(LINK_DOWN);
    go.disabled = true;
  }

  if (dropCloses)
    on('conn', () => {
      if (S.wsState !== 'up') close(false);
    });

  return {
    open(text = '') {
      back ||= document.activeElement;
      open = true;
      why.textContent = text;
      why.hidden = !text;
      err.hidden = true;
      picker?.set('');
      m.show();
      (picker ? picker.input : pin).focus();
    },
    close,
    fail,
    shown: () => open,
    err,
  };
}
