// The panel editor's sidebar: Cell, Panel and Access tabs beside the grid, or beside a Grid panel's Sources and
// Destinations lists (#ed-lists). It opens on Panel and switches to Cell when the selection moves to new squares or
// rows. On a Grid panel the Cell tab edits the rows selected in one list; a selection in the other list ends it.
// initSidebar(editor) takes what runEditor returns.
import { $, S, on } from './core.js';
import { makeTabs } from './tabs.js';
import { rectOf } from './gridmodel.js';
import { createCellTab } from './editorcell.js';
import { createPanelTab } from './editorpanel.js';
import { createAccessTab } from './editoraccess.js';
import { createEdgeList } from './editorgridlist.js';

export function initSidebar(ed) {
  const host = $('ed-side');

  const ctx = { offset: () => ed.names().off || 0, levelCount: () => S.levels.length };
  let active = null; // the list whose selection the Cell tab edits
  const picked = () => {
    const rows = active?.picked() || [];
    return rows.length ? { list: active, rows } : null;
  };
  const onPick = (list) => {
    const any = list.picked().length > 0;
    if (any && active !== list) {
      active?.clearPicks();
      active = list;
    }
    if (any) tabs.select('ed-cell');
    cell.update();
  };
  // A list's Artwork button: the Live Editor shows the Cell tab in its drawer (editorlive.js), else the Cell tab opens here.
  const onArt = () => {
    if (ed.openArt()) return;
    tabs.select('ed-cell');
    cell.openPicker();
  };
  const srcs = createEdgeList(ed, { key: 'sources', rights: 'sources', what: 'source', side: 'left to right on the top edge', onPick, onArt });
  const dsts = createEdgeList(ed, { key: 'destinations', rights: 'dsts', what: 'destination', side: 'top to bottom on the left edge', onPick, onArt });
  $('ed-lists').append(srcs.node, dsts.node);
  ed.useEdgeLists({ srcs, dsts });

  const cell = createCellTab(ed, { picked });
  const panel = createPanelTab(ed);
  const access = createAccessTab(ed, ctx);
  // Ids are prefixed because the Share modal's tabs live on this page too.
  const tabs = makeTabs([
    { id: 'ed-cell', text: 'Cell', panel: cell.node },
    { id: 'ed-panel', text: 'Panel', panel: panel.node },
    { id: 'ed-access', text: 'Access', panel: access.node },
  ]);
  host.append(tabs.node, cell.node, panel.node, access.node);
  ed.useCellTab({ node: cell.node, openPicker: () => cell.openPicker() });
  tabs.select('ed-panel');

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
    srcs.update();
    dsts.update();
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
  return {
    // The toolbar's Rename (editorname.js): the Panel tab's Name field, where the name is edited.
    renameField() {
      tabs.select('ed-panel');
      const name = $('ed-side-name');
      name.focus();
      name.select();
    },
  };
}
