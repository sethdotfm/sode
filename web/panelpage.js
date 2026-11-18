// Entry for the panel pages (/panels, /panels/editor and /panels/<id>). Sign-in and the page lock come from lock.js, as on /log.
import { on, connect, S } from './core.js';
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
import { linkInstall, OTHER_INSTALL } from './itemlink.js';

const id = panelIdFrom(location.pathname);

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-panels__title');
  title.textContent = title.textContent.replace(/^Sode/, text);
  if (!id) document.title = title.textContent;
});

// The run page leaves its scrolling to the Grid's own code.
if (!id) bigScrollAll();

// An Item link names its install, so the panel waits for the hello to say which install this is.
function runLinked() {
  if (!linkInstall(location.hash)) return runPanel(id);
  const off = on('helloData', () => {
    queueMicrotask(off); // a removal inside emit would skip the next listener
    if (linkInstall(location.hash) === S.installUid) return runPanel(id);
    const note = document.getElementById('panels-note');
    note.textContent = OTHER_INSTALL;
    note.hidden = false;
  });
}

if (id) runLinked();
else if (document.getElementById('ed-grid'))
  runEditor().then((editor) => {
    initArtTool(editor, initFind(editor));
    initTemplates(editor);
    initGrip(editor);
    initNameEdit(initSidebar(editor).renameField);
  });
else runPanelsList();

connect();
