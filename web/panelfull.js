import { icon } from './core.js';
import { kioskNow, headHidden, leavesKiosk, fullscreenWords, kioskWords } from './panelchromerules.js';
import { enterFullscreen, exitFullscreen } from './panelfeel.js';

// Remembered per browser. No value is Full width.
const KEY = 'sode-panel-full';

function setIcon(button, { icon: name, label }) {
  button.replaceChildren(icon(name));
  button.setAttribute('aria-label', label);
  button.dataset.pbTip = label;
}

// paint is apply() in initChrome, which forces Full width where the button cannot show. stored() is the choice.
function initWidth(button, paint) {
  const set = (on) => {
    button.setAttribute('aria-pressed', String(on));
    setIcon(button, { icon: on ? 'chevrons-right-left' : 'chevrons-left-right', label: on ? 'Normal width' : 'Full width' });
    paint();
  };
  let on = true;
  try {
    on = localStorage.getItem(KEY) !== '0';
  } catch {
    // Blocked storage only costs the memory of the choice.
  }
  button.addEventListener('click', () => {
    on = !on;
    set(on);
    try {
      localStorage.setItem(KEY, on ? '1' : '0');
    } catch {
      // See above.
    }
  });
  return { stored: () => on, set: () => set(on) };
}

// A Buttons panel's grid is never higher than the window (panels.css). --sode-panel-chrome is the page height that is
// not the grid: all above it plus the page's bottom padding. Nothing above the grid takes its size from the grid, so
// the measure does not loop. Returns the measure, for a change that moves the grid without a resize.
function fitHeight(main, grid, foot) {
  if (!grid) return () => {};
  const measure = () => {
    // A Grid panel's height does not come from this property, and on main it would restyle every crosspoint.
    if (grid.classList.contains('sode-gridpanel')) return;
    const mainStyle = getComputedStyle(main);
    const top = grid.getBoundingClientRect().top - document.documentElement.getBoundingClientRect().top;
    const pad = parseFloat(mainStyle.paddingBottom) || 0;
    // The status line and banner box is out of the page while empty, and in it when a line shows. Reserve one line for
    // it, so a refusal does not resize the cells. A kiosk fixes the box, so it never moves the grid.
    let room = 0;
    if (foot && !document.documentElement.hasAttribute('data-sode-kiosk') && getComputedStyle(foot).display === 'none') {
      const status = foot.querySelector('.sode-panel__status');
      const line = status ? parseFloat(getComputedStyle(status).lineHeight) : 0;
      room = (line || 0) + (parseFloat(mainStyle.rowGap) || 0);
    }
    main.style.setProperty('--sode-panel-chrome', `${Math.ceil(top + pad + room)}px`);
  };
  const seen = new ResizeObserver(measure);
  for (const n of main.children) if (n !== grid) seen.observe(n);
  addEventListener('resize', measure);
  return measure;
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
  const width = initWidth(ui.full, () => apply());
  const fit = fitHeight(ui.main, ui.grid, ui.foot);
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
    // Without the header there is no Full width button, so the page is full width.
    ui.main.classList.toggle('sode-panels--full', width.stored() || ui.head.hidden);
    fit();
    return kiosk;
  };
  width.set();
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
