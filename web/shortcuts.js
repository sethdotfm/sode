// The shortcut list (? or Ctrl+?) dialog of the desktop layout. Closing from
// the keyboard returns to keyboard mode if it was on when the list opened;
// closing with the mouse never does (a click leaves it).

import { $ } from './core.js';
import { mac } from './keys.js';

// keyMode(): keyboard mode is on now; fromHistory(node): node is in Recent
// (whose rows may be redrawn meanwhile); enterKeys(): go back to keyboard
// mode; focusFirst(): focus the first patch box.
export function shortcutList({ keyMode, fromHistory, enterKeys, focusFirst }) {
  let back = null;
  let wasKeys = false;
  let fromHist = false;
  // Mac labels; a key that only a Mac has (⌃Z beside ⌘Z) is hidden elsewhere.
  if (mac) for (const k of $('keys').querySelectorAll('[data-mac]')) { k.textContent = k.dataset.mac; k.hidden = false; }
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
