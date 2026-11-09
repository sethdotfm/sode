// Entry for the three panel pages (/panels, /panels/editor, /panels/<id>). Only a panel itself does
// anything yet; the others are placeholders. Sign-in and the page lock come from lock.js, as on /log.
import { $, on, connect } from './core.js';
import './brand.js';
import './lock.js';
import { runPanel } from './panelrun.js';
import { panelIdFrom } from './panelrules.js';

// 4x8 maps one to one onto a hardware key deck (docs/panels-design.md).
export const DEFAULT_GRID = { panelRows: 4, panelCols: 8 };

const id = panelIdFrom(location.pathname);

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-panels__title');
  title.textContent = title.textContent.replace(/^Sode/, text);
  if (!id) document.title = title.textContent;
});

if (id) runPanel(id);
else $('panels-note').textContent = `Panels are not built yet. The default grid will be ${DEFAULT_GRID.panelRows} by ${DEFAULT_GRID.panelCols}.`;

connect();
