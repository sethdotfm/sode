// Shortcut list (? or Ctrl+?) dialog. Closing from the keyboard returns to keyboard mode if it was on; closing with the mouse never does.

import { $ } from './core.js';
import { mac } from './keys.js';

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
    $('keys-dialog').focus();
  }
  function close(viaKeys = false) {
    $('keys').hidden = true;
    if (wasKeys && viaKeys) enterKeys();
    else if (back?.isConnected && !fromHist) back.focus();
    else focusFirst();
  }
  $('keys-close').addEventListener('click', () => close());
  return { open, close, shown: () => !$('keys').hidden };
}
