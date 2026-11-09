// The panel editor's sidebar: Cell, Panel and Access tabs beside the grid. It opens on Panel and switches to Cell
// when the selection moves to a new square. initSidebar(editor) takes what runEditor returns.
import { $ } from './core.js';
import { sessionFetch } from './sessionkey.js';
import { makeTabs } from './tabs.js';
import { rectOf } from './gridmodel.js';
import { createCellTab } from './editorcell.js';
import { createPanelTab } from './editorpanel.js';
import { createAccessTab } from './editoraccess.js';

export function initSidebar(ed) {
  const host = $('ed-side');
  let levels = []; // [{level, label}] from the router, UI numbers; empty until the first read answers
  let reading = false;

  const ctx = { offset: () => ed.names().off || 0, levelCount: () => levels.length };
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

  async function readLevels() {
    if (levels.length || reading) return;
    reading = true;
    try {
      const res = await sessionFetch('api/v1/tally');
      levels = res.ok ? (await res.json()).levels || [] : [];
    } catch {
      levels = [];
    }
    reading = false;
    if (levels.length) {
      cell.setLevels(levels);
      panel.setLevels(levels);
      paint();
    }
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
  });
  ed.onFields(() => {
    paint();
    readLevels();
  });
  $('ed-name').addEventListener('input', () => panel.update());
  paint();
  readLevels();
}
