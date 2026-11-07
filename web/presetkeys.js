// Page-level preset keys (Alt+1-9, with Shift fills the boxes; Ctrl/Cmd+G popup; Windows/Linux bare-Alt cancel).
// Popup keys are in presetpopup.js; the chip reveal in keys.js.

import { S, emit } from './core.js';
import { chord, mac } from './keys.js';
import { fire, fillBoxes, presetShown } from './presets.js';
import { pinnedPresets } from './presetstore.js';

// Modal, Settings, lock screen, phone sheet or pick bar (html[data-sode-pick]) over the page.
const blocked = () =>
  document.documentElement.dataset.view === 'settings' ||
  'sodePick' in document.documentElement.dataset ||
  !!document.querySelector('.pb-modal-backdrop:not([hidden]), .sode-lock.is-on, #m-sheet:not([hidden])');

const usable = () => S.cfg && presetShown();

document.addEventListener('keydown', (e) => {
  // The popup handles its own Alt+1-9 (it closes, so blocked() would read false and the preset fire twice).
  if (e.repeat || e.defaultPrevented || !usable()) return;
  const c = chord(e);
  const slot = /^alt\+(shift\+)?([1-9])$/.exec(c);
  if (slot) {
    // Alt+1-9 must not reach the browser (Firefox switches tabs), even with no pin in the slot or while blocked.
    e.preventDefault();
    const s = !blocked() && pinnedPresets()[+slot[2] - 1];
    if (s) (slot[1] ? fillBoxes : fire)(s.scope, s.name);
  } else if (c === 'ctrl+g' || (mac && c === 'meta+g')) {
    e.preventDefault();
    if (!blocked()) emit('presetPopup');
  }
});

// Windows/Linux: a bare Alt release focuses the browser menu bar; swallowed while presets show.
document.addEventListener('keyup', (e) => {
  if (!mac && e.key === 'Alt' && presetShown()) e.preventDefault();
}, true);
