// Page-level keys of the strip: Alt+1-9 open the first nine bookmarks, and with Shift a preset bookmark fills the
// boxes. Ctrl/Cmd+G opens the Presets popup. Windows/Linux bare-Alt cancel. The chip reveal is in keys.js.

import { S, emit } from './core.js';
import { chord, mac } from './keys.js';
import { presetShown } from './presets.js';
import { openSlot } from './bookmarks.js';

// Modal, Settings, setup wizard, lock screen, phone sheet or pick bar (html[data-sode-pick]) over the page.
const blocked = () =>
  document.documentElement.dataset.view === 'settings' ||
  document.documentElement.hasAttribute('data-sode-setup') ||
  'sodePick' in document.documentElement.dataset ||
  !!document.querySelector('.pb-modal-backdrop:not([hidden]), .sode-lock.is-on, #m-sheet:not([hidden])');

const usable = () => S.cfg && presetShown();

document.addEventListener('keydown', (e) => {
  if (e.repeat || e.defaultPrevented || !usable()) return;
  const c = chord(e);
  const slot = /^alt\+(shift\+)?([1-9])$/.exec(c);
  if (slot) {
    // Alt+1-9 must not reach the browser (Firefox switches tabs), even with no bookmark in the slot or while blocked.
    e.preventDefault();
    if (!blocked()) openSlot(+slot[2] - 1, !!slot[1]);
  } else if (c === 'ctrl+g' || (mac && c === 'meta+g')) {
    e.preventDefault();
    if (!blocked()) emit('presetPopup');
  }
});

// Windows/Linux: a bare Alt release focuses the browser menu bar; swallowed while the strip shows.
document.addEventListener(
  'keyup',
  (e) => {
    if (!mac && e.key === 'Alt' && presetShown()) e.preventDefault();
  },
  true,
);
