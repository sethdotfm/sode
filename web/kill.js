// Settings > Access > Kill: the button and its dialog. The server re-checks everything (kill.go).

import { $, el, S, on, onMsg, wsSend, sentence } from './core.js';
import { unlocked } from './pin.js';
import { trapDialog } from './dialogkeys.js';
import { killShown } from './settingsrules.js';

// Keep in step with docs/reference.md (Kill) and GLOSSARY.md.
const WARNINGS = [
  'The router link drops.',
  'Takes not yet sent are lost.',
  'Takes already sent may still land on the router.',
  'Routes the router already accepted stay.',
  'Every browser shows the Lost link banner.',
  "Docker starts Sode again unless the container's restart policy is no or on-failure.",
];

let ui = null;
let seq = 0;
let pending = 0; // id of the kill in flight; 0 = none

function render() {
  $('kill-row').hidden = !killShown(S.cfg, S.pinSet, unlocked());
  if ($('kill-row').hidden) close();
}

function close() {
  if (!ui || ui.root.hidden) return;
  ui.dlg.hide();
  ui.input.value = '';
  ui.err.hidden = true;
  ui.go.disabled = false;
  pending = 0;
}

function showErr(text) {
  ui.err.textContent = text;
  ui.err.hidden = false;
  ui.go.disabled = false;
  ui.input.select();
}

// The PIN rides the message every time, even on an unlocked connection: a Kill is never one click.
function submit() {
  if (ui.go.disabled) return;
  if (!ui.input.value) return showErr('Enter the Advanced Settings PIN.');
  ui.err.hidden = true;
  ui.go.disabled = true;
  pending = ++seq;
  if (!wsSend({ t: 'kill', pin: ui.input.value, id: pending })) {
    pending = 0;
    showErr('Not connected to Sode.');
  }
}

function build() {
  if (ui) return;
  const input = el('input', 'pb-input');
  input.type = 'password';
  input.id = 'kill-pin';
  input.autocomplete = 'current-password';
  input.spellcheck = false;
  input.setAttribute('aria-label', 'Advanced Settings PIN');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const cancel = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancel.type = 'button';
  cancel.id = 'kill-cancel';
  cancel.addEventListener('click', close);
  const go = el('button', 'pb-btn pb-btn--danger', 'Kill Sode');
  go.type = 'button';
  go.id = 'kill-go';
  go.addEventListener('click', submit);
  input.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    submit();
  });

  const list = el('ul');
  list.id = 'kill-warnings';
  list.append(...WARNINGS.map((w) => el('li', null, w)));
  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal sode__filters');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'kill-title');
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header pb-modal__header--error', Object.assign(el('span', 'pb-modal__title', 'Kill Sode'), { id: 'kill-title' })),
    el('div', 'pb-modal__body', el('p', null, 'Stops Sode at once, for emergencies only.'), list, input, err),
    el('div', 'pb-modal__footer', cancel, go),
  );
  root.append(dialog);
  document.body.append(root);
  ui = { root, input, err, go, dlg: trapDialog(root, dialog, close) };
}

$('kill-open').addEventListener('click', () => {
  build();
  ui.dlg.show();
  ui.input.focus();
});

// Only a refusal answers; a Kill that worked ends the socket and the Lost link banner says so.
onMsg('killResult', (m) => {
  if (!ui || m.id !== pending) return;
  pending = 0;
  showErr(sentence(m.err) || 'Refused.');
});

on('pinUnlocked', render);
on('hello', render);
on('locking', close);
on('conn', () => {
  if (S.wsState !== 'up') close();
});
