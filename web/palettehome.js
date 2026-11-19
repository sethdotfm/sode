// The Palette's rows that run on Home (main.js only): presets, filter sets, settings and Home's own actions. Another
// page sends its row here (palette.js takeHandoff); Home runs it once the hello, the names and the sign-in are known.
import { $, S, on, emit, wsSend } from './core.js';
import { setHomeRunner, takeHandoff, paletteRowsReady, openPalette } from './palette.js';
import { fire, fillBoxes } from './presets.js';
import { allPresets } from './presetstore.js';
import { pickSet, toggleSet } from './filterstore.js';
import { openFilters } from './filters.js';
import { toggleCompact } from './device.js';
import { showResult } from './banner.js';
import { rowByKey } from './palettecatalogue.js';

function openSettingsAt(t) {
  if (document.documentElement.dataset.view !== 'settings') $('settings-open').click();
  if (t) emit('settingsFind', t);
}

function runAction(id) {
  if (id === 'reread') {
    if (wsSend({ t: 'rereadNames' })) showResult('info', 'Re-reading router names', 'The names update when the router answers.');
  } else if (id === 'undo') emit('undoLast');
  else if (id === 'presets') emit('presetPopup');
  else if (id === 'filters') openFilters();
  else if (id === 'compact') toggleCompact();
  else if (id === 'shortcuts') emit('openShortcuts');
  else if (id === 'stepIn') $('account-stepin').click();
}

// fill: Shift+Enter. A preset fills the Patch boxes and sends nothing; a filter set adds to the stack. A preset fires
// only through the usual Confirm (fire opens it).
function runRow(row, { fill = false, offer = '' } = {}) {
  if (offer === 'stepIn') return runAction('stepIn');
  const t = row.target;
  if (row.kind === 'preset') {
    const p = allPresets().find((x) => x.scope === t.scope && x.id === t.id);
    if (!p) return showResult('warn', 'Preset not found', `${t.name} is not there now.`);
    return (fill ? fillBoxes : fire)(p.scope, p.name);
  }
  if (row.kind === 'filterSet') return (fill && t.scope !== 'all' ? toggleSet : pickSet)(t.scope, t.name);
  if (row.kind === 'setting' || row.kind === 'settingsButton') return openSettingsAt(t);
  if (row.key === 'page:settings') return openSettingsAt(null);
  if (row.kind === 'action') return runAction(t.id);
}
setHomeRunner(runRow);

// A row from another page: run it when it is still allowed here, else show it in the Palette with its reason.
let seen = false;
let pending = takeHandoff();
async function runPending() {
  // A preset needs the router's names to make its plan.
  if (!pending || !S.cfg || !seen || (pending.key.startsWith('preset:') && !S.srcBy.size)) return;
  const h = pending;
  pending = null;
  const row = rowByKey(await paletteRowsReady(), h.key);
  if (!row) return showResult('warn', 'Not found', 'The Palette row is not on this page now.');
  if (row.access.ok || h.offer) return runRow(row, h);
  openPalette();
}
on('access', () => {
  seen = true;
  runPending();
});
on('names', runPending);
on('helloData', runPending);
