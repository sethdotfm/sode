// The Palette's rows that run on Home (main.js only): presets, filter sets, settings, Settings buttons and Home's own
// actions. Another page sends its row here (palette.js takeHandoff); Home runs it once the hello, the names and the
// sign-in are known.
import { $, S, on, emit, routerData } from './core.js';
import { setHomeRunner, takeHandoff, paletteRowsReady, openPalette } from './palette.js';
import { fire, fillBoxes } from './presets.js';
import { allPresets } from './presetstore.js';
import { pickSet, toggleSet } from './filterstore.js';
import { openFilters } from './filters.js';
import { toggleCompact, compactOn } from './device.js';
import { showResult } from './banner.js';
import { rowByKey } from './palettecatalogue.js';
import { openProtectPick } from './protectpick.js';

function openSettingsAt(t) {
  if (document.documentElement.dataset.view !== 'settings') emit('settingsOpen');
  if (t) emit('settingsFind', t);
}

function runAction(id) {
  if (id === 'undo') emit('undoLast');
  else if (id === 'redo') emit('redoLast');
  else if (id === 'presets') emit('presetPopup');
  else if (id === 'filters') openFilters();
  else if (id === 'compact') {
    toggleCompact();
    showResult('done', 'Compact view changed', `Compact view is ${compactOn() ? 'on' : 'off'} now.`);
  } else if (id === 'shortcuts') emit('openShortcuts');
  else if (id === 'stepIn') $('account-stepin').click();
  else if (id === 'protect') openProtectPick();
}

// A Settings button (buttonMode in palettecatalogue.js) is pressed, so its own guards run, such as the PIN of a dialog
// or the Clear History confirm. 'run' presses it over this view; 'settings' opens Settings at it first. A button that
// cannot be pressed now (disabled, or hidden for this user) is shown in Settings, which tells why. press (a bookmark)
// also presses a 'locate' button.
// A my default button that a row pressed: its save gives a result banner (devicesettings.js emits myDefaultDone).
let myDefaultPressed = '';
on('myDefaultDone', (d) => {
  if (d.id !== myDefaultPressed) return;
  myDefaultPressed = '';
  showResult('done', 'My default changed', d.text);
  // The pressed button can hide after the save, which drops the focus to body. The setting keeps it.
  if (!document.activeElement || document.activeElement === document.body) $(d.field)?.focus({ preventScroll: true });
});

function pressButton(t, press) {
  const mode = t.mode === 'locate' && press ? 'settings' : t.mode;
  if (/^my-default-/.test(t.button)) myDefaultPressed = t.button;
  if (mode !== 'run' && mode !== 'settings') return openSettingsAt(t);
  const b = document.getElementById(t.button);
  const ok = b && !b.disabled && !b.closest('[hidden]');
  if (mode === 'settings' || !ok) openSettingsAt(t);
  if (ok) b.click();
}

// fill: Shift+Enter. A preset fills the Patch boxes and sends nothing; a filter set adds to the stack. A preset fires
// only through the usual Confirm (fire opens it).
function runRow(row, { fill = false, offer = '', press = false } = {}) {
  if (offer === 'stepIn') return runAction('stepIn');
  const t = row.target;
  if (row.kind === 'preset') {
    const p = allPresets().find((x) => x.scope === t.scope && x.id === t.id);
    if (!p) return showResult('warn', 'Preset not found', `${t.name} is not there now.`);
    return (fill ? fillBoxes : fire)(p.scope, p.name);
  }
  if (row.kind === 'filterSet') return (fill && t.scope !== 'all' ? toggleSet : pickSet)(t.scope, t.id);
  if (row.kind === 'settingsButton') return pressButton(t, press);
  if (row.kind === 'setting') return openSettingsAt(t);
  if (row.key === 'page:settings') return openSettingsAt(null);
  if (row.key === 'page:home') return emit('homeKey');
  if (row.kind === 'action') return runAction(t.id);
}
setHomeRunner(runRow);

// A row from another page: run it when it is still allowed here, else show it in the Palette with its reason.
let seen = false;
let pending = takeHandoff();
async function runPending() {
  // A preset needs the router's names to make its plan.
  if (!pending || !S.cfg || !seen || (pending.key.startsWith('preset:') && !routerData('').srcBy.size)) return;
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
