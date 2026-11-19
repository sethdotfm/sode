// Settings view, shared by both layouts. Lives at #settings so Back and reload work. Server
// settings save together and the view stays open; This browser choices are per browser and apply
// at once. Fields: settingsform.js; save: settingssave.js; search: settingsfind.js.

import { $, S, on, emit, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
import { initDeviceSettings } from './devicesettings.js';
import { showResult } from './plans.js';
import { layout } from './layout.js';
import { initToc } from './settingstoc.js';
import { fillAbout } from './about.js';
import { askRecheck } from './recheck.js';
import { askApproval } from './approval.js';
import { form, isOpen, fillForm, fillGlobalDefaults, renderOffsetNote, renderHiddenNote, renderNamesStatus } from './settingsform.js';
import { initSave, markClean, warnUnsaved, unwarn, showSaveErr } from './settingssave.js';
import { initFind } from './settingsfind.js';
import { fillRouters, routerTitle } from './settingsrouters.js';
import { initRoles } from './settingsroles.js';
import './mykeys.js';
import './myclients.js';
import './users.js';
import './apisettings.js';
import './accesspreset.js';
import './setupagain.js';
import './profiles.js';
import './exportdialog.js';
import './ownexport.js';
import './swap.js';
import { askBrokenCounts } from './brokencount.js';

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
  if (layout() === 'desktop') $('settings-find').focus({ preventScroll: true });
}

function closeSettings() {
  if (location.hash === '#settings') history.replaceState(null, '', location.pathname + location.search);
  delete root.dataset.view;
  if (layout() === 'desktop') $('src').focus();
}

// Set after the search index is built; a my-default button added later rebuilds it.
let reindex = () => {};
const { renderPrefs } = initDeviceSettings({ form, reindex: () => reindex() });

fillAbout($('settings-foot'), { id: 'open-log' });

// Every section is in index.html, so the table of contents sees them all at load.
const sections = [...form.querySelectorAll(':scope > fieldset.sode__group')];
const toc = initToc({ form, sections, toc: $('settings-toc'), isOpen });
const { showPage, markSection } = toc;
const roles = initRoles({ form, sections, toc, isOpen });

function showRouters() {
  fillRouters((id) => toc.goRouter(id));
  askBrokenCounts();
  toc.renameRouter('1', routerTitle());
  reindex();
}

function syncSave() {
  if (S.wsState !== 'up') $('settings-save').disabled = false;
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
let clearAnswer = null; // the Approval popup of a clear sent with an Approval
let myRole = '';
function sendClear(approval = null, answer = null) {
  clearing = Date.now();
  if (!wsSend({ t: 'clearHistory', ...(approval && { approval }) })) {
    clearing = 0;
    if (!answer) showSaveErr(LINK_DOWN);
    return false;
  }
  clearAnswer = answer;
  resetClear();
  return true;
}
on('access', (a) => {
  myRole = a.role || '';
});
$('clear-confirm').addEventListener('click', () => sendClear());
on('historyCleared', () => {
  if (!clearPending()) return;
  clearing = 0;
  clearAnswer?.done();
  clearAnswer = null;
  showResult('ok', 'History cleared', 'History is empty for everyone.');
});
// A refusal asks a PIN recheck and sends the clear again; below an Administrator it asks an Approval.
onMsg('clearHistoryResult', (m) => {
  if (m.ok || !clearPending()) return;
  clearing = 0;
  const answer = clearAnswer;
  clearAnswer = null;
  if (answer && m.approval) return void answer.fail(m.err);
  answer?.done();
  if (!answer && !m.locked && myRole && myRole !== 'administrator') {
    askApproval('Clearing History', 'an Administrator', (ap, ans) => sendClear(ap, ans), {
      action: () => ({ t: 'clearHistory' }),
      // The server clears when approved; the grace then covers its historyCleared.
      onEnd: (end) => {
        if (end === 'approved') clearing = Date.now();
      },
    });
    return;
  }
  if (m.recheck) {
    askRecheck('Clearing History').then((ok) => (ok ? sendClear() : showSaveErr(sentence(m.err))));
    return;
  }
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

initSave(toc);
document.addEventListener('keydown', (e) => {
  if (isOpen() && e.key === 'Escape') {
    e.preventDefault();
    exitSettings();
  }
});
window.addEventListener('popstate', showView);

// The Broken banner's and the Broken lists' Swap…: Swap stages into this page's one Save, so Settings opens first.
on('swapAt', (m) => {
  if (!isOpen()) openSettings();
  toc.goRouter('1');
  emit('openSwap', { mine: false, side: m.side, from: m.from });
});

on('hello', () => {
  if (isOpen()) showRouters();
  showView();
  syncSave();
});
on('names', renderNamesStatus);
on('namesChecked', renderNamesStatus);
on('conn', () => {
  renderNamesStatus();
  syncSave();
});

reindex = initFind({ sections, toc });
