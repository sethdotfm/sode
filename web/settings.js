// Settings view, shared by both layouts. Lives at #settings so Back and
// reload work. Server settings save together; the layout choice is per
// browser and applies at once.

import { $, el, S, on, wsSend, fmtTime, showResult, levelColor } from './core.js';
import { layout, layoutPref, setLayoutPref } from './layout.js';

const root = document.documentElement;
const form = $('settings-view');
const fields = () => form.querySelectorAll('[data-key]');
const isOpen = () => root.dataset.view === 'settings';

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
  renderColors(S.cfg.levelColors || []);
  $('cfg-layout').value = layoutPref();
  resetClear();
  $('settings-err').hidden = true;
  $('settings-save').disabled = false;
  $('settings-path').textContent = S.configPath ? `Saved to ${S.configPath}` : 'Not saved to disk (SODE_CONFIG unset)';
  root.dataset.view = 'settings'; // before renderNamesStatus, which only draws when open
  renderOffsetNote();
  renderNamesStatus();
  window.scrollTo(0, 0);
  if (layout() === 'desktop') $('cfg-router').focus();
}

function closeSettings() {
  if (location.hash === '#settings') history.replaceState(null, '', location.pathname + location.search);
  delete root.dataset.view;
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
  return c;
}

// ---- level colours: one hex box per level, previewed on its slot

const HEX = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i;
const toHex = (v) => {
  const h = v.trim().replace(/^#/, '').toLowerCase();
  return '#' + (h.length === 3 ? [...h].map((c) => c + c).join('') : h);
};

const swatchOf = (ui) => (ui === 1 ? 'video' : String((ui - 2) % 16 + 1).padStart(2, '0'));

// The default swatch's hex in the current theme, for a placeholder.
function defaultHex(ui) {
  const probe = el('span');
  probe.style.color = `var(--pb-signal-${swatchOf(ui)})`;
  document.body.append(probe);
  const rgb = getComputedStyle(probe).color.match(/\d+/g) || [];
  probe.remove();
  return '#' + rgb.slice(0, 3).map((c) => Number(c).toString(16).padStart(2, '0')).join('');
}

function preview(slot, ui, v) {
  levelColor(slot, { swatch: swatchOf(ui), color: HEX.test(v.trim()) ? toHex(v) : '' });
}

// vals: colours to show, by level; keeps what's typed when the count changes.
function renderColors(vals) {
  const n = Number($('cfg-levels').value);
  const rows = [];
  for (let ui = 1; Number.isInteger(n) && ui <= Math.min(n, 256); ui++) {
    const slot = el('span', 'sode__slot sode__slot--on', String(ui));
    slot.setAttribute('aria-hidden', 'true');
    const f = el('input', 'pb-input');
    f.id = `cfg-color-${ui}`;
    f.value = vals[ui - 1] || '';
    f.placeholder = defaultHex(ui);
    f.spellcheck = false;
    f.setAttribute('aria-label', `Level ${ui} colour`);
    f.addEventListener('input', () => {
      preview(slot, ui, f.value);
      f.classList.toggle('pb-input--invalid', f.value.trim() !== '' && !HEX.test(f.value.trim()));
    });
    preview(slot, ui, f.value);
    rows.push(el('div', 'sode__color', slot, f));
  }
  $('cfg-colors').replaceChildren(...rows);
}

function renderOffsetNote() {
  const off = Number($('cfg-offset').value);
  $('cfg-offset-note').textContent = Number.isInteger(off) ? `Level 1 is sent as wire level ${1 + off}` : '';
}

function renderNamesStatus() {
  if (!isOpen()) return;
  const at = S.namesAt && new Date(S.namesAt);
  const when = at && at.getFullYear() > 2000 ? `refreshed ${fmtTime(S.namesAt)}` : 'not loaded yet';
  $('names-status').textContent = `${S.srcs.length} sources, ${S.dsts.length} destinations, ${when}`;
  const btn = $('names-refresh');
  btn.disabled = S.router !== 'up';
  btn.textContent = 'Refresh names now';
}

function configResult(m) {
  $('settings-save').disabled = false;
  if (m.ok) {
    if (isOpen()) closeSettings();
    showResult('ok', 'Settings saved', S.configPath ? `Written to ${S.configPath}.` : 'Applied (not saved to disk).');
    return;
  }
  $('settings-err').textContent = m.err;
  $('settings-err').hidden = false;
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
  showResult('ok', 'History cleared', 'The Recent list is empty for everyone.');
});

for (const id of ['settings-open', 'm-settings']) $(id).addEventListener('click', openSettings);
$('settings-cancel').addEventListener('click', closeSettings);
$('settings-back').addEventListener('click', closeSettings);
$('cfg-offset').addEventListener('input', renderOffsetNote);
$('cfg-colors-reset').addEventListener('click', () => {
  for (const f of $('cfg-colors').querySelectorAll('input')) {
    f.value = '';
    f.dispatchEvent(new Event('input'));
  }
});
$('cfg-levels').addEventListener('input', () => {
  const vals = [...$('cfg-colors').querySelectorAll('input')].map((f) => f.value);
  renderColors([...vals, ...(S.cfg.levelColors || []).slice(vals.length)]);
});
$('cfg-layout').addEventListener('change', (e) => setLayoutPref(e.target.value));
$('names-refresh').addEventListener('click', () => {
  if (wsSend({ t: 'refreshNames' })) {
    $('names-refresh').disabled = true;
    $('names-refresh').textContent = 'Refreshing…';
  }
});
form.addEventListener('submit', (e) => {
  e.preventDefault();
  const c = readSettings();
  const bad = [...fields()].find((f) => f.dataset.type === 'int' && !Number.isInteger(c[f.dataset.key]));
  const badColor = [...$('cfg-colors').querySelectorAll('input')].find((f) => f.value.trim() && !HEX.test(f.value.trim()));
  if (badColor) {
    $('settings-err').textContent = `${badColor.getAttribute('aria-label')} must be a hex colour like #3a7bd5.`;
    $('settings-err').hidden = false;
    badColor.focus();
    return;
  }
  if (bad) {
    $('settings-err').textContent = `${form.querySelector(`label[for="${bad.id}"]`)?.textContent || 'A number field'} must be a whole number.`;
    $('settings-err').hidden = false;
    bad.focus();
    return;
  }
  if (!wsSend({ t: 'config', config: c })) {
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

on('hello', showView);
on('names', renderNamesStatus);
on('namesChecked', renderNamesStatus);
on('conn', renderNamesStatus);
on('configResult', configResult);
