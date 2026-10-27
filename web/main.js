// Entry point: views subscribe to core, then the connection starts.
import './layout.js';
import { connect } from './core.js';
import { initTheme } from './device.js';
import './desktop.js';
import './settings.js';
import { applySharedLink } from './filters.js';
import './mobile.js';

initTheme();
applySharedLink();
connect();
