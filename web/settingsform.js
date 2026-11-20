// Settings form fields: fill from S.cfg, read back for the one Save, check before save, and the live notes and
// previews beside the fields. settingssave.js wraps the form as formSource, a save source of the one Save.

import { $, el, S, icon } from './core.js';
import { levelColor, levelLabel, levelInput } from './levelview.js';
import { fmtTime } from './device.js';
import { parseLevelsAs } from './levels.js';
import { resolve, toGlobal } from './prefs.js';
import { iconURL, setBrand } from './brandicon.js';
import { parseFilter, compileFilter } from './filter.js';
import { intProblem, namesTimeText } from './settingsrules.js';
import { rereadNames } from './rereadnames.js';

const root = document.documentElement;
export const form = $('settings-view');
export const isOpen = () => root.dataset.view === 'settings';
const fields = () => form.querySelectorAll('[data-key]');

const formMode = () => form.querySelector('input[name="lvlnum"]:checked')?.value || '1';
const formLevels = () => Number($('cfg-levels').value);
let shownMode = '1';

// A computed rgb() color as #rrggbb.
const rgbHex = (css) =>
  '#' +
  (css.match(/\d+/g) || [])
    .slice(0, 3)
    .map((c) => Number(c).toString(16).padStart(2, '0'))
    .join('');

// brandBorder is a color name or a custom #rrggbb; the select shows "custom" for the second and the color input holds it.
const brandValue = () => ($('cfg-brandborder').value === 'custom' ? $('cfg-brandcustom').value : $('cfg-brandborder').value);

// Previewed before save: window border and the app icon the server draws (icon.go).
function renderBrandPreview() {
  const c = brandValue();
  $('cfg-brandcustom').hidden = $('cfg-brandborder').value !== 'custom';
  setBrand($('brand-swatch'), c);
  $('brand-icon').src = iconURL(180, c);
}
function fillBrandColors(v) {
  const sel = $('cfg-brandborder');
  const opt = (value, text) => Object.assign(el('option', null, text), { value });
  sel.replaceChildren(opt('', 'Off'), ...S.brandColors.map((c) => opt(c.name, c.label)), opt('custom', 'Custom…'));
  const custom = (v || '').startsWith('#');
  sel.value = custom ? 'custom' : v || '';
  if (custom) $('cfg-brandcustom').value = v;
}
$('cfg-brandborder').addEventListener('change', () => {
  const custom = $('cfg-brandborder').value === 'custom';
  // Custom starts from the color shown before, not the input's black.
  const was = rgbHex(getComputedStyle($('brand-swatch')).borderTopColor);
  if (custom && /^#[0-9a-f]{6}$/.test(was)) $('cfg-brandcustom').value = was;
  renderBrandPreview();
  if (!custom) return;
  try {
    $('cfg-brandcustom').showPicker();
  } catch {
    $('cfg-brandcustom').focus();
  }
});
$('cfg-brandcustom').addEventListener('input', renderBrandPreview);

export function fillForm() {
  for (const f of fields()) {
    const v = S.cfg[f.dataset.key];
    if (f.dataset.type === 'bool') f.checked = !!v;
    else if (f.dataset.type === 'list') f.value = (v || []).join(', ');
    else f.value = v ?? '';
  }
  fillBrandColors(S.cfg.brandBorder);
  $(`cfg-lvlnum-${S.cfg.levelNumbering || '1'}`).checked = true;
  shownMode = formMode();
  $('cfg-default').value = levelInput(parseLevelsAs(S.cfg.defaultLevels, '1', S.cfg.levels) || [], shownMode);
  renderNumbering();
  renderBrandPreview();
  renderColors(S.cfg.levelColors || []);
  for (const page of ['main', 'log', 'panel']) $(`cfg-lock-${page}`).checked = !!S.cfg.locks?.[page];
  $('cfg-new-persistent').checked = !!S.cfg.newUserDefaults?.persistent;
  $('cfg-new-no-auto-lock').checked = !!S.cfg.newUserDefaults?.noAutoLock;
}

// After renderPrefs (devicesettings.js), which draws the This browser section.
export function fillGlobalDefaults() {
  for (const s of form.querySelectorAll('select[data-gpref]')) s.value = resolve(s.dataset.gpref, null, null, S.cfg.browserDefaults);
}

// The form's one Save fields as the page reads them. The review compares two reads, so a value the form shows
// differently from S.cfg (blank level colors, a rewritten default levels box) is no change.
export function readSettings() {
  const c = {};
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
  if (Number.isNaN(c.autoLockMinutes)) c.autoLockMinutes = 0;
  c.brandBorder = brandValue();
  c.locks = { main: $('cfg-lock-main').checked, log: $('cfg-lock-log').checked, panel: $('cfg-lock-panel').checked };
  c.newUserDefaults = { persistent: $('cfg-new-persistent').checked, noAutoLock: $('cfg-new-no-auto-lock').checked };
  c.levelColors = [...$('cfg-colors').querySelectorAll('input.pb-input')].map((f) => f.value.trim());
  c.levelNumbering = formMode();
  c.browserDefaults = Object.fromEntries([...form.querySelectorAll('select[data-gpref]')].map((s) => [s.dataset.gpref, toGlobal(s.dataset.gpref, s.value)]));

  const def = $('cfg-default').value.trim();
  const list = def ? parseLevelsAs(def, c.levelNumbering, formLevels()) : [];
  c.defaultLevels = list ? levelInput(list, '1') : null;
  return c;
}

// The first field readSettings' result cannot be saved with: {text, node}, or null. The PIN fields are the save path's.
export function formProblem(c) {
  const badColor = [...$('cfg-colors').querySelectorAll('input.pb-input')].find((f) => f.value.trim() && !HEX.test(f.value.trim()));
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

// Not limited to the level count, since the count can be mid-edit: Save checks it.
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
  const hex = rgbHex(getComputedStyle(probe).color);
  probe.remove();
  return hex;
}

function preview(slot, ui, v) {
  levelColor(slot, { swatch: swatchOf(ui), color: HEX.test(v.trim()) ? toHex(v) : S.levelDefaults[ui - 1] || '' });
}

// The native picker for a hex box. The color input stays in layout (invisible) so the picker opens at the button.
function colorPick(f) {
  const pick = el('input', { type: 'color', class: 'sode__colorpick-under', tabIndex: -1, 'aria-hidden': 'true' });
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon('palette'));
  b.type = 'button';
  b.dataset.pbTip = 'Pick a color';
  b.setAttribute('aria-label', `Pick ${f.getAttribute('aria-label').toLowerCase()}`);
  b.addEventListener('click', () => {
    const v = f.value.trim() || f.placeholder;
    pick.value = HEX.test(v) ? toHex(v) : '#000000';
    try {
      pick.showPicker();
    } catch {
      pick.click();
    }
  });
  pick.addEventListener('input', () => {
    f.value = pick.value;
    f.dispatchEvent(new Event('input'));
  });
  return el('span', 'sode__colorpick-wrap', b, pick);
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
    rows.push(el('div', 'sode__color', slot, f, colorPick(f)));
  }
  $('cfg-colors').replaceChildren(...rows);
}

function rerenderColors() {
  const vals = [...$('cfg-colors').querySelectorAll('input.pb-input')].map((f) => f.value);
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
  btn.textContent = 'Re-read now';
}

$('cfg-offset').addEventListener('input', renderOffsetNote);
$('cfg-hidden').addEventListener('input', renderHiddenNote);
$('cfg-colors-reset').addEventListener('click', () => {
  for (const f of $('cfg-colors').querySelectorAll('input.pb-input')) {
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
  if (rereadNames()) {
    $('names-refresh').disabled = true;
    $('names-refresh').textContent = 'Re-reading…';
  }
});
