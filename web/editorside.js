// The panel editor's sidebar: Cell, Panel and Access tabs beside the grid. It opens on Panel and switches to Cell
// when the selection moves to a new square. initSidebar(editor) takes what runEditor returns.
import { $, S, on } from './core.js';
import { makeTabs } from './tabs.js';
import { rectOf } from './gridmodel.js';
import { createCellTab } from './editorcell.js';
import { createPanelTab } from './editorpanel.js';
import { createAccessTab } from './editoraccess.js';

export function initSidebar(ed) {
  const host = $('ed-side');

  const ctx = { offset: () => ed.names().off || 0, levelCount: () => S.levels.length };
  const cell = createCellTab(ed);
  const panel = createPanelTab(ed);
  const access = createAccessTab(ed, ctx);
  // Ids are prefixed because the Share modal's tabs live on this page too.
  const tabs = makeTabs([
    { id: 'ed-cell', text: 'Cell', panel: cell.node },
    { id: 'ed-panel', text: 'Panel', panel: panel.node },
    { id: 'ed-access', text: 'Access', panel: access.node },
  ]);
  tabs.select('ed-panel');
  host.append(tabs.node, cell.node, panel.node, access.node);

  // Levels come from the hello the page's socket gets (S.levels), which carries each level's label and color.
  let seen = '';
  function readLevels() {
    const now = S.levels.map((l) => `${l.ui}${l.label}${l.color || l.swatch}`).join();
    if (now === seen) return;
    seen = now;
    cell.resetLevels();
    panel.resetLevels();
    paint();
  }

  const paint = () => {
    cell.update();
    panel.update();
    access.update();
  };

  let sel = '';
  ed.onChange((g) => {
    const q = rectOf(g);
    const now = q ? `${q.r1},${q.c1},${q.r2},${q.c2}` : '';
    if (now !== sel && now) tabs.select('ed-cell');
    sel = now;
    cell.update();
    panel.update();
  });
  ed.onFields(() => {
    paint();
    readLevels();
  });
  $('ed-name').addEventListener('input', () => panel.update());
  on('hello', readLevels);
  paint();
  readLevels();
}
