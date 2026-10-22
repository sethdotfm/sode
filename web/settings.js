// Settings view, shared by both layouts. Lives at #settings so Back and
// reload work. Server settings save together; the layout choice is per
// browser and applies at once.

import { $, S, on, wsSend, fmtTime, showResult } from './core.js';
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
  $('cfg-layout').value = layoutPref();
  $('settings-err').hidden = true;
  $('settings-save').disabled = false;
  $('settings-path').textContent = S.configPath ? `Saved to ${S.configPath}` : 'Not saved to disk (SODE_CONFIG unset)';
  renderOffsetNote();
  renderNamesStatus();
  root.dataset.view = 'settings';
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
  return c;
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

for (const id of ['settings-open', 'm-settings']) $(id).addEventListener('click', openSettings);
$('settings-cancel').addEventListener('click', closeSettings);
$('settings-back').addEventListener('click', closeSettings);
$('cfg-offset').addEventListener('input', renderOffsetNote);
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
