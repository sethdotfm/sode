// Settings' own PIN prompt: shown over the form while this connection needs the Advanced Settings PIN, with its help
// dialog.

import { $, sentence } from './core.js';
import { layout } from './layout.js';
import { unlock, needsPin } from './pin.js';
import { form, isOpen } from './settingsform.js';

// Focus moves only when this flips (or on open), not on every status update.
export function renderLock(opening = false) {
  if (!isOpen()) return;
  const locked = needsPin();
  const flipped = locked !== 'locked' in form.dataset;
  if (locked) form.dataset.locked = '';
  else {
    delete form.dataset.locked;
    $('lock-help').hidden = true;
  }
  if (!opening && !flipped) return;
  if (layout() !== 'desktop') return;
  if (locked) $('lock-pin').focus();
  else $('settings-find').focus({ preventScroll: true });
}

export function showLockErr(text) {
  renderLock();
  $('lock-err').textContent = sentence(text);
  $('lock-err').hidden = false;
  $('lock-pin').select();
}

export async function sendUnlock() {
  $('lock-unlock').disabled = true;
  const r = await unlock($('lock-pin').value);
  if (r.superseded) return;
  $('lock-unlock').disabled = false;
  if (r.stale || !isOpen()) return;
  if (!r.ok) return showLockErr(r.err);
  $('lock-pin').value = '';
  $('lock-err').hidden = true;
  renderLock();
}

export function resetLock() {
  $('lock-pin').value = '';
  $('lock-unlock').disabled = false;
  $('lock-err').hidden = true;
}

function openLockHelp() {
  $('lock-help').hidden = false;
  $('lock-help-dialog').focus();
}
export function closeLockHelp() {
  $('lock-help').hidden = true;
  if (isOpen()) $('lock-pin').focus();
}
$('lock-help-open').addEventListener('click', openLockHelp);
$('lock-help-close').addEventListener('click', closeLockHelp);
