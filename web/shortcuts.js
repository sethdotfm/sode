// Shortcut list (? or Ctrl+?) dialog. A close from the keyboard returns to keyboard mode if it was on; a close with the mouse never does.

import { $, el, on } from './core.js';
import { mac, keyLabel } from './keys.js';

// The open list's close, which a Lock runs.
let shut = null;
// The page made a key list, so the Palette's Keyboard shortcuts row opens it here (palettecatalogue.js keyList).
let made = false;
export const hasKeyList = () => made;
on('locking', () => shut?.());

export function shortcutList({ keyMode, fromHistory, enterKeys, focusFirst }) {
  made = true;
  let back = null;
  let wasKeys = false;
  let fromHist = false;
  // The HTML holds the label for other platforms. A mod key takes Control too on a Mac (⌃z beside ⌘z).
  if (mac)
    for (const k of $('keys').querySelectorAll('kbd[data-key]')) {
      const spec = k.dataset.key;
      k.textContent = keyLabel(spec);
      if (/^mod(\+|$)/.test(spec)) k.after(' ', el('kbd', null, keyLabel(spec.replace('mod', 'ctrl'))));
    }
  function open() {
    back = document.activeElement;
    wasKeys = keyMode();
    fromHist = fromHistory(back);
    $('keys').hidden = false;
    shut = close;
    $('keys-dialog').focus();
  }
  function close(viaKeys = false) {
    shut = null;
    $('keys').hidden = true;
    if (wasKeys && viaKeys) enterKeys();
    else if (back?.isConnected && !fromHist) back.focus();
    else focusFirst();
  }
  $('keys-close').addEventListener('click', () => close());
  // The Palette's Keyboard shortcuts row.
  on('openShortcuts', () => $('keys').hidden && open());
  return { open, close, shown: () => !$('keys').hidden };
}
