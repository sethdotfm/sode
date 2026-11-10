import { icon } from './core.js';
import { kioskNow, headHidden, leavesKiosk, fullscreenWords, kioskWords } from './panelchromerules.js';
import { enterFullscreen, exitFullscreen } from './panelfeel.js';

// Full width: the grid drops the page's width cap and padding. Remembered per browser (`sode-panel-full`).
const KEY = 'sode-panel-full';

function setIcon(button, { icon: name, label }) {
  button.replaceChildren(icon(name));
  button.setAttribute('aria-label', label);
  button.dataset.pbTip = label;
}

function initWidth(button, main) {
  const set = (on) => {
    main.classList.toggle('sode-panels--full', on);
    button.setAttribute('aria-pressed', String(on));
    setIcon(button, { icon: on ? 'chevrons-right-left' : 'chevrons-left-right', label: on ? 'Normal width' : 'Full width' });
  };
  let on = false;
  try {
    on = localStorage.getItem(KEY) === '1';
  } catch {
    // Blocked storage only costs the memory of the choice.
  }
  set(on);
  button.addEventListener('click', () => {
    on = !on;
    set(on);
    try {
      localStorage.setItem(KEY, on ? '1' : '0');
    } catch {
      // See above.
    }
  });
}

// Browser full screen, a toggle that follows the browser's own state (Esc leaves it without a click here).
function initScreen(button) {
  if (!document.fullscreenEnabled) {
    button.hidden = true; // iOS Safari and embedded frames refuse it
    return;
  }
  const sync = () => {
    const on = !!document.fullscreenElement;
    button.setAttribute('aria-pressed', String(on));
    setIcon(button, fullscreenWords(on));
  };
  sync();
  document.addEventListener('fullscreenchange', sync);
  button.addEventListener('click', () => (document.fullscreenElement ? exitFullscreen(document) : enterFullscreen(document)));
}

// Kiosk for this open page only: the header button turns it on, and with the header gone Esc turns it off. The panel setting and #kiosk=1 only give the starting state; a reload
// goes back to them. apply() is the one place that hides the header, the Panels link and the Lock bar.
export function initChrome(ui) {
  initWidth(ui.full, ui.main);
  initScreen(ui.screen);
  let override = null;
  let base = false;
  let titleBar = true;
  const apply = () => {
    const kiosk = kioskNow(override, base);
    ui.head.hidden = headHidden(kiosk, titleBar);
    ui.home.hidden = kiosk;
    document.documentElement.toggleAttribute('data-sode-kiosk', kiosk); // hides the Lock bar too (panels.css)
    setIcon(ui.kiosk, kioskWords(kiosk));
    ui.kiosk.setAttribute('aria-pressed', String(kiosk));
    return kiosk;
  };
  const toggle = () => {
    override = !kioskNow(override, base);
    apply();
  };
  ui.kiosk.addEventListener('click', toggle);
  document.addEventListener('keydown', (e) => {
    if (leavesKiosk(e, kioskNow(override, base))) toggle();
  });
  return {
    // runPanel calls this each time it reads the panel's look, so a live update keeps a pressed Kiosk button's choice.
    show(baseKiosk, titleBarOn) {
      base = baseKiosk;
      titleBar = titleBarOn;
      return apply();
    },
  };
}
