// Entry for /panels, /panels/editor and /panels/<id>.
import { on, once, connect, S } from './core.js';
import './brand.js';
import './lock.js';
import './inboxdialog.js';
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
import { bigScroll, bigScrollAll } from './bigscroll.js';
import { linkInstall, OTHER_INSTALL } from './itemlink.js';
import { layout, onLayout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { showApprovalCards } from './approvalcards.js';
import './palette.js';
import { registerShell } from './swreg.js';

const id = panelIdFrom(location.pathname);

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-panels__title');
  title.textContent = title.textContent.replace(/^Sode/, text);
  if (!id) document.title = title.textContent;
});

// The run page leaves the grid's scrolling to the grid's own code.
if (!id) bigScrollAll();
else for (const n of document.querySelectorAll('#confirm-body, #keys .pb-modal__body')) bigScroll(n);

// An Item link names its install, so the panel waits for the hello to say which install this is.
function runLinked() {
  if (!linkInstall(location.hash)) return runPanel(id);
  once('helloData', () => {
    if (linkInstall(location.hash) === S.installUid) return runPanel(id);
    const note = document.getElementById('panels-note');
    note.textContent = OTHER_INSTALL;
    note.hidden = false;
  });
}

// No phone editor (docs/panels-design.md, Phone and touch).
function editorOnPhone() {
  const head = document.querySelector('.sode-ed > .sode-panels__head');
  for (const n of document.querySelectorAll('.sode-ed > *')) if (n !== head) n.hidden = true;
  head.after(desktopOnly('edit panels'));
  onLayout(() => location.reload());
}

if (!id) showApprovalCards();

if (id) runLinked();
else if (document.getElementById('ed-grid') && layout() === 'phone') editorOnPhone();
else if (document.getElementById('ed-grid'))
  runEditor().then((editor) => {
    initArtTool(editor, initFind(editor));
    initTemplates(editor);
    initGrip(editor);
    initNameEdit(initSidebar(editor).renameField);
  });
else runPanelsList();

registerShell();
connect();
