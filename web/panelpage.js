// Entry for the panel pages (/panels, /panels/editor and /panels/<id>). Sign-in and the page lock come from lock.js, as on /log.
import { on, connect } from './core.js';
import './brand.js';
import './lock.js';
import { runPanel } from './panelrun.js';
import { runPanelsList } from './panelslist.js';
import { panelIdFrom } from './panelrules.js';
import { runEditor } from './paneleditor.js';
import { initFind } from './editorfind.js';
import { initTemplates } from './editortemplates.js';
import { initGrip } from './editorgrip.js';

// 4x8 maps one to one onto a hardware key deck (docs/panels-design.md).
export const DEFAULT_GRID = { panelRows: 4, panelCols: 8 };

const id = panelIdFrom(location.pathname);

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-panels__title');
  title.textContent = title.textContent.replace(/^Sode/, text);
  if (!id) document.title = title.textContent;
});

if (id) runPanel(id);
else if (document.getElementById('ed-grid'))
  runEditor().then((editor) => {
    initFind(editor);
    initTemplates(editor);
    initGrip(editor);
  });
else runPanelsList();

connect();
