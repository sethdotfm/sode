import './layout.js';
import './keys.js';
import { connect, on } from './core.js';
import { initTheme } from './device.js';
import './desktop.js';
import './settings.js';
import './brand.js';
import './stepinframe.js';
import './lock.js';
import './profile.js';
import './filters.js';
import './searchbox.js';
import { openItemLinks } from './itemlinkopen.js';
import './mobile.js';
import './presetpopup.js';
import './presetkeys.js';
import './panelwatch.js';
import './inboxdialog.js';
import './noadmin.js';
import './setupwizard.js';
import { bigScrollAll } from './bigscroll.js';

on('brand', ({ text }) => {
  document.title = text;
});

initTheme();
openItemLinks();
bigScrollAll();
connect();
