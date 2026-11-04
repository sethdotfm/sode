// Settings view, shared by both layouts. Lives at #settings so Back and
// reload work. Server settings save together, and the view stays open
// after a save; the This device choices are per browser and apply at once.
// Leaving with unsaved changes warns first. With a settings PIN, the view
// opens locked and locks again when it closes.

import { $, el, S, on, emit, wsSend, sentence } from './core.js';
import { levelColor, levelLabel, levelInput } from './levelview.js';
import { fmtTime, ownPref, globalPref, setPref } from './device.js';
import { filterSummary, normFilter } from './filterstore.js';
import { showResult } from './plans.js';
import { parseLevelsAs } from './levels.js';
import { PREFS, resolve, toGlobal } from './prefs.js';
import { iconURL, setBrand } from './brandicon.js';
import { layout } from './layout.js';
import { parseFilter, compileFilter } from './filter.js';
import { unlock, relock, unlocked, trusted, needsPin, demandPin } from './pin.js';
import { guardedEdited, loginOffered } from './lockrules.js';
import { omitOwned } from './owned.js';
import { buildIndex, find, slug } from './settingsindex.js';
import './users.js'; // adds its section before the table of contents is built

const root = document.documentElement;
const form = $('settings-view');
const fields = () => form.querySelectorAll('[data-key]');
const isOpen = () => root.dataset.view === 'settings';
// A save is in flight: its own hello / locked arrive before configResult
// closes the view, so don't flip to the PIN prompt for them.
const saving = () => $('settings-save').disabled;

// The level numbering picked in the form, and how the default levels box
// is currently written (so switching modes can rewrite it).
const formMode = () => form.querySelector('input[name="lvlnum"]:checked')?.value || '1';
const formLevels = () => Number($('cfg-levels').value);
let shownMode = '1';

function showView() {
  const want = location.hash === '#settings' && S.cfg;
  if (want && !isOpen()) openSettings();
  else if (!want && isOpen()) {
    // Back with unsaved changes: stay, and warn; Back again leaves.
    if (warnUnsaved()) history.pushState(null, '', '#settings');
    else closeSettings();
  }
}

// ---- unsaved changes: the server settings in the form against the last
// opened or saved state. This device applies at once, so it isn't counted.

let clean = '';
let savedAt = null;
const formState = () => JSON.stringify([...form.querySelectorAll('input, select, textarea')]
  .filter((f) => !f.closest('.sode__device, .sode__lock, .sode__users') && f.id !== 'settings-find')
  .map((f) => (f.type === 'checkbox' || f.type === 'radio' ? f.checked : f.value)));
const dirty = () => isOpen() && formState() !== clean;
const warned = () => !$('settings-unsaved').hidden;

function markClean(saved) {
  clean = formState();
  savedAt = saved ? new Date().toISOString() : null;
  unwarn();
}

// The first try at leaving with unsaved changes shows the warning and
// returns true; the next one goes.
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

// Next to Save: unsaved changes, or where (and when) it was saved.
function renderSaveState() {
  const path = S.configPath ? el('code', null, S.configPath) : null;
  // The warning, once shown, says it already.
  $('settings-path').replaceChildren(...(dirty() ? (warned() ? [] : ['Unsaved changes'])
    : !path ? [savedAt ? `Applied ${fmtTime(savedAt)}; not saved: ` : 'Not saved: ', el('code', null, 'SODE_CONFIG'), ' unset']
    : savedAt ? [`Saved ${fmtTime(savedAt)} to `, path]
    : ['Saved to ', path]));
}

// The picked brand colour, as the window border and as the app icon the
// server draws (icon.go), before it is saved.
function renderBrandPreview() {
  const sel = $('cfg-brandborder');
  const c = sel.value;
  setBrand($('brand-swatch'), c);
  $('brand-icon').src = iconURL(180, c);
  $('brand-name').textContent = sel.selectedOptions[0]?.textContent || 'Off';
}
// The colours come from the server's table (hello), Off first.
function fillBrandColors() {
  const sel = $('cfg-brandborder');
  const opt = (value, text) => Object.assign(el('option', null, text), { value });
  sel.replaceChildren(opt('', 'Off'), ...S.brandColors.map((c) => opt(c.name, c.label)));
}
$('cfg-brandborder').addEventListener('change', renderBrandPreview);

export function openSettings() {
  if (!S.cfg) return;
  // Locked or not: the version shows at the bottom of both views.
  for (const v of form.querySelectorAll('[data-version]')) v.textContent = S.version ? `Sode ${S.version}` : '';
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
  $('cfg-pin').value = '';
  $('cfg-pin2').value = '';
  $('cfg-pin-clear').checked = false;
  renderPinStatus();
  relock(); // opens locked; a reply to an unlock sent before is dropped
  $('lock-pin').value = '';
  $('lock-unlock').disabled = false;
  $('lock-err').hidden = true;
  renderPrefs();
  for (const s of form.querySelectorAll('select[data-gpref]')) s.value = resolve(s.dataset.gpref, null, S.cfg.deviceDefaults);
  resetClear();
  $('settings-err').hidden = true;
  $('settings-save').disabled = false;
  root.dataset.view = 'settings'; // before renderNamesStatus, which only draws when open
  renderOffsetNote();
  renderHiddenNote();
  renderGlobalSets();
  renderNamesStatus();
  window.scrollTo(0, 0);
  markSection();
  markClean(false);
  renderLock(true);
}

// Shows the PIN prompt or the form. Focus moves only when that flips (or
// on open), not on every status update.
function renderLock(opening = false) {
  if (!isOpen()) return;
  const locked = needsPin();
  const flipped = locked !== ('locked' in form.dataset);
  if (locked) form.dataset.locked = '';
  else { delete form.dataset.locked; $('lock-help').hidden = true; }
  if (!opening && !flipped) return;
  if (locked) $('lock-pin').focus();
  else if (layout() === 'desktop') $('cfg-namerefresh').focus();
}

function closeSettings() {
  if (location.hash === '#settings') history.replaceState(null, '', location.pathname + location.search);
  delete root.dataset.view;
  $('lock-help').hidden = true;
  relock(true);
  if (layout() === 'desktop') $('src').focus();
}

// The config as the form now stands, without what the server owns (owned.js).
function readSettings() {
  const c = omitOwned(S.cfg);
  for (const f of fields()) {
    const k = f.dataset.key;
    if (f.dataset.type === 'bool') c[k] = f.checked;
    else if (f.dataset.type === 'list') c[k] = f.value.split(',').map((x) => x.trim()).filter(Boolean);
    else if (f.dataset.type === 'int') c[k] = f.value.trim() === '' ? NaN : Number(f.value);
    else c[k] = f.value.trim();
  }
  c.levelColors = [...$('cfg-colors').querySelectorAll('input')].map((f) => f.value.trim());
  c.levelNumbering = formMode();
  c.deviceDefaults = Object.fromEntries([...form.querySelectorAll('select[data-gpref]')].map((s) => [s.dataset.gpref, toGlobal(s.dataset.gpref, s.value)]));

  // Default levels are typed in the form's numbering but saved 1-based.
  const def = $('cfg-default').value.trim();
  const list = def ? parseLevelsAs(def, c.levelNumbering, formLevels()) : [];
  c.defaultLevels = list ? levelInput(list, '1') : null;
  return c;
}

// ---- This device and Defaults for every device: the same six choices
// (prefs.js). A device one starts with "Global (value)", which follows the
// default; it applies at once. A default one is saved with the form.

const prefOptions = (name, globalText) => {
  const opt = (value, text) => Object.assign(el('option', null, text), { value });
  return [
    ...(globalText ? [opt('', globalText)] : []),
    ...Object.entries(PREFS[name].values).map(([v, text]) => opt(v, text)),
  ];
};

for (const s of form.querySelectorAll('select[data-gpref]')) s.replaceChildren(...prefOptions(s.dataset.gpref));

// On any change of the settings in force, so "Global (value)" stays true
// when the defaults change under an open form, and Alt+Shift+T shows.
function renderPrefs() {
  for (const s of form.querySelectorAll('select[data-pref]')) {
    const name = s.dataset.pref;
    s.replaceChildren(...prefOptions(name, `Global (${PREFS[name].values[globalPref(name)]})`));
    s.value = ownPref(name);
  }
}
renderPrefs();
on('prefs', renderPrefs);

// Account: only while logged in as a user (lock.js says so with 'access').
on('access', (L) => {
  $('account').hidden = !L.user && !loginOffered(L);
  $('account-name').textContent = L.user ? `Logged in as ${L.user}` : 'Not logged in';
  for (const id of ['account-pin', 'account-logout']) $(id).hidden = !L.user;
  $('account-login').hidden = !!L.user;
  $('soft-lock').hidden = L.enforced; // a PIN lock has its own Lock and Log out
});
$('soft-lock-btn').addEventListener('click', () => emit('softLock'));
$('account-pin').addEventListener('click', () => emit('changePin'));
$('account-logout').addEventListener('click', () => emit('logout'));
$('account-login').addEventListener('click', () => emit('login'));

// ---- table of contents: one button per section, scrolling to it, with the
// section in view marked. Wide screens pin it beside the column; narrow
// ones show it as a strip across the top; the phone layout leaves it out.

const sections = [...form.querySelectorAll(':scope > fieldset.sode__group')];
const toc = $('settings-toc');
// A clicked section stays marked while the page scrolls to it, which may
// stop short of it near the bottom, until the scroll ends or the user
// scrolls themselves.
let picked = -1;
toc.replaceChildren(...sections.map((sec, i) => {
  const b = el('button', 'pb-tab', sec.querySelector('legend').textContent);
  b.type = 'button';
  b.addEventListener('click', () => {
    picked = i;
    mark(i);
    sec.scrollIntoView({ behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' });
  });
  return b;
}));
function mark(cur) {
  [...toc.children].forEach((b, i) => {
    b.classList.toggle('active', i === cur);
    if (i === cur) b.setAttribute('aria-current', 'location');
    else b.removeAttribute('aria-current');
  });
}
let tocTick = false;
function markSection() {
  tocTick = false;
  if (!isOpen() || picked >= 0) return;
  // The last section whose top has reached the top of the window (where a
  // click scrolls it to, its scroll margin down); at the very bottom the
  // last one, which may be too short to ever reach it.
  const atEnd = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2;
  const line = (s) => parseFloat(getComputedStyle(s).scrollMarginTop) + 8;
  mark(Math.max(atEnd ? sections.length - 1 : sections.findLastIndex((s) => s.getBoundingClientRect().top <= line(s)), 0));
}
window.addEventListener('scroll', () => {
  if (tocTick) return;
  tocTick = true;
  requestAnimationFrame(markSection);
}, { passive: true });
for (const ev of ['scrollend', 'wheel', 'touchstart', 'keydown']) {
  window.addEventListener(ev, () => {
    if (picked < 0) return;
    picked = -1;
    if (ev !== 'scrollend') markSection();
  }, { passive: true });
}

// ---- level numbering: a three-way switch, labelled for the level count

function renderNumbering() {
  const n = formLevels();
  const span = (mode) => (Number.isInteger(n) && n > 1 ? `${levelLabel(1, mode)}–${levelLabel(n, mode)}` : levelLabel(1, mode));
  form.querySelector('label[for="cfg-lvlnum-0"]').textContent = span('0');
  form.querySelector('label[for="cfg-lvlnum-1"]').textContent = span('1');
  form.querySelector('label[for="cfg-lvlnum-va"]').textContent = Number.isInteger(n) && n > 1 ? `V + ${levelLabel(2, 'va')}–${levelLabel(n, 'va')}` : 'V';
}

// After the numbering changes: rewrite the default levels box in the new
// numbering (if it parsed in the old one) and relabel the previews. Not
// limited to the level count, which may be mid-edit: Save checks that.
function numberingChanged() {
  const f = $('cfg-default');
  const list = f.value.trim() && parseLevelsAs(f.value, shownMode, Infinity);
  if (list) f.value = levelInput(list, formMode());
  shownMode = formMode();
  renderNumbering();
  renderOffsetNote();
  rerenderColors();
}

// ---- level colors: one hex box per level, previewed on its slot

const HEX = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i;
const toHex = (v) => {
  const h = v.trim().replace(/^#/, '').toLowerCase();
  return '#' + (h.length === 3 ? [...h].map((c) => c + c).join('') : h);
};

const swatchOf = (ui) => (ui === 1 ? 'video' : String((ui - 2) % 16 + 1).padStart(2, '0'));

// A level's default hex: the server's default color, else its patchbay
// swatch in the current theme.
function defaultHex(ui) {
  if (S.levelDefaults[ui - 1]) return S.levelDefaults[ui - 1];
  const probe = el('span');
  probe.style.color = `var(--pb-signal-${swatchOf(ui)})`;
  document.body.append(probe);
  const rgb = getComputedStyle(probe).color.match(/\d+/g) || [];
  probe.remove();
  return '#' + rgb.slice(0, 3).map((c) => Number(c).toString(16).padStart(2, '0')).join('');
}

function preview(slot, ui, v) {
  levelColor(slot, { swatch: swatchOf(ui), color: HEX.test(v.trim()) ? toHex(v) : S.levelDefaults[ui - 1] || '' });
}

// vals: colors to show, by level; keeps what's typed when the count changes.
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

// Redraws the color rows, keeping what's typed.
function rerenderColors() {
  const vals = [...$('cfg-colors').querySelectorAll('input')].map((f) => f.value);
  renderColors([...vals, ...(S.cfg.levelColors || []).slice(vals.length)]);
}

function renderOffsetNote() {
  const off = Number($('cfg-offset').value);
  $('cfg-offset-note').textContent = Number.isInteger(off) ? `Level ${levelLabel(1, formMode())} = wire level ${1 + off}` : '';
}

// ---- filter sets for everyone: listed here, edited in Recent's Filters

function renderGlobalSets() {
  if (!isOpen()) return;
  const sets = S.cfg?.filterSets || [];
  $('cfg-sets').replaceChildren(...(sets.length
    ? sets.map((s) => el('li', 'sode__gset', el('strong', null, s.name), el('span', 'sode__hint', filterSummary(normFilter(s)))))
    : [el('li', 'sode__hint', 'None')]));
}

// What the globally hidden expression matches, against the router's names.
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
  const when = at && at.getFullYear() > 2000 ? `refreshed ${fmtTime(S.namesAt)}` : 'not loaded yet';
  $('names-status').textContent = `${S.srcs.length} sources, ${S.dsts.length} destinations, ${when}`;
  const btn = $('names-refresh');
  btn.disabled = S.router !== 'up';
  btn.textContent = 'Refresh now';
}

function configResult(m) {
  $('settings-save').disabled = false;
  if (m.ok) {
    if (!isOpen()) return;
    // Stay open. A new or removed PIN was sent once; a changed PIN has
    // relocked everyone, this view included.
    $('cfg-pin').value = '';
    $('cfg-pin2').value = '';
    $('cfg-pin-clear').checked = false;
    renderPinStatus();
    markClean(true);
    renderLock();
    return;
  }
  if (m.locked) {
    relock();
    demandPin();
    showLockErr(m.err);
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

function renderPinStatus() {
  $('cfg-pin-clear-row').hidden = !S.pinSet;
  $('cfg-pin-status').textContent = S.pinSet
    ? 'PIN set. Leave blank to keep it.'
    : 'No PIN. 4–64 characters.';
}

async function sendUnlock() {
  $('lock-unlock').disabled = true;
  const r = await unlock($('lock-pin').value);
  if (r.superseded) return;
  $('lock-unlock').disabled = false;
  if (r.stale || !isOpen()) return; // sent before the view closed or reopened
  if (!r.ok) return showLockErr(r.err);
  $('lock-pin').value = '';
  $('lock-err').hidden = true;
  renderLock();
}

// After a hello or a connection change (pin.js has dropped any unlock the
// server no longer honors): show the prompt or the form to match.
function syncLock() {
  if (S.wsState !== 'up') $('settings-save').disabled = false; // its result is lost
  if (isOpen()) renderPinStatus();
  if (!saving()) renderLock();
}

// Clearing history is two steps: ask, then a red button that does it.
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
$('clear-confirm').addEventListener('click', () => {
  if (!wsSend({ t: 'clearHistory' })) {
    $('settings-err').textContent = 'Not connected to Sode.';
    $('settings-err').hidden = false;
    return;
  }
  resetClear();
  showResult('ok', 'History cleared', 'Recent is empty for everyone.');
});

for (const id of ['settings-open', 'm-settings']) $(id).addEventListener('click', openSettings);
$('settings-exit').addEventListener('click', exitSettings);
$('settings-back').addEventListener('click', exitSettings); // phone: like Exit
$('lock-cancel').addEventListener('click', exitSettings);

// Forgot PIN: how to clear it, in a dialog like the shortcut list.
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
// Editing again after the warning takes it back, until the next try to leave.
for (const ev of ['input', 'change']) form.addEventListener(ev, () => (warned() ? unwarn() : renderSaveState()));
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
form.addEventListener('submit', (e) => {
  e.preventDefault();
  if ('locked' in form.dataset) return void sendUnlock(); // Enter or Unlock in the PIN prompt
  // A trusted user saves the rest without the PIN, but not a change to the
  // PIN itself: ask now instead of after a refusal.
  if (S.pinSet && trusted() && !unlocked() && ($('cfg-pin').value || $('cfg-pin-clear').checked)) {
    demandPin();
    renderLock();
    return;
  }
  const c = readSettings();
  // Also not the guards the admin keeps: router, trusted proxies, PIN tries.
  if (S.pinSet && trusted() && !unlocked() && guardedEdited(c, S.cfg)) {
    demandPin();
    renderLock();
    return;
  }
  const bad = [...fields()].find((f) => f.dataset.type === 'int' && !Number.isInteger(c[f.dataset.key]));
  const badColor = [...$('cfg-colors').querySelectorAll('input')].find((f) => f.value.trim() && !HEX.test(f.value.trim()));
  if (badColor) {
    $('settings-err').textContent = `${badColor.getAttribute('aria-label')} must be hex, like #3a7bd5.`;
    $('settings-err').hidden = false;
    badColor.focus();
    return;
  }
  if (c.defaultLevels === null) {
    $('settings-err').textContent = `Default levels: "${$('cfg-default').value.trim()}" isn't a level or range here.`;
    $('settings-err').hidden = false;
    $('cfg-default').focus();
    return;
  }
  const hidden = parseFilter(c.hiddenDsts);
  if (hidden.error) {
    $('settings-err').textContent = `Globally hidden destinations: ${hidden.error}.`;
    $('settings-err').hidden = false;
    $('cfg-hidden').focus();
    return;
  }
  if (bad) {
    $('settings-err').textContent = `${form.querySelector(`label[for="${bad.id}"]`)?.textContent || 'A number field'} must be a whole number.`;
    $('settings-err').hidden = false;
    bad.focus();
    return;
  }
  const pin = $('cfg-pin').value;
  if (pin && [...pin].length < 4) {
    $('settings-err').textContent = 'The settings PIN must be at least 4 characters.';
    $('settings-err').hidden = false;
    $('cfg-pin').focus();
    return;
  }
  if (pin !== $('cfg-pin2').value) {
    $('settings-err').textContent = 'The two PINs don\'t match.';
    $('settings-err').hidden = false;
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
    // Esc closes the dialog, not Settings; focus stays on Close.
    if (e.key === 'Escape') { e.preventDefault(); closeLockHelp(); }
    if (e.key === 'Tab') { e.preventDefault(); $('lock-help-close').focus(); }
    return;
  }
  if (isOpen() && e.key === 'Escape') {
    e.preventDefault();
    exitSettings();
  }
});
window.addEventListener('popstate', showView);

on('hello', () => { showView(); syncLock(); renderGlobalSets(); });
on('filters', renderGlobalSets);
on('names', renderNamesStatus);
on('namesChecked', renderNamesStatus);
on('conn', () => { renderNamesStatus(); syncLock(); });
on('access', () => { if (isOpen() && !saving()) renderLock(); });
on('locked', () => {
  if (isOpen() && !saving()) showLockErr('Settings are locked; enter the PIN again.');
});
on('configResult', configResult);

// ---- settings search: the box above the table of contents. Hits are the
// sections and the individual settings (settingsindex.js); picking one
// scrolls to it, focuses a control and outlines it with a "Found" tag for a
// moment. Esc clears the box first; a second Esc leaves Settings as ever.

const findBox = $('settings-find');
const foundList = $('settings-found');
const secIds = sections.map((sec) => slug(sec.querySelector('legend').textContent));
const targets = new Map(); // `${section}/${setting}` -> the element to scroll to

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
const settingsIndex = buildIndex(sections.map((sec, i) => ({
  id: secIds[i], title: sec.querySelector('legend').textContent, settings: settingsOf(sec, secIds[i]),
})));

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
  foundList.replaceChildren(...hits.map((h, i) => {
    const li = el('li', null, el('span', null, h.label), el('span', 'sode__hint', h.kind === 'section' ? 'Section' : 'in ' + h.section));
    li.id = `settings-found-${i}`;
    li.setAttribute('role', 'option');
    li.addEventListener('mousedown', (e) => e.preventDefault()); // keep focus in the box
    li.addEventListener('click', () => pickHit(h));
    return li;
  }));
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
  findBox.value = '';
  showHits();
  picked = i;
  mark(i);
  node.scrollIntoView({ behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: h.kind === 'section' ? 'start' : 'center' });
  flash(node);
  if (h.kind === 'setting') document.getElementById(h.id)?.focus({ preventScroll: true });
  else findBox.blur();
}

// Typing here isn't editing the settings: the form must not see it (it would
// dismiss the unsaved-changes warning).
for (const ev of ['input', 'change']) findBox.addEventListener(ev, (e) => { e.stopPropagation(); if (ev === 'input') showHits(); });
findBox.addEventListener('blur', () => { foundList.hidden = true; findBox.setAttribute('aria-expanded', 'false'); });
findBox.addEventListener('focus', () => { if (hits.length) { foundList.hidden = false; findBox.setAttribute('aria-expanded', 'true'); } });
findBox.addEventListener('keydown', (e) => {
  if (e.ctrlKey || e.metaKey || e.altKey) return; // Sode's shortcuts stay theirs
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    if (!hits.length) return;
    e.preventDefault();
    cur = (cur + (e.key === 'ArrowDown' ? 1 : hits.length - 1)) % hits.length;
    markHit();
  } else if (e.key === 'Enter') {
    e.preventDefault(); // not a form submit (Save)
    if (hits[cur]) pickHit(hits[cur]);
  } else if (e.key === 'Escape' && findBox.value) {
    e.preventDefault();
    e.stopPropagation(); // the first Esc only clears; the next leaves Settings
    findBox.value = '';
    showHits();
  }
});
