// Settings form fields: fill from S.cfg, read back for the one Save, check before save, and the live notes and
// previews beside the fields. settingssave.js wraps the form as formSource, a save source of the one Save.

import { $, el, S, icon, routerData, labelText } from './core.js';
import { levelColor } from './levelview.js';
import { resolve, toGlobal } from './prefs.js';
import { iconURL, setBrand } from './brandicon.js';
import { brandCaution, pickerColors, CAUTION_NOTE } from './accent.js';
import { parseFilter, compileFilter } from './filter.js';
import { intProblem, positionTip } from './settingsrules.js';

const root = document.documentElement;
export const form = $('settings-view');
export const isOpen = () => root.dataset.view === 'settings';
const fields = () => form.querySelectorAll('[data-key]');

// Level colors are one palette by position for every router: a slot for each level of the router with the most.
const colorSlots = () => Math.max(1, ...S.routers.map((r) => (r.levels || []).length));

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
  $('cfg-brandcustom-wrap').hidden = $('cfg-brandborder').value !== 'custom';
  setBrand($('brand-swatch'), c);
  $('brand-icon').src = iconURL(180, c);
  $('brand-caution').hidden = !brandCaution(c, S.brandColors);
}
function fillBrandColors(v) {
  const sel = $('cfg-brandborder');
  const opt = (value, text) => Object.assign(el('option', null, text), { value });
  sel.replaceChildren(opt('', 'Off'), ...pickerColors(S.brandColors, v).map((c) => opt(c.name, c.label)), opt('custom', 'Custom…'));
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
});
// The swatch is a real color input under its label, so a click opens the picker at the swatch. showPicker() from the
// select's change handler opens it at the select instead.
$('cfg-brandcustom-wrap').append(icon('paintbrush', false));
$('cfg-brandcustom').addEventListener('input', renderBrandPreview);
$('brand-caution').append(icon('triangle-alert'), el('span', null, CAUTION_NOTE));

export function fillForm() {
  for (const f of fields()) {
    const v = S.cfg[f.dataset.key];
    if (f.dataset.type === 'bool') f.checked = !!v;
    else if (f.dataset.type === 'list') f.value = (v || []).join(', ');
    else f.value = v ?? '';
  }
  fillBrandColors(S.cfg.brandBorder);
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
  c.browserDefaults = Object.fromEntries([...form.querySelectorAll('select[data-gpref]')].map((s) => [s.dataset.gpref, toGlobal(s.dataset.gpref, s.value)]));
  return c;
}

// The first field readSettings' result cannot be saved with: {text, node}, or null. The PIN fields are the save path's.
// The router settings check themselves (settingsrouterdetail.js).

export function formProblem(c) {
  const badColor = [...$('cfg-colors').querySelectorAll('input.pb-input')].find((f) => f.value.trim() && !HEX.test(f.value.trim()));
  if (badColor) return { text: `${badColor.getAttribute('aria-label')} must be hex, like #3a7bd5.`, node: badColor };
  const hidden = parseFilter(c.hiddenDsts);
  if (hidden.error) return { text: `Hidden destinations in History: ${hidden.error}.`, node: $('cfg-hidden') };
  const bad = [...fields()].find((f) => intProblem([{ key: f.dataset.key, type: f.dataset.type, value: f.value }]));
  if (bad) return { text: `${labelText(form.querySelector(`label[for="${bad.id}"]`)) || 'A number field'} must be a whole number.`, node: bad };
  return null;
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

// Each slot is a position: level 1 of every router takes slot 1, whatever its numbering shows.
function renderColors(vals) {
  const n = colorSlots();
  const rows = [];
  for (let ui = 1; ui <= Math.min(n, 256); ui++) {
    const slot = el('span', 'sode__slot sode__slot--on', String(ui));
    slot.setAttribute('aria-hidden', 'true');
    const tip = positionTip(ui, S.routers);
    if (tip) slot.dataset.pbTip = tip;
    const f = el('input', 'pb-input');
    f.id = `cfg-color-${ui}`;
    f.value = vals[ui - 1] || '';
    f.placeholder = defaultHex(ui);
    f.spellcheck = false;
    f.setAttribute('aria-label', `Position ${ui} color`);
    if (tip) f.setAttribute('aria-description', tip);
    f.addEventListener('input', () => {
      preview(slot, ui, f.value);
      f.classList.toggle('pb-input--invalid', f.value.trim() !== '' && !HEX.test(f.value.trim()));
      renderColorsError();
    });
    preview(slot, ui, f.value);
    rows.push(el('div', 'sode__color', slot, f, colorPick(f)));
  }
  $('cfg-colors').replaceChildren(...rows);
  renderColorsError();
}

// The focus ring hides the invalid outline and the swatch falls back to the default, so a bad color says so at once.
function renderColorsError() {
  const bad = $('cfg-colors').querySelector('input.pb-input--invalid');
  $('cfg-colors-err').hidden = !bad;
  $('cfg-colors-err').textContent = bad ? `${bad.getAttribute('aria-label')} must be hex, like #3a7bd5.` : '';
}

export function renderHiddenNote() {
  const f = $('cfg-hidden');
  const p = parseFilter(f.value);
  f.classList.toggle('pb-input--invalid', !!p.error);
  let text = '';
  if (p.error) text = `${p.error}.`;
  else if (p.rules.length) {
    const fn = compileFilter(p.rules);
    const n = routerData('').dsts.filter((d) => fn(d.n + routerData('').off, d.name)).length;
    text = `${n} of ${routerData('').dsts.length} destinations`;
  }
  $('cfg-hidden-note').textContent = text;
}

$('cfg-hidden').addEventListener('input', renderHiddenNote);
$('cfg-colors-reset').addEventListener('click', () => {
  for (const f of $('cfg-colors').querySelectorAll('input.pb-input')) {
    f.value = '';
    f.dispatchEvent(new Event('input'));
  }
});
