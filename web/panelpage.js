// Entry for the panel pages (/panels, /panels/editor and /panels/<id>). Sign-in and the page lock come from lock.js, as on /log.
import { on, connect } from './core.js';
import './brand.js';
import './lock.js';
import { runPanel } from './panelrun.js';
import { runPanelsList } from './panelslist.js';
import { panelIdFrom } from './panelrules.js';
import { runEditor } from './paneleditor.js';
import { initFind } from './editorfind.js';
import { initArtTool } from './editorarttool.js';
import { initTemplates } from './editortemplates.js';
import { initSidebar } from './editorside.js';
import { initNameEdit } from './editorname.js';
import { initGrip } from './editorgrip.js';
import { bigScrollAll } from './bigscroll.js';

const id = panelIdFrom(location.pathname);

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-panels__title');
  title.textContent = title.textContent.replace(/^Sode/, text);
  if (!id) document.title = title.textContent;
});

// The run page leaves its scrolling to the Grid's own code.
if (!id) bigScrollAll();

if (id) runPanel(id);
else if (document.getElementById('ed-grid'))
  runEditor().then((editor) => {
    initArtTool(editor, initFind(editor));
    initTemplates(editor);
    initGrip(editor);
    initNameEdit(initSidebar(editor).renameField);
  });
else runPanelsList();

connect();
