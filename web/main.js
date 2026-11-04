// Entry point: views subscribe to core, then the connection starts.
import './layout.js';
import { connect, on } from './core.js';
import { initTheme } from './device.js';
import './desktop.js';
import './settings.js';
import './brand.js';
import './lock.js';
import './profile.js';
import './filters.js';
import { applySharedLink } from './transfer.js';
import './mobile.js';
import './presetpopup.js';

on('brand', ({ text }) => { document.title = text; });

initTheme();
applySharedLink();
connect();
