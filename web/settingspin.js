// The Advanced Settings PIN fields in Settings (Access): behind a "Change
// Advanced PIN" button; a Save error or a Settings search hit opens them.

import { $, S } from './core.js';

export function renderPinStatus() {
  $('cfg-pin-clear-row').hidden = !S.pinSet;
  $('cfg-pin-status').textContent = S.pinSet
    ? 'A PIN is set. Leave both fields blank to keep it.'
    : 'No PIN is set. Use 4–64 characters.';
  $('cfg-pin-change').textContent = S.pinSet ? 'Change Advanced PIN' : 'Set Advanced PIN';
}

export function showPinFields(open) {
  $('cfg-pin-box').hidden = !open;
  $('cfg-pin-change').setAttribute('aria-expanded', String(open));
}

// Back to the closed, empty state (the form opened, or a save went through).
export function resetPinFields() {
  $('cfg-pin').value = '';
  $('cfg-pin2').value = '';
  $('cfg-pin-clear').checked = false;
  showPinFields(false);
  renderPinStatus();
}

$('cfg-pin-change').addEventListener('click', () => {
  showPinFields($('cfg-pin-box').hidden);
  if (!$('cfg-pin-box').hidden) $('cfg-pin').focus();
});
