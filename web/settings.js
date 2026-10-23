// Settings view, shared by both layouts. Lives at #settings so Back and
// reload work. Server settings save together; the layout choice is per
// browser and applies at once. With a settings PIN, the view opens locked
// and locks again when it closes.

import { $, el, S, on, wsSend, fmtTime, clockPref, scrollTopPref, setScrollTopPref, showResult, levelColor, levelLabel, levelInput, parseLevelsAs } from './core.js';
import { layout, layoutPref, setLayoutPref } from './layout.js';

const root = document.documentElement;
const form = $('settings-view');
const fields = () => form.querySelectorAll('[data-key]');
const isOpen = () => root.dataset.view === 'settings';
// The PIN was accepted on this connection. Only means anything while a PIN
// is set: with none, the form is open anyway.
let unlocked = false;
// A save is in flight: its own hello / locked arrive before configResult
// closes the view, so don't flip to the PIN prompt for them.
const saving = () => $('settings-save').disabled;
// Numbers each unlock sent; opening or closing the view skips ahead, so a
// reply to one sent before then is ignored.
let unlockSeq = 0;

// The level numbering picked in the form, and how the default levels box
// is currently written (so switching modes can rewrite it).
const formMode = () => form.querySelector('input[name="lvlnum"]:checked')?.value || '1';
const formLevels = () => Number($('cfg-levels').value);
let shownMode = '1';

function showView() {
  const want = location.hash === '#settings' && S.cfg;
  if (want && !isOpen()) openSettings();
  else if (!want && isOpen()) closeSettings();
}

export function openSettings() {
  if (!S.cfg) return;
  if (location.hash !== '#settings') history.pushState(null, '', '#settings');
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
  renderColors(S.cfg.levelColors || []);
  $('cfg-pin').value = '';
  $('cfg-pin2').value = '';
  $('cfg-pin-clear').checked = false;
  renderPinStatus();
  unlocked = false;
  unlockSeq++;
  $('lock-pin').value = '';
  $('lock-unlock').disabled = false;
  $('lock-err').hidden = true;
  $('cfg-layout').value = layoutPref();
  $('cfg-clock').value = clockPref();
  $('cfg-scrolltop').checked = scrollTopPref();
  resetClear();
  $('settings-err').hidden = true;
  $('settings-save').disabled = false;
  $('settings-path').replaceChildren(...(S.configPath ? ['Saved to ', el('code', null, S.configPath)] : ['Not saved: ', el('code', null, 'SODE_CONFIG'), ' unset']));
  root.dataset.view = 'settings'; // before renderNamesStatus, which only draws when open
  renderOffsetNote();
  renderNamesStatus();
  window.scrollTo(0, 0);
  renderLock(true);
}

// Shows the PIN prompt or the form. Focus moves only when that flips (or
// on open), not on every status update.
function renderLock(opening = false) {
  if (!isOpen()) return;
  const locked = S.pinSet && !unlocked;
  const flipped = locked !== ('locked' in form.dataset);
  if (locked) form.dataset.locked = '';
  else delete form.dataset.locked;
  if (!opening && !flipped) return;
  if (locked) $('lock-pin').focus();
  else if (layout() === 'desktop') $('cfg-namerefresh').focus();
}

function closeSettings() {
  if (location.hash === '#settings') history.replaceState(null, '', location.pathname + location.search);
  delete root.dataset.view;
  wsSend({ t: 'lock' });
  unlocked = false;
  unlockSeq++;
  if (layout() === 'desktop') $('src').focus();
}

function readSettings() {
  const c = { ...S.cfg };
  for (const f of fields()) {
    const k = f.dataset.key;
    if (f.dataset.type === 'bool') c[k] = f.checked;
    else if (f.dataset.type === 'list') c[k] = f.value.split(',').map((x) => x.trim()).filter(Boolean);
    else if (f.dataset.type === 'int') c[k] = f.value.trim() === '' ? NaN : Number(f.value);
    else c[k] = f.value.trim();
  }
  c.levelColors = [...$('cfg-colors').querySelectorAll('input')].map((f) => f.value.trim());
  c.levelNumbering = formMode();
  // Default levels are typed in the form's numbering but saved 1-based.
  const def = $('cfg-default').value.trim();
  const list = def ? parseLevelsAs(def, c.levelNumbering, formLevels()) : [];
  c.defaultLevels = list ? levelInput(list, '1') : null;
  return c;
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
    if (isOpen()) closeSettings();
    showResult('ok', 'Settings saved', S.configPath ? `Written to ${S.configPath}.` : 'Applied (not saved to disk).');
    return;
  }
  if (m.locked) {
    unlocked = false;
    showLockErr(m.err);
    return;
  }
  $('settings-err').textContent = sentence(m.err);
  $('settings-err').hidden = false;
}

// Server errors are Go-style (lowercase, no full stop); shown as sentences.
const sentence = (t) => {
  const s = String(t || '').trim();
  return s && s[0].toUpperCase() + s.slice(1) + (/[.!?…]$/.test(s) ? '' : '.');
};

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

function unlockResult(m) {
  if (m.id !== unlockSeq) return; // sent before the view closed or reopened
  $('lock-unlock').disabled = false;
  if (!isOpen()) return;
  if (!m.ok) return showLockErr(m.err);
  unlocked = true;
  $('lock-pin').value = '';
  $('lock-err').hidden = true;
  renderLock();
}

// The server forgets an unlock when the socket drops, and only honors it
// for the PIN it was entered against: with no PIN now, a PIN set later
// (here or by someone else, even while disconnected) needs entering.
function relock() {
  if (S.wsState !== 'up') {
    unlocked = false;
    $('lock-unlock').disabled = false; // their results are lost
    $('settings-save').disabled = false;
  }
  if (!S.pinSet) unlocked = false;
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
$('settings-cancel').addEventListener('click', closeSettings);
$('lock-cancel').addEventListener('click', closeSettings);
$('cfg-offset').addEventListener('input', renderOffsetNote);
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
$('cfg-layout').addEventListener('change', (e) => setLayoutPref(e.target.value));
$('cfg-scrolltop').addEventListener('change', (e) => setScrollTopPref(e.target.checked));
$('names-refresh').addEventListener('click', () => {
  if (wsSend({ t: 'refreshNames' })) {
    $('names-refresh').disabled = true;
    $('names-refresh').textContent = 'Refreshing…';
  }
});
form.addEventListener('submit', (e) => {
  e.preventDefault();
  if ('locked' in form.dataset) { // Enter or Unlock in the PIN prompt
    if (!wsSend({ t: 'unlock', pin: $('lock-pin').value, id: ++unlockSeq })) return showLockErr('Not connected to Sode.');
    $('lock-unlock').disabled = true;
    return;
  }
  const c = readSettings();
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
  if (isOpen() && e.key === 'Escape') {
    e.preventDefault();
    closeSettings();
  }
});
window.addEventListener('popstate', showView);

on('hello', () => { showView(); relock(); });
on('names', renderNamesStatus);
on('namesChecked', renderNamesStatus);
on('conn', () => { renderNamesStatus(); relock(); });
on('unlockResult', unlockResult);
on('locked', () => {
  unlocked = false;
  if (isOpen() && !saving()) showLockErr('Settings are locked; enter the PIN again.');
});
on('configResult', configResult);
