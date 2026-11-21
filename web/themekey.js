// The theme key, Alt+Shift+t, from any focus on every page. Browsers keep Ctrl+Shift+T for reopening tabs.
import { chord } from './keys.js';
import { flipTheme } from './device.js';

export const THEME_CHORD = 'alt+shift+t';

// Capture, as the Home key. preventDefault keeps macOS Option+Shift+T from typing a character, also where the key
// does nothing. That is a Kiosk panel, and a panel that stores its own theme (panelrun.js lookApplier).
document.addEventListener(
  'keydown',
  (e) => {
    if (chord(e) !== THEME_CHORD) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    const root = document.documentElement;
    if (e.repeat || root.hasAttribute('data-sode-kiosk') || root.hasAttribute('data-sode-panel-theme')) return;
    flipTheme();
  },
  true,
);
