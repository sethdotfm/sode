// The Step in and Step out controls in Settings > My user, the Step in popup and the "This steps you out" question on
// links that leave the page. stepin.js holds the state.
import { $, el, S, on, sentence, LINK_DOWN } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { namePicker } from './namepick.js';
import { layout } from './layout.js';
import { sendStepIn, stepOut, steppedIn } from './stepin.js';
import { stepInOffered, stepsOut } from './stepinrules.js';

let ui = null;
let seq = 0;
let waiting = 0;

function modal(id, title, body, buttons, onEsc) {
  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal sode__filters sode__pinprompt');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', `${id}-title`);
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', title), { id: `${id}-title` })),
    ...(body.length ? [el('div', 'pb-modal__body', ...body)] : []),
    el('div', 'pb-modal__footer', ...buttons),
  );
  root.append(dialog);
  document.body.append(root);
  return trapDialog(root, dialog, onEsc);
}

function button(id, label, cls, fn) {
  const b = el('button', `pb-btn ${cls}`, label);
  b.type = 'button';
  b.id = id;
  b.addEventListener('click', fn);
  return b;
}

function close() {
  if (!ui) return;
  ui.dlg.hide();
  ui.pin.value = '';
  ui.err.hidden = true;
  ui.go.disabled = false;
  waiting = 0;
}

function fail(text) {
  ui.err.textContent = sentence(text);
  ui.err.hidden = false;
}

function submit() {
  if (ui.go.disabled) return;
  const name = ui.picker.value().trim();
  if (!name) return fail('Enter your name.');
  ui.err.hidden = true;
  const id = ++seq;
  if (!sendStepIn(name, ui.pin.value, id)) return fail(LINK_DOWN);
  waiting = id;
  ui.go.disabled = true;
}

on('stepInResult', (m) => {
  if (!ui || m.id !== waiting) return;
  waiting = 0;
  ui.go.disabled = false;
  if (m.ok) close();
  else {
    ui.pin.value = '';
    fail(m.err || 'Wrong PIN.');
  }
});

function build() {
  if (ui) return;
  const picker = namePicker({ id: 'stepin-name', users: () => S.users || [], onInput: () => {}, onPick: () => ui.pin.focus() });
  picker.input.placeholder = 'Your name';
  picker.input.setAttribute('aria-label', 'Your name');
  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.inputMode = 'numeric';
  pin.id = 'stepin-pin';
  pin.autocomplete = 'off';
  pin.placeholder = 'Your PIN';
  pin.setAttribute('aria-label', 'Your PIN');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const go = button('stepin-go', 'Step in', '', submit);
  const hint = el('p', 'sode__hint', 'This tab only. Not saved in this browser.');
  const dlg = modal('stepin', 'Step in', [hint, picker.root, pin, err], [button('stepin-cancel', 'Cancel', 'pb-btn--outline', close), go], close);
  picker.input.addEventListener('keydown', (e) => {
    if (picker.key(e)) e.preventDefault();
  });
  pin.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    submit();
  });
  ui = { dlg, picker, pin, err, go };
}

function open() {
  build();
  ui.picker.set('');
  ui.err.hidden = true;
  ui.dlg.show();
  ui.picker.input.focus();
}

let leave = null;
let leaveTo = '';

function askLeave(href) {
  if (!leave) {
    const go = () => {
      leave.hide();
      location.href = leaveTo;
    };
    leave = modal(
      'stepout-ask',
      'Step out?',
      [],
      [button('stepout-ask-cancel', 'Cancel', 'pb-btn--outline', () => leave.hide()), button('stepout-ask-go', 'Step out', '', go)],
      () => leave.hide(),
    );
  }
  leaveTo = href;
  leave.show();
}

export function initStepIn() {
  const note = $('account-step-note');
  on('access', (L) => {
    const stepped = steppedIn();
    $('account-stepin').hidden = !stepInOffered({ locked: L.locked, phone: layout() === 'phone', stepped, user: L.user, users: S.users });
    $('account-stepout').hidden = !stepped;
    if (stepped) {
      $('account-name').replaceChildren('Stepped in as ', el('strong', null, L.user));
      for (const id of ['account-login', 'account-logout']) $(id).hidden = true;
    }
    if (stepped) note.hidden = true;
    if (!stepped) close();
  });
  on('stepEnded', (m) => {
    note.textContent = m.note || '';
    note.hidden = !m.note;
  });
  $('account-stepin').addEventListener('click', open);
  $('account-stepout').addEventListener('click', () => stepOut());
  document.addEventListener(
    'click',
    (e) => {
      const a = e.target.closest?.('a[href]');
      if (!a || !steppedIn() || e.defaultPrevented || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
      if (!stepsOut(a.href, location.href, a.target)) return;
      e.preventDefault();
      askLeave(a.href);
    },
    true,
  );
}
