// Settings view, shared by both layouts. Lives at #settings so Back and reload work. Server
// settings save together and the view stays open; This browser choices are per browser and apply
// at once. Fields: settingsform.js; save: settingssave.js; PIN prompt: settingslock.js; search: settingsfind.js.

import { $, S, on, onMsg, wsSend, sentence } from './core.js';
import { initDeviceSettings } from './devicesettings.js';
import { showResult } from './plans.js';
import { layout } from './layout.js';
import { initToc } from './settingstoc.js';
import { fillAbout } from './about.js';
import { renderPinStatus, resetPinFields } from './settingspin.js';
import { relock, unlocked, needsPin } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { form, isOpen, fillForm, fillGlobalDefaults, renderOffsetNote, renderHiddenNote, renderNamesStatus } from './settingsform.js';
import { renderLock, showLockErr, resetLock, closeLockHelp } from './settingslock.js';
import { initSave, markClean, warnUnsaved, unwarn, saving, showSaveErr } from './settingssave.js';
import { initFind } from './settingsfind.js';
import { fillRouters, routerTitle } from './settingsrouters.js';
import { initRoles } from './settingsroles.js';
import './mykeys.js';
import './users.js';
import './apisettings.js';
import './profiles.js';
import './configfile.js';

const root = document.documentElement;

function showView() {
  const want = location.hash === '#settings' && S.cfg;
  if (want && !isOpen()) openSettings();
  else if (!want && isOpen()) {
    if (warnUnsaved()) history.pushState(null, '', '#settings');
    else closeSettings();
  }
}

function exitSettings() {
  if (!warnUnsaved()) closeSettings();
}

// Bus 'settingsClose' {discard}: another module closes Settings when the form would be stale
// (for example, after a config import).
on('settingsClose', (m) => {
  if (!isOpen()) return;
  if (m?.discard) {
    unwarn();
    closeSettings();
  } else exitSettings();
});

function openSettings() {
  if (!S.cfg) return;
  showPage(-1);
  if (location.hash !== '#settings') history.pushState(null, '', '#settings');
  fillForm();
  showRouters();
  resetPinFields();
  relock(); // opens locked; a reply to an unlock sent before is dropped
  resetLock();
  renderPrefs();
  fillGlobalDefaults();
  resetClear();
  $('settings-err').hidden = true;
  $('settings-save').disabled = false;
  root.dataset.view = 'settings'; // before renderNamesStatus, which only draws when open
  roles.apply();
  renderOffsetNote();
  renderHiddenNote();
  renderNamesStatus();
  window.scrollTo(0, 0);
  markSection();
  markClean(false);
  renderLock(true);
}

function closeSettings() {
  if (location.hash === '#settings') history.replaceState(null, '', location.pathname + location.search);
  delete root.dataset.view;
  $('lock-help').hidden = true;
  relock(true);
  if (layout() === 'desktop') $('src').focus();
}

// Set after the search index is built; a my-default button added later rebuilds it.
let reindex = () => {};
const { renderPrefs } = initDeviceSettings({ form, reindex: () => reindex() });

fillAbout($('lock-exit'));
fillAbout($('settings-foot'), { id: 'open-log' });

// Every section is in index.html, so the table of contents sees them all at load.
const sections = [...form.querySelectorAll(':scope > fieldset.sode__group')];
const toc = initToc({ form, sections, toc: $('settings-toc'), isOpen });
const { showPage, markSection } = toc;
const roles = initRoles({ form, sections, toc, isOpen });

function showRouters() {
  fillRouters((id) => toc.goRouter(id));
  toc.renameRouter('1', routerTitle());
  reindex();
}

function syncLock() {
  if (S.wsState !== 'up') $('settings-save').disabled = false;
  if (isOpen()) renderPinStatus();
  if (!saving()) renderLock();
}

function resetClear() {
  $('clear-step1').hidden = false;
  $('clear-step2').hidden = true;
}
$('clear-history').addEventListener('click', () => {
  $('clear-step1').hidden = true;
  $('clear-step2').hidden = false;
  $('clear-cancel').focus();
});
$('clear-cancel').addEventListener('click', resetClear);
// The banner comes with the server's `historyCleared`, not when the request goes out (it is possible
// that the server refuses it). `clearing` is when the request went out (0: none); an answer older than the grace
// below is not ours.
let clearing = 0;
const CLEAR_GRACE_MS = 5000;
const clearPending = () => Date.now() - clearing < CLEAR_GRACE_MS;
function sendClear() {
  clearing = Date.now();
  if (!wsSend({ t: 'clearHistory' })) {
    clearing = 0;
    showSaveErr('Not connected to Sode.');
    return;
  }
  resetClear();
}
$('clear-confirm').addEventListener('click', async () => {
  if (S.pinSet && !unlocked() && !(await askPin('Clearing history needs the Advanced Settings PIN.'))) return;
  sendClear();
});
on('historyCleared', () => {
  if (!clearPending()) return;
  clearing = 0;
  showResult('ok', 'History cleared', 'History is empty for everyone.');
});
// Only a refusal with nothing to re-ask (Sode shuts down) answers this way.
onMsg('clearHistoryResult', (m) => {
  if (m.ok || !clearPending()) return;
  clearing = 0;
  showSaveErr(sentence(m.err));
});
on('wsclose', () => {
  clearing = 0;
});

for (const id of ['settings-open', 'm-settings']) $(id).addEventListener('click', openSettings);
$('settings-exit').addEventListener('click', exitSettings);
$('settings-back').addEventListener('click', () => {
  if (!toc.back()) exitSettings();
});
$('lock-cancel').addEventListener('click', exitSettings);

initSave(toc);
document.addEventListener('keydown', (e) => {
  if (!$('lock-help').hidden) {
    if (e.key === 'Escape') {
      e.preventDefault();
      closeLockHelp();
    }
    if (e.key === 'Tab') {
      e.preventDefault();
      $('lock-help-close').focus();
    }
    return;
  }
  if (isOpen() && e.key === 'Escape') {
    e.preventDefault();
    exitSettings();
  }
});
window.addEventListener('popstate', showView);

on('hello', () => {
  if (isOpen()) showRouters();
  showView();
  syncLock();
});
on('names', renderNamesStatus);
on('namesChecked', renderNamesStatus);
on('conn', () => {
  renderNamesStatus();
  syncLock();
});
on('access', () => {
  if (isOpen() && !saving()) renderLock();
});
// A bare `locked`: this connection's unlock is gone (PIN changed elsewhere, or a clear-history
// was refused). A request of ours that waits on it asks again in the popup. Otherwise only a
// connection that needs the PIN for Settings drops to the PIN prompt.
on('locked', () => {
  if (clearPending()) {
    clearing = 0;
    reask('Clearing history needs the Advanced Settings PIN.').then((ok) => {
      if (ok) sendClear();
    });
  } else if (isOpen() && !saving() && needsPin()) showLockErr('Settings are locked. Enter the PIN again.');
});

reindex = initFind({ sections, toc });
