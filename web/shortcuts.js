// Shortcut list (? or Ctrl+?) dialog. A close from the keyboard returns to keyboard mode if it was on; a close with the mouse never does.

import { $, on } from './core.js';
import { mac } from './keys.js';

// The open list's close, which a Lock runs.
let shut = null;
on('locking', () => shut?.());

export function shortcutList({ keyMode, fromHistory, enterKeys, focusFirst }) {
  let back = null;
  let wasKeys = false;
  let fromHist = false;
  // A Mac-only key (⌃Z beside ⌘Z) is hidden elsewhere.
  if (mac)
    for (const k of $('keys').querySelectorAll('[data-mac]')) {
      k.textContent = k.dataset.mac;
      k.hidden = false;
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
  return { open, close, shown: () => !$('keys').hidden };
}
