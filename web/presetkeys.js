// Page-level keys for presets, in one place: Option/Alt+1-9 fire the first nine
// pinned presets, Ctrl+G (⌘G on a Mac) opens the Presets popup, and on
// Windows/Linux a bare Alt tap is kept from focusing the browser's menu bar.
// The popup's own keys (move, reorder, pin) live in presetpopup.js and the
// chip key reveal (html[data-alt]) in keys.js.

import { S, emit } from './core.js';
import { chord } from './keys.js';
import { fire, presetShown } from './presets.js';
import { pinnedPresets } from './presetstore.js';

const mac = /Mac|iPhone|iPad/.test(navigator.platform);

// A modal, Settings or the lock screen over the page (the Presets popup is a
// modal too): not now.
const blocked = () =>
  document.documentElement.dataset.view === 'settings' ||
  !!document.querySelector('.pb-modal-backdrop:not([hidden]), .sode-lock.is-on, #m-sheet:not([hidden])');

const usable = () => S.cfg && presetShown();

document.addEventListener('keydown', (e) => {
  if (e.repeat || !usable()) return;
  const c = chord(e);
  const slot = /^alt\+([1-9])$/.exec(c);
  if (slot) {
    // Even with no pin in that slot, and even while blocked: Alt+1-9 must not
    // reach the browser (Firefox switches tabs).
    e.preventDefault();
    const s = !blocked() && pinnedPresets()[+slot[1] - 1];
    if (s) fire(s.scope, s.name);
  } else if (c === 'ctrl+g' || (mac && c === 'meta+g')) {
    // Ctrl+G on every OS, ⌘G on a Mac; the browser's find-next is stopped.
    e.preventDefault();
    if (!blocked()) emit('presetPopup');
  }
});

// Windows and Linux: letting go of Alt alone moves focus to the browser's menu
// bar. Alt is Sode's key for presets (Alt+1-9 and the reveal), so it is not
// passed on while presets are showing.
document.addEventListener('keyup', (e) => {
  if (!mac && e.key === 'Alt' && presetShown()) e.preventDefault();
}, true);
