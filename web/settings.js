// Settings view, shared by both layouts. Lives at #settings so Back and reload work. Server
// settings save together and the view stays open; This device choices are per browser and apply
// at once.

import { $, el, S, on, emit, wsSend, sentence } from './core.js';
import { levelColor, levelLabel, levelInput } from './levelview.js';
import { fmtTime, ownPref, globalPref, defaultPref, myPref, setPref, pref } from './device.js';
import { myDefaultButtons } from './mydefaults.js';
import { profile, profileName, saveProfile } from './profile.js';
import { showResult } from './plans.js';
import { parseLevelsAs } from './levels.js';
import { PREFS, resolve, toGlobal } from './prefs.js';
import { iconURL, setBrand } from './brandicon.js';
import { layout } from './layout.js';
import { parseFilter, compileFilter } from './filter.js';
import { initToc } from './settingstoc.js';
import { fillAbout } from './about.js';
import { renderPinStatus, showPinFields, resetPinFields } from './settingspin.js';
import { unlock, relock, unlocked, trusted, needsPin } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { guardedEdited, loginOffered } from './lockrules.js';
import { omitOwned } from './owned.js';
import { intProblem } from './settingsrules.js';
import { buildIndex, find, slug } from './settingsindex.js';
import './users.js'; // adds its section before the table of contents is built
import './profiles.js';
import './configfile.js';

const root = document.documentElement;
const form = $('settings-view');
const fields = () => form.querySelectorAll('[data-key]');
const isOpen = () => root.dataset.view === 'settings';
// A save is in flight: its own hello / locked arrive before configResult closes the view, so
// don't flip to the PIN prompt for them.
const saving = () => $('settings-save').disabled;

const formMode = () => form.querySelector('input[name="lvlnum"]:checked')?.value || '1';
const formLevels = () => Number($('cfg-levels').value);
let shownMode = '1';

function showView() {
  const want = location.hash === '#settings' && S.cfg;
  if (want && !isOpen()) openSettings();
  else if (!want && isOpen()) {
    if (warnUnsaved()) history.pushState(null, '', '#settings');
    else closeSettings();
  }
}

// ---- unsaved changes: the form against the last opened or saved state. data-apart blocks save apart from the form and aren't counted.

let clean = '';
let savedAt = null;
const formState = () =>
  JSON.stringify(
    [...form.querySelectorAll('input, select, textarea')]
      .filter((f) => !f.closest('[data-apart]') && f.id !== 'settings-find')
      .map((f) => (f.type === 'checkbox' || f.type === 'radio' ? f.checked : f.value)),
  );
const dirty = () => isOpen() && formState() !== clean;
const warned = () => !$('settings-unsaved').hidden;

function markClean(saved) {
  clean = formState();
  savedAt = saved ? new Date().toISOString() : null;
  unwarn();
}

function warnUnsaved() {
  if (!dirty() || warned()) return false;
  $('settings-unsaved').hidden = false;
  $('settings-exit').textContent = 'Discard and exit';
  $('settings-exit').classList.replace('pb-btn--outline', 'pb-btn--danger');
  renderSaveState();
  return true;
}

function unwarn() {
  $('settings-unsaved').hidden = true;
  $('settings-exit').textContent = 'Exit';
  $('settings-exit').classList.replace('pb-btn--danger', 'pb-btn--outline');
  renderSaveState();
}

function exitSettings() {
  if (!warnUnsaved()) closeSettings();
}

// Bus 'settingsClose' {discard}: another module closes Settings when the form would be stale
// (e.g. after a config import).
on('settingsClose', (m) => {
  if (!isOpen()) return;
  if (m?.discard) {
    unwarn();
    closeSettings();
  } else exitSettings();
});

function renderSaveState() {
  const path = S.configPath ? el('code', null, S.configPath) : null;
  $('settings-path').replaceChildren(
    ...(dirty()
      ? warned()
        ? []
        : ['Unsaved changes']
      : !path
        ? [savedAt ? `Applied ${fmtTime(savedAt)}; not saved: ` : 'Not saved: ', el('code', null, 'SODE_CONFIG'), ' unset']
        : savedAt
          ? [`Saved ${fmtTime(savedAt)} to `, path]
          : ['Saved to ', path]),
  );
}

// Previewed before save: window border and the app icon the server draws (icon.go).
function renderBrandPreview() {
  const sel = $('cfg-brandborder');
  const c = sel.value;
  setBrand($('brand-swatch'), c);
  $('brand-icon').src = iconURL(180, c);
  $('brand-name').textContent = sel.selectedOptions[0]?.textContent || 'Off';
}
function fillBrandColors() {
  const sel = $('cfg-brandborder');
  const opt = (value, text) => Object.assign(el('option', null, text), { value });
  sel.replaceChildren(opt('', 'Off'), ...S.brandColors.map((c) => opt(c.name, c.label)));
}
$('cfg-brandborder').addEventListener('change', renderBrandPreview);

export function openSettings() {
  if (!S.cfg) return;
  showPage(-1);
  if (location.hash !== '#settings') history.pushState(null, '', '#settings');
  fillBrandColors();
  for (const f of fields()) {
    const v = S.cfg[f.dataset.key];
    if (f.dataset.type === 'bool') f.checked = !!v;
    else if (f.dataset.type === 'list') f.value = (v || []).join(', ');
    else f.value = v ?? '';
  }
  $(`cfg-lvlnum-${S.cfg.levelNumbering || '1'}`).checked = true;
  shownMode = formMode();
  $('cfg-default').value = levelInput(parseLevelsAs(S.cfg.defaultLevels, '1', S.cfg.levels) || [], shownMode);
  renderNumbering();
  renderBrandPreview();
  renderColors(S.cfg.levelColors || []);
  $('cfg-lock-main').checked = !!S.cfg.locks?.main;
  $('cfg-lock-log').checked = !!S.cfg.locks?.log;
  // What the form showed: only a change from it is sent, so a form opened before someone else
  // changed the locks can't undo their change.
  shownAccess = { main: !!S.cfg.locks?.main, log: !!S.cfg.locks?.log, auto: S.cfg.autoLockMinutes ?? 0 };
  resetPinFields();
  relock(); // opens locked; a reply to an unlock sent before is dropped
  $('lock-pin').value = '';
  $('lock-unlock').disabled = false;
  $('lock-err').hidden = true;
  renderPrefs();
  for (const s of form.querySelectorAll('select[data-gpref]')) s.value = resolve(s.dataset.gpref, null, null, S.cfg.deviceDefaults);
  resetClear();
  $('settings-err').hidden = true;
  $('settings-save').disabled = false;
  root.dataset.view = 'settings'; // before renderNamesStatus, which only draws when open
  renderOffsetNote();
  renderHiddenNote();
  renderNamesStatus();
  window.scrollTo(0, 0);
  markSection();
  markClean(false);
  renderLock(true);
}

// Focus moves only when this flips (or on open), not on every status update.
function renderLock(opening = false, afterPin = false) {
  if (!isOpen()) return;
  const locked = needsPin();
  const flipped = locked !== 'locked' in form.dataset;
  if (locked) form.dataset.locked = '';
  else {
    delete form.dataset.locked;
    $('lock-help').hidden = true;
  }
  if (!opening && !flipped) return;
  if (layout() !== 'desktop') return;
  if (locked) $('lock-pin').focus();
  else if (afterPin) $('settings-find').focus({ preventScroll: true });
  else $('cfg-namerefresh').focus();
}

function closeSettings() {
  if (location.hash === '#settings') history.replaceState(null, '', location.pathname + location.search);
  delete root.dataset.view;
  $('lock-help').hidden = true;
  relock(true);
  if (layout() === 'desktop') $('src').focus();
}

let shownAccess = null;

function readSettings() {
  const c = omitOwned(S.cfg);
  for (const f of fields()) {
    const k = f.dataset.key;
    if (f.dataset.type === 'bool') c[k] = f.checked;
    else if (f.dataset.type === 'list')
      c[k] = f.value
        .split(',')
        .map((x) => x.trim())
        .filter(Boolean);
    else if (f.dataset.type === 'int') c[k] = f.value.trim() === '' ? NaN : Number(f.value);
    else c[k] = f.value.trim();
  }
  // Locks and idle time are sent only when edited; a missing key keeps the live value. Blank
  // idle time means 0 (never).
  if (Number.isNaN(c.autoLockMinutes)) c.autoLockMinutes = 0;
  const locks = { main: $('cfg-lock-main').checked, log: $('cfg-lock-log').checked };
  if (shownAccess && locks.main === shownAccess.main && locks.log === shownAccess.log) delete c.locks;
  else c.locks = locks;
  if (shownAccess && c.autoLockMinutes === shownAccess.auto) delete c.autoLockMinutes;
  c.levelColors = [...$('cfg-colors').querySelectorAll('input')].map((f) => f.value.trim());
  c.levelNumbering = formMode();
  c.deviceDefaults = Object.fromEntries([...form.querySelectorAll('select[data-gpref]')].map((s) => [s.dataset.gpref, toGlobal(s.dataset.gpref, s.value)]));

  const def = $('cfg-default').value.trim();
  const list = def ? parseLevelsAs(def, c.levelNumbering, formLevels()) : [];
  c.defaultLevels = list ? levelInput(list, '1') : null;
  return c;
}

// ---- This device and Global defaults: the same choices (prefs.js). A device one starts at "Everyone's default (value)" ("My" with their own), which follows the default; applies at once. A default one is saved with the form.

const prefOptions = (name, globalText) => {
  const opt = (value, text) => Object.assign(el('option', null, text), { value });
  return [...(globalText ? [opt('', globalText)] : []), ...Object.entries(PREFS[name].values).map(([v, text]) => opt(v, text))];
};

for (const s of form.querySelectorAll('select[data-gpref]')) s.replaceChildren(...prefOptions(s.dataset.gpref));

const originText = (name, v) => {
  const text = PREFS[name].values[v];
  if (myPref(name) === v) return `${text} (my default)`;
  if (globalPref(name) === v) return `${text} (everyone's default)`;
  return text;
};

// Built on the first profile, after the search index was made, so creating one rebuilds the
// index (indexNow).
let indexNow = () => {};
const myButtons = new Map();
function myButton(select) {
  const name = select.dataset.pref;
  let mb = myButtons.get(name);
  if (mb) return mb;
  const mk = (id, label) => {
    const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__my-default', label);
    b.type = 'button';
    b.dataset.find = id;
    b.dataset.label = label;
    return b;
  };
  const save = mk(`my-default-${name}`, 'Save as my default');
  const clear = mk(`my-default-clear-${name}`, 'Clear my default');
  const note = el('span', 'sode__hint');
  note.hidden = true;
  note.setAttribute('role', 'status');
  const run = (change, after) => async () => {
    const p = profile();
    if (!p.name) return;
    const prefs = { ...p.prefs };
    change(prefs);
    save.disabled = clear.disabled = true;
    const r = await saveProfile({ prefs });
    save.disabled = clear.disabled = false;
    note.hidden = r.ok;
    note.textContent = r.ok ? '' : r.err || 'Could not save.';
    if (r.ok) after();
  };
  save.addEventListener(
    'click',
    run(
      (prefs) => {
        prefs[name] = pref(name);
      },
      () => setPref(name, ''),
    ),
  );
  clear.addEventListener(
    'click',
    run(
      (prefs) => {
        delete prefs[name];
      },
      () => renderPrefs(),
    ),
  );
  const row = el('div', 'sode__my-defaults');
  row.append(save, clear, note);
  select.closest('.sode__field').append(row);
  mb = { save, clear, note, row };
  myButtons.set(name, mb);
  indexNow();
  return mb;
}

// On any change of settings in force, so "Global (value)" stays true under an open form, and
// Alt+Shift+T shows.
function renderPrefs() {
  const user = profileName();
  for (const s of form.querySelectorAll('select[data-pref]')) {
    const name = s.dataset.pref;
    const opt = (value, text) => Object.assign(el('option', null, text), { value });
    s.replaceChildren(
      opt('', `${myPref(name) ? 'My' : "Everyone's"} default (${PREFS[name].values[defaultPref(name)]})`),
      ...Object.keys(PREFS[name].values).map((v) => opt(v, originText(name, v))),
    );
    s.value = ownPref(name);
    const mb = myButtons.get(name) || (user ? myButton(s) : null);
    if (!mb) continue;
    const show = myDefaultButtons(pref(name), myPref(name));
    mb.save.hidden = !user || !show.save;
    mb.clear.hidden = !user || !show.clear;
    mb.row.hidden = !user;
    if (!user) mb.note.hidden = true;
  }
}
renderPrefs();
on('prefs', renderPrefs);
on('profile', renderPrefs);

let acctUser = '';
on('access', (L) => {
  acctUser = L.user || '';
  const acct = !!L.user || loginOffered(L);
  $('account-name').hidden = !acct;
  $('account-name').replaceChildren(...(L.user ? ['Logged in as ', el('strong', null, L.user)] : ['Not logged in']));
  for (const id of ['account-profile', 'account-transfer', 'account-pin', 'account-logout']) $(id).hidden = !L.user;
  $('account-login').hidden = !!L.user || !acct;
  $('soft-lock-btn').hidden = L.enforced;
});
$('soft-lock-btn').addEventListener('click', () => emit('softLock'));
$('account-pin').addEventListener('click', () => emit('changePin'));
$('account-profile').addEventListener('click', () => emit('openProfile', { name: acctUser }));
$('account-transfer').addEventListener('click', () => emit('openTransfer', { scope: 'user', user: acctUser }));
$('account-logout').addEventListener('click', () => emit('logout'));
$('account-login').addEventListener('click', () => emit('login'));

fillAbout($('lock-exit'));
fillAbout($('settings-foot'), { id: 'open-log' });

const sections = [...form.querySelectorAll(':scope > fieldset.sode__group')];
const toc = $('settings-toc');
const tocApi = initToc({ form, sections, toc, isOpen });
const { phone, showPage, reveal, focusField, markSection } = tocApi;

function renderNumbering() {
  const n = formLevels();
  const span = (mode) => (Number.isInteger(n) && n > 1 ? `${levelLabel(1, mode)}–${levelLabel(n, mode)}` : levelLabel(1, mode));
  form.querySelector('label[for="cfg-lvlnum-0"]').textContent = span('0');
  form.querySelector('label[for="cfg-lvlnum-1"]').textContent = span('1');
  form.querySelector('label[for="cfg-lvlnum-va"]').textContent = Number.isInteger(n) && n > 1 ? `V + ${levelLabel(2, 'va')}–${levelLabel(n, 'va')}` : 'V';
}

// Rewrite the default levels box in the new numbering (if it parsed in the old one) and relabel
// previews. Not limited to the level count, which may be mid-edit: Save checks that.
function numberingChanged() {
  const f = $('cfg-default');
  const list = f.value.trim() && parseLevelsAs(f.value, shownMode, Infinity);
  if (list) f.value = levelInput(list, formMode());
  shownMode = formMode();
  renderNumbering();
  renderOffsetNote();
  rerenderColors();
}

const HEX = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i;
const toHex = (v) => {
  const h = v.trim().replace(/^#/, '').toLowerCase();
  return '#' + (h.length === 3 ? [...h].map((c) => c + c).join('') : h);
};

const swatchOf = (ui) => (ui === 1 ? 'video' : String(((ui - 2) % 16) + 1).padStart(2, '0'));

function defaultHex(ui) {
  if (S.levelDefaults[ui - 1]) return S.levelDefaults[ui - 1];
  const probe = el('span');
  probe.style.color = `var(--pb-signal-${swatchOf(ui)})`;
  document.body.append(probe);
  const rgb = getComputedStyle(probe).color.match(/\d+/g) || [];
  probe.remove();
  return (
    '#' +
    rgb
      .slice(0, 3)
      .map((c) => Number(c).toString(16).padStart(2, '0'))
      .join('')
  );
}

function preview(slot, ui, v) {
  levelColor(slot, { swatch: swatchOf(ui), color: HEX.test(v.trim()) ? toHex(v) : S.levelDefaults[ui - 1] || '' });
}

function renderColors(vals) {
  const n = Number($('cfg-levels').value);
  const rows = [];
  for (let ui = 1; Number.isInteger(n) && ui <= Math.min(n, 256); ui++) {
    const slot = el('span', 'sode__slot sode__slot--on', levelLabel(ui, formMode()));
    slot.setAttribute('aria-hidden', 'true');
    const f = el('input', 'pb-input');
    f.id = `cfg-color-${ui}`;
    f.value = vals[ui - 1] || '';
    f.placeholder = defaultHex(ui);
    f.spellcheck = false;
    f.setAttribute('aria-label', `Level ${levelLabel(ui, formMode())} color`);
    f.addEventListener('input', () => {
      preview(slot, ui, f.value);
      f.classList.toggle('pb-input--invalid', f.value.trim() !== '' && !HEX.test(f.value.trim()));
    });
    preview(slot, ui, f.value);
    rows.push(el('div', 'sode__color', slot, f));
  }
  $('cfg-colors').replaceChildren(...rows);
}

function rerenderColors() {
  const vals = [...$('cfg-colors').querySelectorAll('input')].map((f) => f.value);
  renderColors([...vals, ...(S.cfg.levelColors || []).slice(vals.length)]);
}

function renderOffsetNote() {
  const off = Number($('cfg-offset').value);
  $('cfg-offset-note').textContent = Number.isInteger(off) ? `Level ${levelLabel(1, formMode())} = wire level ${1 + off}` : '';
}

function renderHiddenNote() {
  const f = $('cfg-hidden');
  const p = parseFilter(f.value);
  f.classList.toggle('pb-input--invalid', !!p.error);
  let text = '';
  if (p.error) text = `${p.error}.`;
  else if (p.rules.length) {
    const fn = compileFilter(p.rules);
    const n = S.dsts.filter((d) => fn(d.n + (S.cfg?.numberOffset ?? 0), d.name)).length;
    text = `${n} of ${S.dsts.length} destinations`;
  }
  $('cfg-hidden-note').textContent = text;
}

function renderNamesStatus() {
  if (!isOpen()) return;
  const at = S.namesAt && new Date(S.namesAt);
  $('names-at').textContent = at && at.getFullYear() > 2000 ? `Refreshed ${fmtTime(S.namesAt)}` : 'Not loaded yet';
  $('names-status').textContent = `${S.srcs.length} sources, ${S.dsts.length} destinations`;
  const btn = $('names-refresh');
  btn.disabled = S.router !== 'up';
  btn.textContent = 'Refresh now';
}

function configResult(m) {
  $('settings-save').disabled = false;
  if (m.ok) {
    if (!isOpen()) return;
    // Stay open. A changed PIN has relocked everyone, this view included.
    resetPinFields();
    $('settings-err').hidden = true;
    markClean(true);
    renderLock();
    return;
  }
  if (m.locked) {
    relock();
    // Not trusted: Settings flips to its own PIN prompt. A trusted user's form never did, so
    // ask in the popup and save again.
    if (needsPin()) return showLockErr(m.err);
    reask('Saving that needs the Advanced Settings PIN.').then((ok) => {
      if (ok) form.requestSubmit();
    });
    return;
  }
  $('settings-err').textContent = sentence(m.err);
  $('settings-err').hidden = false;
}

function showLockErr(text) {
  renderLock();
  $('lock-err').textContent = sentence(text);
  $('lock-err').hidden = false;
  $('lock-pin').select();
}

async function sendUnlock() {
  $('lock-unlock').disabled = true;
  const r = await unlock($('lock-pin').value);
  if (r.superseded) return;
  $('lock-unlock').disabled = false;
  if (r.stale || !isOpen()) return;
  if (!r.ok) return showLockErr(r.err);
  $('lock-pin').value = '';
  $('lock-err').hidden = true;
  renderLock(false, true);
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
// The banner comes with the server's `historyCleared`, not when the request goes out (it may be
// refused). `clearing` is when the request went out (0: none); an answer older than the grace
// below isn't ours.
let clearing = 0;
const CLEAR_GRACE_MS = 5000;
const clearPending = () => Date.now() - clearing < CLEAR_GRACE_MS;
function sendClear() {
  clearing = Date.now();
  if (!wsSend({ t: 'clearHistory' })) {
    clearing = 0;
    $('settings-err').textContent = 'Not connected to Sode.';
    $('settings-err').hidden = false;
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
on('wsclose', () => {
  clearing = 0;
});

for (const id of ['settings-open', 'm-settings']) $(id).addEventListener('click', openSettings);
$('settings-exit').addEventListener('click', exitSettings);
$('settings-back').addEventListener('click', () => {
  if (!tocApi.back()) exitSettings();
});
$('lock-cancel').addEventListener('click', exitSettings);

function openLockHelp() {
  $('lock-help').hidden = false;
  $('lock-help-dialog').focus();
}
function closeLockHelp() {
  $('lock-help').hidden = true;
  if (isOpen()) $('lock-pin').focus();
}
$('lock-help-open').addEventListener('click', openLockHelp);
$('lock-help-close').addEventListener('click', closeLockHelp);
for (const ev of ['input', 'change']) {
  form.addEventListener(ev, () => {
    $('settings-err').hidden = true;
    return warned() ? unwarn() : renderSaveState();
  });
}
window.addEventListener('beforeunload', (e) => {
  if (dirty()) e.preventDefault();
});
$('cfg-offset').addEventListener('input', renderOffsetNote);
$('cfg-hidden').addEventListener('input', renderHiddenNote);
$('cfg-colors-reset').addEventListener('click', () => {
  for (const f of $('cfg-colors').querySelectorAll('input')) {
    f.value = '';
    f.dispatchEvent(new Event('input'));
  }
});
$('cfg-levels').addEventListener('input', () => {
  rerenderColors();
  renderNumbering();
});
for (const r of form.querySelectorAll('input[name="lvlnum"]')) r.addEventListener('change', numberingChanged);
for (const s of form.querySelectorAll('select[data-pref]')) s.addEventListener('change', () => setPref(s.dataset.pref, s.value));
$('names-refresh').addEventListener('click', () => {
  if (wsSend({ t: 'refreshNames' })) {
    $('names-refresh').disabled = true;
    $('names-refresh').textContent = 'Refreshing…';
  }
});
const askThenSave = (why) =>
  askPin(why).then((ok) => {
    if (ok) form.requestSubmit();
  });
form.addEventListener('submit', (e) => {
  e.preventDefault();
  if ('locked' in form.dataset) return void sendUnlock();
  // A trusted user saves without the PIN, but not a change to the PIN itself: ask now instead
  // of after a refusal.
  if (S.pinSet && trusted() && !unlocked() && ($('cfg-pin').value || $('cfg-pin-clear').checked)) {
    return void askThenSave('Changing the PIN needs the Advanced Settings PIN.');
  }
  const c = readSettings();
  if (S.pinSet && trusted() && !unlocked() && guardedEdited(c, S.cfg)) {
    return void askThenSave('Changing that setting needs the Advanced Settings PIN.');
  }
  const bad = [...fields()].find((f) => intProblem([{ key: f.dataset.key, type: f.dataset.type, value: f.value }]));
  const badColor = [...$('cfg-colors').querySelectorAll('input')].find((f) => f.value.trim() && !HEX.test(f.value.trim()));
  if (badColor) {
    $('settings-err').textContent = `${badColor.getAttribute('aria-label')} must be hex, like #3a7bd5.`;
    $('settings-err').hidden = false;
    focusField(badColor);
    return;
  }
  if (c.defaultLevels === null) {
    $('settings-err').textContent = `Default levels: "${$('cfg-default').value.trim()}" isn't a level or range here.`;
    $('settings-err').hidden = false;
    focusField($('cfg-default'));
    return;
  }
  const hidden = parseFilter(c.hiddenDsts);
  if (hidden.error) {
    $('settings-err').textContent = `Globally hidden destinations: ${hidden.error}.`;
    $('settings-err').hidden = false;
    focusField($('cfg-hidden'));
    return;
  }
  if (bad) {
    $('settings-err').textContent = `${form.querySelector(`label[for="${bad.id}"]`)?.textContent || 'A number field'} must be a whole number.`;
    $('settings-err').hidden = false;
    focusField(bad);
    return;
  }
  const pin = $('cfg-pin').value;
  if (pin && [...pin].length < 4) {
    $('settings-err').textContent = 'The settings PIN must be at least 4 characters.';
    $('settings-err').hidden = false;
    showPinFields(true);
    focusField($('cfg-pin'));
    return;
  }
  if (pin !== $('cfg-pin2').value) {
    $('settings-err').textContent = "The two PINs don't match.";
    $('settings-err').hidden = false;
    showPinFields(true);
    reveal($('cfg-pin2'));
    $('cfg-pin2').select();
    return;
  }
  if (!wsSend({ t: 'config', config: c, pin, clearPin: $('cfg-pin-clear').checked })) {
    $('settings-err').textContent = 'Not connected to Sode.';
    $('settings-err').hidden = false;
    return;
  }
  $('settings-save').disabled = true;
});
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
// was refused). A request of ours waiting on it asks again in the popup; otherwise only a
// connection that needs the PIN for Settings drops to the PIN prompt.
on('locked', () => {
  if (clearPending()) {
    clearing = 0;
    reask('Clearing history needs the Advanced Settings PIN.').then((ok) => {
      if (ok) sendClear();
    });
  } else if (isOpen() && !saving() && needsPin()) showLockErr('Settings are locked; enter the PIN again.');
});
on('configResult', configResult);

// ---- settings search. Picking a hit scrolls to it, focuses a control and outlines it with a "Found" tag. Esc clears the box first; a second Esc leaves Settings.

const findBox = $('settings-find');
const foundList = $('settings-found');
const secIds = sections.map((sec) => slug(sec.querySelector('legend').textContent));
const targets = new Map();

function settingsOf(sec, secId) {
  const out = new Map();
  const add = (id, label, node) => {
    label = (label || '').trim();
    if (id && label && !out.has(id)) {
      out.set(id, { id, label });
      targets.set(`${secId}/${id}`, node);
    }
  };
  for (const lab of sec.querySelectorAll('label[for]')) {
    if (!lab.closest('.pb-btn-group')) add(lab.htmlFor, lab.textContent, lab.closest('.sode__field') || lab);
  }
  for (const lab of sec.querySelectorAll('.sode__field-label[id]')) add(lab.id.replace(/-label$/, ''), lab.textContent, lab.closest('.sode__field') || lab);
  for (const chk of sec.querySelectorAll('.pb-check')) add(chk.querySelector('input')?.id, chk.querySelector('.pb-check__label')?.textContent, chk);
  for (const f of sec.querySelectorAll('[data-find]')) add(f.dataset.find, f.dataset.label || f.textContent, f);
  return [...out.values()];
}
const OUTSIDE = [{ secId: 'history', id: 'open-log', label: 'View Logs', node: $('open-log') }];
const makeIndex = () =>
  buildIndex(
    sections.map((sec, i) => {
      const settings = settingsOf(sec, secIds[i]);
      for (const x of OUTSIDE.filter((o) => o.secId === secIds[i])) {
        settings.push({ id: x.id, label: x.label });
        targets.set(`${x.secId}/${x.id}`, x.node);
      }
      return { id: secIds[i], title: sec.querySelector('legend').textContent, settings };
    }),
  );
let settingsIndex = makeIndex();
indexNow = () => {
  settingsIndex = makeIndex();
};

let hits = [];
let cur = 0;
let flashTimer = 0;
let flashed = null;

function unflash() {
  clearTimeout(flashTimer);
  if (!flashed) return;
  flashed.node.classList.remove('sode__is-found');
  flashed = null;
}
function flash(node) {
  unflash();
  node.classList.add('sode__is-found');
  flashed = { node };
  flashTimer = setTimeout(unflash, 2500);
}

function showHits() {
  const q = findBox.value.trim();
  hits = q ? find(settingsIndex, q) : [];
  cur = 0;
  foundList.replaceChildren(
    ...hits.map((h, i) => {
      const li = el('li', null, el('span', null, h.label), el('span', 'sode__hint', h.kind === 'section' ? 'Section' : 'in ' + h.section));
      li.id = `settings-found-${i}`;
      li.setAttribute('role', 'option');
      li.addEventListener('mousedown', (e) => e.preventDefault());
      li.addEventListener('click', () => pickHit(h));
      return li;
    }),
  );
  foundList.hidden = !hits.length;
  $('settings-find-none').hidden = !q || hits.length > 0;
  findBox.setAttribute('aria-expanded', String(hits.length > 0));
  markHit();
}
function markHit() {
  [...foundList.children].forEach((li, i) => li.setAttribute('aria-selected', String(i === cur)));
  if (hits.length) findBox.setAttribute('aria-activedescendant', `settings-found-${cur}`);
  else findBox.removeAttribute('aria-activedescendant');
  foundList.children[cur]?.scrollIntoView({ block: 'nearest' });
}
function pickHit(h) {
  const i = secIds.indexOf(h.sectionId);
  const node = h.kind === 'section' ? sections[i] : targets.get(`${h.sectionId}/${h.id}`);
  if (!node) return;
  if (h.kind === 'setting' && /^cfg-pin(2|-clear)?$/.test(h.id)) showPinFields(true);
  if (phone()) showPage(i);
  findBox.value = '';
  showHits();
  tocApi.pick(i);
  node.scrollIntoView({
    behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
    block: h.kind === 'section' ? 'start' : 'center',
  });
  flash(node);
  if (h.kind === 'setting') document.getElementById(h.id)?.focus({ preventScroll: true });
  else findBox.blur();
}

// Typing here isn't editing the settings: the form must not see it (it would dismiss the
// unsaved-changes warning).
for (const ev of ['input', 'change'])
  findBox.addEventListener(ev, (e) => {
    e.stopPropagation();
    if (ev === 'input') showHits();
  });
findBox.addEventListener('blur', () => {
  foundList.hidden = true;
  findBox.setAttribute('aria-expanded', 'false');
});
findBox.addEventListener('focus', () => {
  if (hits.length) {
    foundList.hidden = false;
    findBox.setAttribute('aria-expanded', 'true');
  }
});
findBox.addEventListener('keydown', (e) => {
  if (e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    if (!hits.length) return;
    e.preventDefault();
    cur = (cur + (e.key === 'ArrowDown' ? 1 : hits.length - 1)) % hits.length;
    markHit();
  } else if (e.key === 'Enter') {
    e.preventDefault();
    if (hits[cur]) pickHit(hits[cur]);
  } else if (e.key === 'Escape' && findBox.value) {
    e.preventDefault();
    e.stopPropagation();
    findBox.value = '';
    showHits();
  }
});
