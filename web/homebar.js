// The main page's page header: the same bar as /panels and /log (panelsbar.js), over both the Home and the Settings
// view. Home needs no Home link, so the Grid popup takes its place. Settings and Home links switch the view in this
// page, so Settings' unsaved-edits rule still applies.
import { $, el, icon, emit, on } from './core.js';
import { keyLabel } from './keys.js';
import { panelsBar, barTitle } from './panelsbar.js';
import { showGrid } from './gridpopup.js';

function leave(href) {
  if (href === './#settings') {
    if (document.documentElement.dataset.view !== 'settings') $('settings-open').click();
    return false;
  }
  if (href === './') {
    emit('homeKey');
    return false;
  }
  return true;
}

function gridButton() {
  const n = el('button', 'pb-btn pb-btn--ghost pb-btn--icon sode-bar__grid', icon('grid-3x3', false));
  n.type = 'button';
  n.setAttribute('aria-label', 'Grid popup');
  n.setAttribute('aria-keyshortcuts', 'Control+E');
  n.dataset.pbTip = `Grid popup (${keyLabel('ctrl+e')})`;
  // The patch boxes keep focus, as for the footer's Grid button (gridpopup.js).
  n.addEventListener('mousedown', (e) => e.preventDefault());
  n.addEventListener('click', () => showGrid());
  return n;
}

panelsBar($('home-bar'), { panels: true, leave, home: gridButton() });

on('brand', ({ text }) => barTitle($('home-bar').querySelector('.sode-panels__title'), text));
