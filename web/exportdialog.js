// Settings > Server > Export & Import: the dialog with the Export and Import tabs (docs/agents/profiles.md).
// The export routes take an Administrator's session token in a header; exportSession gives one to a
// connection that has none. The token stays in memory here, so sign-in storage is not changed. Pure rules are
// in exportrules.js. Import is a placeholder for now.

import { $, el, on, onMsg, wsSend } from './core.js';
import { trapDialog } from './widgets.js';
import { makeTabs } from './tabs.js';
import { layout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { PREFS } from './prefs.js';
import { ownPref } from './device.js';
import { localSets } from './filterstore.js';
import { localPresets } from './presetstore.js';
import { ROLES } from './usersrules.js';
import * as R from './exportrules.js';

const SESSION_HEADER = 'X-Sode-Session'; // sessionHeader in internal/app/exportroute.go
const SIZE_WAIT_MS = 300;

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal pb-modal--md sode__xi');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'xi-title');
dialog.tabIndex = -1;

const pinsBox = el('input', 'pb-switch__input');
pinsBox.type = 'checkbox';
pinsBox.id = 'xi-pins';
pinsBox.setAttribute('role', 'switch');
const cats = el('div', 'sode__xi-cats');
const exportPanel = el(
  'div',
  null,
  el('div', 'sode__field', el('label', 'pb-check', pinsBox, el('span', 'pb-check__label', 'Include PINs'))),
  el('span', 'sode__hint', 'Sizes are before compression.'),
  cats,
);
const importPanel = el('div', null, el('p', 'sode__hint', 'Import comes in a later release.'));
const phone = el('div');
const note = el('span', 'sode__hint');
note.id = 'xi-note';
note.setAttribute('role', 'status');
note.setAttribute('aria-live', 'polite');

const backupBtn = btn('pb-btn--outline', 'Back up everything', () => download(R.backupBody(browserValues())));
backupBtn.id = 'xi-backup';
const exportBtn = btn('pb-btn--ok', 'Export', () => download(R.exportBody(list, pick, browserValues())));
exportBtn.id = 'xi-export';
const closeBtn = btn('pb-btn--outline', 'Close', () => close());
closeBtn.id = 'xi-close';
const exportActs = el('span', 'sode__xi-acts', backupBtn, exportBtn);

const tabs = makeTabs(
  [
    { id: 'xi-export-tab', text: 'Export', panel: exportPanel },
    { id: 'xi-import-tab', text: 'Import', panel: importPanel },
  ],
  (id) => {
    exportActs.hidden = id !== 'xi-export-tab';
  },
);
const body = el('div', 'pb-modal__body', tabs.node, exportPanel, importPanel, phone);
dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Export & Import'), { id: 'xi-title' })),
  body,
  el('div', 'pb-modal__footer', closeBtn, note, exportActs),
);
root.append(dialog);
document.body.append(root);

let open = false;
let back = null;
let list = null;
let pick = null;
let token = null; // null: not asked yet; '' with no users
const expanded = new Set();
let sizeTimer = 0;
let sizeSeq = 0;

const setNote = (t, error = false) => {
  note.textContent = t;
  note.classList.toggle('sode__error-text', error);
};

// This browser: its own device preferences as top-level keys, with its filter sets and presets (browser.yaml in
// internal/app/testdata/export-v1).
function browserValues() {
  const out = {};
  for (const n of Object.keys(PREFS)) if (ownPref(n)) out[n] = ownPref(n);
  return { ...out, filterSets: localSets(), presets: localPresets() };
}

// ---- session token

let askSeq = 0;
const asks = new Map();
onMsg('exportSessionResult', (m) => {
  const done = asks.get(m.id);
  if (!done) return;
  asks.delete(m.id);
  done(m);
});

function session() {
  if (token !== null) return Promise.resolve(token);
  const id = ++askSeq;
  return new Promise((resolve, reject) => {
    if (!wsSend({ t: 'exportSession', id })) return reject(new Error('Not connected to Sode. Try again when the link is back.'));
    asks.set(id, (m) => {
      if (!m.ok) return reject(new Error(m.err || 'Sode refused the export.'));
      token = m.token || '';
      resolve(token);
    });
  });
}

async function call(path, init = {}) {
  const tok = await session();
  const headers = { ...init.headers };
  if (tok) headers[SESSION_HEADER] = tok;
  const res = await fetch(path, { ...init, headers, cache: 'no-store' });
  if (res.status === 401 || res.status === 403) token = null; // ask for a new session next time
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    throw new Error(err?.error?.message || err?.message || `Sode answered ${res.status}.`);
  }
  return res;
}

const post = (path, b) => call(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(b) });

// ---- the list

const sectionLabel = (id) => document.querySelector(`fieldset[data-section="${id}"] > legend`)?.textContent || id;
const roleLabel = (v) => ROLES.find((r) => r.value === v)?.label || v;
const browserBytes = () => new TextEncoder().encode(JSON.stringify(browserValues())).length;

// key: names the box across redraws, so focus stays on it.
function check(key, text, checked, mixed, onChange, disabled = false) {
  const box = el('input', 'pb-check__input');
  box.dataset.key = key;
  box.type = 'checkbox';
  box.checked = checked;
  box.indeterminate = mixed;
  box.disabled = disabled;
  box.addEventListener('change', () => onChange(box.checked));
  return el('label', 'pb-check', box, el('span', 'pb-check__label', text));
}

const sizeEl = (n) => el('span', 'sode__xi-size', R.sizeText(n));

function itemRow(key, text, detail, size, checked, onChange) {
  return el('div', 'sode__xi-item', check(key, text, checked, false, onChange), el('span', 'sode__hint', detail), sizeEl(size));
}

function render() {
  if (!list) return;
  const focused = dialog.contains(document.activeElement) ? document.activeElement.dataset.key : '';
  pinsBox.checked = pick.pins;
  const by = R.routerBroughtBy(list, pick);
  const rows = [];
  for (const [cat, text] of R.CATEGORIES) {
    const c = list.categories.find((x) => x.name === cat) || { items: [], sections: [] };
    const state = R.catState(list, pick, cat);
    const forced = cat === 'routers' && !pick.routers && !!by;
    const parts = cat === 'settings' ? c.sections || [] : c.items || [];
    const empty = cat !== 'browser' && !parts.length;
    const head = el('div', 'sode__xi-head');
    head.dataset.cat = cat;
    const caret = btn('pb-btn--ghost pb-btn--icon pb-btn--sm sode__xi-caret', expanded.has(cat) ? '▾' : '▸', () => {
      if (expanded.has(cat)) expanded.delete(cat);
      else expanded.add(cat);
      render();
    });
    caret.dataset.key = `caret:${cat}`;
    caret.setAttribute('aria-expanded', String(expanded.has(cat)));
    caret.setAttribute('aria-label', `Show the ${text.toLowerCase()} items`);
    caret.hidden = empty || cat === 'browser' || cat === 'routers';
    const count = parts.length > 1 && cat !== 'routers' ? ` ${cat === 'settings' ? pick.sections.size : pick.items[cat].size}/${parts.length}` : '';
    head.append(
      caret,
      check(
        `cat:${cat}`,
        text + count,
        state !== 'none' || forced,
        state === 'some',
        (on) => {
          R.setCat(list, pick, cat, on);
          changed();
        },
        empty || forced,
      ),
      el('span', 'sode__hint', empty ? 'None' : forced ? 'Always included' : ''),
      sizeEl(R.catSize(list, pick, cat, browserBytes())),
    );
    rows.push(head);
    if (forced && by) rows.push(el('div', 'sode__xi-items', el('span', 'sode__hint sode__xi-sub', `Brought by ${by.name}`)));
    if (!expanded.has(cat) || caret.hidden) continue;
    const items = el('div', 'sode__xi-items');
    if (cat === 'settings') {
      for (const s of parts)
        items.append(
          itemRow(`item:${cat}:${s.id}`, sectionLabel(s.id), '', s.size, pick.sections.has(s.id), (on) => {
            R.toggle(pick.sections, s.id, on);
            changed();
          }),
        );
    } else {
      for (const it of parts) {
        const detail = it.detail ? roleLabel(it.detail) : R.KIND_LABEL[it.kind] || '';
        items.append(
          itemRow(`item:${cat}:${it.id}`, it.name, detail, it.size, pick.items[cat].has(it.id), (on) => {
            R.toggle(pick.items[cat], it.id, on);
            changed();
          }),
        );
      }
    }
    rows.push(items);
  }
  cats.replaceChildren(...rows);
  if (focused) [...cats.querySelectorAll('[data-key]')].find((x) => x.dataset.key === focused)?.focus();
  exportBtn.disabled = R.nothingPicked(list, pick);
}

function changed() {
  render();
  sizeLater();
}

// The zip size, built by the server as the export would be.
function sizeLater() {
  clearTimeout(sizeTimer);
  const seq = ++sizeSeq;
  exportBtn.textContent = 'Export';
  if (R.nothingPicked(list, pick)) return;
  sizeTimer = setTimeout(async () => {
    try {
      const res = await post('api/export/size', R.exportBody(list, pick, browserValues()));
      const { size } = await res.json();
      if (seq === sizeSeq && open) exportBtn.textContent = `Export ${R.sizeText(size)}`;
    } catch (e) {
      if (seq === sizeSeq && open) setNote(e.message, true);
    }
  }, SIZE_WAIT_MS);
}

async function load() {
  setNote('Reading what Sode holds…');
  try {
    const res = await call('api/export/items');
    const got = await res.json();
    if (!open) return;
    list = got;
    pick = R.newPick(list);
    setNote('');
    changed();
  } catch (e) {
    if (open) setNote(e.message, true);
  }
}

async function download(b) {
  setNote('Making the file…');
  backupBtn.disabled = exportBtn.disabled = true;
  try {
    const res = await post('api/export', b);
    const blob = await res.blob();
    const a = el('a');
    a.href = URL.createObjectURL(blob);
    a.download = R.fileNameOf(res.headers.get('Content-Disposition'), new Date().toISOString().slice(0, 10));
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 0);
    setNote(`Saved ${a.download} (${R.sizeText(blob.size)}).`);
  } catch (e) {
    setNote(e.message, true);
  } finally {
    backupBtn.disabled = false;
    exportBtn.disabled = !list || R.nothingPicked(list, pick);
  }
}

pinsBox.addEventListener('change', () => {
  if (!pick) return;
  pick.pins = pinsBox.checked;
  sizeLater();
});

// ---- open and close

function openDialog() {
  back = document.activeElement;
  open = true;
  const desk = layout() !== 'phone';
  tabs.node.hidden = !desk;
  exportActs.hidden = !desk;
  phone.replaceChildren(desk ? '' : desktopOnly('export or import'));
  setNote('');
  if (desk) {
    tabs.select('xi-export-tab');
    load();
  } else exportPanel.hidden = importPanel.hidden = true;
  dlg.show();
  dialog.focus();
}

function close() {
  open = false;
  clearTimeout(sizeTimer);
  list = pick = null;
  cats.replaceChildren();
  exportBtn.textContent = 'Export';
  dlg.hide();
  if (back?.isConnected) back.focus();
}

const dlg = trapDialog(root, dialog, close);
$('xi-open')?.addEventListener('click', openDialog);
on('locking', () => {
  if (open) close();
});
// A sign-in as someone else, or a sign-out, ends what the token stood for.
on('access', () => {
  token = null;
});
