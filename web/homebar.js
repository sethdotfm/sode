// The main page's page header: the same bar as /panels and /log (panelsbar.js), over both the Home and the Settings
// view. Its Settings and Home links switch the view in this page, so Settings' unsaved-edits rule still applies.
import { $, emit } from './core.js';
import { panelsBar } from './panelsbar.js';

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

panelsBar($('home-bar'), { panels: true, leave });
