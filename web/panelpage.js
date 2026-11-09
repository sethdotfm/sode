// Entry for the three panel pages (/panels, /panels/editor, /panels/<id>): a placeholder until
// the panels land. Sign-in and the page lock come from lock.js, as on /log.
import { $, on, connect } from './core.js';
import './brand.js';
import './lock.js';

// 4x8 maps one to one onto a hardware key deck (docs/panels-design.md).
export const DEFAULT_GRID = { panelRows: 4, panelCols: 8 };

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-panels__title');
  title.textContent = title.textContent.replace(/^Sode/, text);
  document.title = title.textContent;
});

$('panels-note').textContent = `Panels are not built yet. The default grid will be ${DEFAULT_GRID.panelRows} by ${DEFAULT_GRID.panelCols}.`;

connect();
