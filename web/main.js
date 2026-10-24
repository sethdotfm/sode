// Entry point: views subscribe to core, then the connection starts.
import './layout.js';
import { connect, initTheme, initClock } from './core.js';
import './desktop.js';
import './settings.js';
import { applySharedLink } from './filters.js';
import './mobile.js';

initTheme();
initClock();
applySharedLink();
connect();
