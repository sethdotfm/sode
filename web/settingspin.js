// Advanced PIN fields in Settings > Access, behind "Change Advanced PIN"; a Save error or search hit opens them.

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
