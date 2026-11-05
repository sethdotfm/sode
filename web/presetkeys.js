// Page-level keys for presets, in one place: Option/Alt+1-9 fire the first nine
// pinned presets (with Shift they fill the patch boxes instead), Ctrl+G (⌘G on a Mac) opens the Presets popup, and on
// Windows/Linux a bare Alt tap is kept from focusing the browser's menu bar.
// The popup's own keys (move, reorder, pin) live in presetpopup.js and the
// chip key reveal (html[data-alt]) in keys.js.

import { S, emit } from './core.js';
import { chord, mac } from './keys.js';
import { fire, fillBoxes, presetShown } from './presets.js';
import { pinnedPresets } from './presetstore.js';

// A modal (the desktop confirm and the Presets popup are modals), Settings,
// the lock screen or the phone sheet over the page, or a pick bar waiting for
// its Add (html[data-sode-pick]): not now.
const blocked = () =>
  document.documentElement.dataset.view === 'settings' ||
  'sodePick' in document.documentElement.dataset ||
  !!document.querySelector('.pb-modal-backdrop:not([hidden]), .sode-lock.is-on, #m-sheet:not([hidden])');

const usable = () => S.cfg && presetShown();

document.addEventListener('keydown', (e) => {
  // The Presets popup handles its own Alt+1-9 (it closes, so blocked() would
  // read false here and the preset would fire twice).
  if (e.repeat || e.defaultPrevented || !usable()) return;
  const c = chord(e);
  const slot = /^alt\+(shift\+)?([1-9])$/.exec(c);
  if (slot) {
    // Even with no pin in that slot, and even while blocked: Alt+1-9 must not
    // reach the browser (Firefox switches tabs). With Shift the preset only
    // fills the patch boxes, like Shift+click.
    e.preventDefault();
    const s = !blocked() && pinnedPresets()[+slot[2] - 1];
    if (s) (slot[1] ? fillBoxes : fire)(s.scope, s.name);
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
