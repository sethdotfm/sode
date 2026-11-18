// Settings form fields: fill from S.cfg, read back for the `config` message, check before save, and the live notes and
// previews beside the fields. data-apart sections (users.js, apisettings.js, configfile.js) own their own fields.

import { $, el, S, wsSend } from './core.js';
import { levelColor, levelLabel, levelInput } from './levelview.js';
import { fmtTime } from './device.js';
import { parseLevelsAs } from './levels.js';
import { resolve, toGlobal } from './prefs.js';
import { iconURL, setBrand } from './brandicon.js';
import { parseFilter, compileFilter } from './filter.js';
import { omitOwned } from './owned.js';
import { intProblem, namesTimeText } from './settingsrules.js';

const root = document.documentElement;
export const form = $('settings-view');
export const isOpen = () => root.dataset.view === 'settings';
const fields = () => form.querySelectorAll('[data-key]');

const formMode = () => form.querySelector('input[name="lvlnum"]:checked')?.value || '1';
const formLevels = () => Number($('cfg-levels').value);
let shownMode = '1';

// What the form showed: only a change from it is sent, so a form opened before someone else changed the locks cannot
// undo their change.
let shownAccess = null;

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

export function fillForm() {
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
  $('cfg-lock-panel').checked = !!S.cfg.locks?.panel;
  shownAccess = { main: !!S.cfg.locks?.main, log: !!S.cfg.locks?.log, panel: !!S.cfg.locks?.panel, auto: S.cfg.autoLockMinutes ?? 0 };
}

// After renderPrefs (devicesettings.js), which draws the This browser section.
export function fillGlobalDefaults() {
  for (const s of form.querySelectorAll('select[data-gpref]')) s.value = resolve(s.dataset.gpref, null, null, S.cfg.browserDefaults);
}

export function readSettings() {
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
  const locks = { main: $('cfg-lock-main').checked, log: $('cfg-lock-log').checked, panel: $('cfg-lock-panel').checked };
  if (shownAccess && locks.main === shownAccess.main && locks.log === shownAccess.log && locks.panel === shownAccess.panel) delete c.locks;
  else c.locks = locks;
  if (shownAccess && c.autoLockMinutes === shownAccess.auto) delete c.autoLockMinutes;
  c.levelColors = [...$('cfg-colors').querySelectorAll('input')].map((f) => f.value.trim());
  c.levelNumbering = formMode();
  c.browserDefaults = Object.fromEntries([...form.querySelectorAll('select[data-gpref]')].map((s) => [s.dataset.gpref, toGlobal(s.dataset.gpref, s.value)]));

  const def = $('cfg-default').value.trim();
  const list = def ? parseLevelsAs(def, c.levelNumbering, formLevels()) : [];
  c.defaultLevels = list ? levelInput(list, '1') : null;
  return c;
}

// The first field readSettings' result cannot be saved with: {text, node}, or null. The PIN fields are the save path's.
export function formProblem(c) {
  const badColor = [...$('cfg-colors').querySelectorAll('input')].find((f) => f.value.trim() && !HEX.test(f.value.trim()));
  if (badColor) return { text: `${badColor.getAttribute('aria-label')} must be hex, like #3a7bd5.`, node: badColor };
  if (c.defaultLevels === null) return { text: `Default levels: "${$('cfg-default').value.trim()}" is not a level or range here.`, node: $('cfg-default') };
  const hidden = parseFilter(c.hiddenDsts);
  if (hidden.error) return { text: `Globally hidden destinations: ${hidden.error}.`, node: $('cfg-hidden') };
  const bad = [...fields()].find((f) => intProblem([{ key: f.dataset.key, type: f.dataset.type, value: f.value }]));
  if (bad) return { text: `${form.querySelector(`label[for="${bad.id}"]`)?.textContent || 'A number field'} must be a whole number.`, node: bad };
  return null;
}

function renderNumbering() {
  const n = formLevels();
  const span = (mode) => (Number.isInteger(n) && n > 1 ? `${levelLabel(1, mode)}–${levelLabel(n, mode)}` : levelLabel(1, mode));
  form.querySelector('label[for="cfg-lvlnum-0"]').textContent = span('0');
  form.querySelector('label[for="cfg-lvlnum-1"]').textContent = span('1');
  form.querySelector('label[for="cfg-lvlnum-va"]').textContent = Number.isInteger(n) && n > 1 ? `V + ${levelLabel(2, 'va')}–${levelLabel(n, 'va')}` : 'V';
}

// Rewrite the default levels box in the new numbering (if it parsed in the old one) and relabel
// previews. Not limited to the level count, since it is possible that the count is mid-edit:
// Save checks that.
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

export function renderOffsetNote() {
  const off = Number($('cfg-offset').value);
  $('cfg-offset-note').textContent = Number.isInteger(off) ? `Level ${levelLabel(1, formMode())} = level ${1 + off} on the router` : '';
}

export function renderHiddenNote() {
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

export function renderNamesStatus() {
  if (!isOpen()) return;
  $('names-at').textContent = namesTimeText({ at: S.namesAt, savedAt: S.namesSavedAt }, fmtTime);
  $('names-status').textContent = `${S.srcs.length} sources, ${S.dsts.length} destinations`;
  const btn = $('names-refresh');
  btn.disabled = S.router !== 'up';
  btn.textContent = 'Refresh now';
}

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
$('names-refresh').addEventListener('click', () => {
  if (wsSend({ t: 'rereadNames' })) {
    $('names-refresh').disabled = true;
    $('names-refresh').textContent = 'Refreshing…';
  }
});
