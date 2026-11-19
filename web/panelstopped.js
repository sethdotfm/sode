// A Public panel page whose key could not be renewed (docs/panels-design.md). The grid stays on screen, disabled,
// under a fixed status, and the page waits on its keyless stream for a public line (panelrun.js).
import { icon } from './core.js';
import { PUBLIC_STOPPED } from './panelkeyrules.js';

export function showStopped(ui) {
  ui.main.classList.add('sode-panels--stopped');
  for (const n of [ui.grid, ui.levels]) if (n) n.inert = true;
  ui.note.setAttribute('role', 'status');
  ui.note.dataset.stopped = '';
  ui.note.replaceChildren(icon('circle-pause'), ` ${PUBLIC_STOPPED}`);
  ui.note.hidden = false;
}
