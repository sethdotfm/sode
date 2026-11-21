import { icon } from './core.js';
import { kioskNow, headHidden, leavesKiosk, fullscreenWords, kioskWords } from './panelchromerules.js';
import { enterFullscreen, exitFullscreen } from './panelfeel.js';

// Remembered per browser.
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

// Follows the browser's own state: Esc leaves without a click here. Returns its stop.
export function initScreen(button) {
  if (!document.fullscreenEnabled) {
    button.hidden = true; // iOS Safari and embedded frames refuse it
    return () => {};
  }
  const sync = () => {
    const on = !!document.fullscreenElement;
    button.setAttribute('aria-pressed', String(on));
    setIcon(button, fullscreenWords(on));
  };
  sync();
  document.addEventListener('fullscreenchange', sync);
  button.addEventListener('click', () => (document.fullscreenElement ? exitFullscreen(document) : enterFullscreen(document)));
  return () => document.removeEventListener('fullscreenchange', sync);
}

// Kiosk for this open page only. The panel setting and #kiosk=1 give the starting state, and a reload goes back to them.
// apply() alone hides the header, the Panels link and the Lock bar.
export function initChrome(ui) {
  initWidth(ui.full, ui.main);
  initScreen(ui.screen);
  let override = null;
  let base = false;
  let titleBar = true;
  let grid = false;
  const apply = () => {
    const kiosk = kioskNow(override, base);
    ui.head.hidden = headHidden(kiosk, titleBar);
    ui.home.hidden = kiosk;
    // A Grid panel reaches the bottom of the window, so its Panels link goes in the header, or in ui.tools (gridpanelheadview.js).
    // ui.toolsShown is set where the group can show with the header hidden.
    const box = ui.tools || ui.head;
    const inHead = grid && (ui.toolsShown ? ui.toolsShown(kiosk) : !ui.head.hidden);
    if (inHead !== box.contains(ui.home)) (inHead ? box : ui.main).append(ui.home);
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
    // Called on each read of the panel's look, so a live update keeps a pressed Kiosk button's choice.
    show(baseKiosk, titleBarOn, gridPanel = false) {
      base = baseKiosk;
      titleBar = titleBarOn;
      grid = gridPanel;
      return apply();
    },
  };
}
