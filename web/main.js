// Entry point: views subscribe to core, then the connection starts.
import './layout.js';
import { connect, initTheme } from './core.js';
import './desktop.js';
import './settings.js';
import './mobile.js';

initTheme();
connect();
