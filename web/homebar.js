// The main page's page header: the same bar as /panels and /log (panelsbar.js), over both the Home and the Settings
// view. Home needs no Home link, so the Grid popup takes its place. Settings switches the view in this page, so
// Settings' unsaved-edits rule still applies. The ids grid-open and settings-open are what other code and the browser
// checks press.
import { $, el, icon, emit, on } from './core.js';
import { keyLabel } from './keys.js';
import { panelsBar, barTitle } from './panelsbar.js';
import { showGrid } from './gridpopup.js';

function iconButton(cls, id, name, label) {
  const n = el('button', `pb-btn pb-btn--ghost pb-btn--icon ${cls}`, icon(name, false));
  n.type = 'button';
  n.id = id;
  n.setAttribute('aria-label', label);
  n.dataset.pbTip = label;
  return n;
}

function gridButton() {
  const n = iconButton('sode-bar__grid', 'grid-open', 'sode-grid', 'Grid popup');
  n.setAttribute('aria-keyshortcuts', 'Control+E');
  n.dataset.pbTip = `Grid popup (${keyLabel('ctrl+e')})`;
  // The patch boxes keep focus, as for Take.
  n.addEventListener('mousedown', (e) => e.preventDefault());
  n.addEventListener('click', () => showGrid());
  return n;
}

// Opening Settings again would refill the form and lose unsaved edits.
function settingsButton() {
  const n = iconButton('sode-bar__settings', 'settings-open', 'settings', 'Settings');
  n.addEventListener('click', () => {
    if (document.documentElement.dataset.view !== 'settings') emit('settingsOpen');
  });
  return n;
}

panelsBar($('home-bar'), { panels: true, first: gridButton(), settings: settingsButton(), conn: $('conn') });

on('brand', ({ text }) => barTitle($('home-bar').querySelector('.sode-panels__title'), text));
