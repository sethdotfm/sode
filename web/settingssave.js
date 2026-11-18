// The Settings save path: unsaved changes against the last opened or saved form, the form's Save (`config`), and its
// answer. data-apart sections save on their own and are not counted.

import { $, el, S, on, wsSend, sentence } from './core.js';
import { fmtTime } from './device.js';
import { relock, unlocked, trusted, needsPin } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { showPinFields, resetPinFields } from './settingspin.js';
import { form, isOpen, readSettings, formProblem } from './settingsform.js';
import { renderLock, showLockErr, sendUnlock } from './settingslock.js';

// A save is in flight: its own hello / locked arrive before configResult closes the view, so
// do not flip to the PIN prompt for them.
export const saving = () => $('settings-save').disabled;

let clean = '';
let savedAt = null;
const formState = () =>
  JSON.stringify(
    [...form.querySelectorAll('input, select, textarea')]
      .filter((f) => !f.closest('[data-apart]') && f.id !== 'settings-find')
      .map((f) => (f.type === 'checkbox' || f.type === 'radio' ? f.checked : f.value)),
  );
export const dirty = () => isOpen() && formState() !== clean;
const warned = () => !$('settings-unsaved').hidden;

export function markClean(saved) {
  clean = formState();
  savedAt = saved ? new Date().toISOString() : null;
  unwarn();
}

// True when it warned: the first try to leave with unsaved changes warns, a second one goes.
export function warnUnsaved() {
  if (!dirty() || warned()) return false;
  $('settings-unsaved').hidden = false;
  $('settings-exit').textContent = 'Discard and exit';
  $('settings-exit').classList.replace('pb-btn--outline', 'pb-btn--danger');
  renderSaveState();
  return true;
}

export function unwarn() {
  $('settings-unsaved').hidden = true;
  $('settings-exit').textContent = 'Exit';
  $('settings-exit').classList.replace('pb-btn--danger', 'pb-btn--outline');
  renderSaveState();
}

function renderSaveState() {
  const path = S.configPath ? el('code', null, S.configPath) : null;
  $('settings-path').replaceChildren(
    ...(dirty()
      ? warned()
        ? []
        : ['Unsaved changes']
      : !path
        ? [savedAt ? `Applied ${fmtTime(savedAt)}; not saved: ` : 'Not saved: ', el('code', null, 'SODE_CONFIG'), ' unset']
        : savedAt
          ? [`Saved ${fmtTime(savedAt)} to `, path]
          : ['Saved to ', path]),
  );
}

export function showSaveErr(text) {
  $('settings-err').textContent = text;
  $('settings-err').hidden = false;
}

function configResult(m) {
  $('settings-save').disabled = false;
  if (m.ok) {
    if (!isOpen()) return;
    // Stay open. A changed PIN has relocked everyone, this view included.
    resetPinFields();
    $('settings-err').hidden = true;
    markClean(true);
    renderLock();
    return;
  }
  if (m.locked) {
    relock();
    // Not trusted: Settings flips to its own PIN prompt. A trusted user's form never did, so
    // ask in the popup and save again.
    if (needsPin()) return showLockErr(m.err);
    reask('Saving that needs the Advanced Settings PIN.').then((ok) => {
      if (ok) form.requestSubmit();
    });
    return;
  }
  showSaveErr(sentence(m.err));
}

const askThenSave = (why) =>
  askPin(why).then((ok) => {
    if (ok) form.requestSubmit();
  });

// focusField and reveal come from the table of contents (settingstoc.js), which opens a field's page on a phone.
export function initSave({ focusField, reveal }) {
  for (const ev of ['input', 'change']) {
    form.addEventListener(ev, () => {
      $('settings-err').hidden = true;
      return warned() ? unwarn() : renderSaveState();
    });
  }
  window.addEventListener('beforeunload', (e) => {
    if (dirty()) e.preventDefault();
  });
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    if ('locked' in form.dataset) return void sendUnlock();
    // A trusted user saves without the PIN, but not a change to the PIN itself: ask now instead
    // of after a refusal.
    if (S.pinSet && trusted() && !unlocked() && ($('cfg-pin').value || $('cfg-pin-clear').checked)) {
      return void askThenSave('Changing the PIN needs the Advanced Settings PIN.');
    }
    const c = readSettings();
    const bad = formProblem(c);
    if (bad) {
      showSaveErr(bad.text);
      focusField(bad.node);
      return;
    }
    const pin = $('cfg-pin').value;
    if (pin && [...pin].length < 4) {
      showSaveErr('The Advanced Settings PIN must be at least 4 characters.');
      showPinFields(true);
      focusField($('cfg-pin'));
      return;
    }
    if (pin !== $('cfg-pin2').value) {
      showSaveErr('The two PINs do not match.');
      showPinFields(true);
      reveal($('cfg-pin2'));
      $('cfg-pin2').select();
      return;
    }
    if (!wsSend({ t: 'config', config: c, pin, clearPin: $('cfg-pin-clear').checked })) {
      showSaveErr('Not connected to Sode.');
      return;
    }
    $('settings-save').disabled = true;
  });
  on('configResult', configResult);
}
