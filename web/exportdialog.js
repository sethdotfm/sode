import { $, el, on } from './core.js';
import { btn } from './formbits.js';
import { trapDialog } from './widgets.js';
import { makeTabs } from './tabs.js';
import { layout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { ROLES } from './usersrules.js';
import * as R from './exportrules.js';
import { browserValues, post, call, downloadZip, downloadBackup } from './exportcalls.js';
import { importTab } from './importtab.js';

const SIZE_WAIT_MS = 300;

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
const exportPanel = el('div', null, el('div', 'sode__field', el('label', 'pb-check', pinsBox, el('span', 'pb-check__label', 'Include PINs'))), cats);
const phone = el('div');
const note = el('span', 'sode__hint');
note.id = 'xi-note';
note.setAttribute('role', 'status');
note.setAttribute('aria-live', 'polite');

const backupBtn = btn('pb-btn--outline', 'Back up everything', () => download(downloadBackup));
backupBtn.id = 'xi-backup';
const exportBtn = btn('pb-btn--ok', 'Export', () => download(() => downloadZip(R.exportBody(list, pick, browserValues()))));
exportBtn.id = 'xi-export';
const closeBtn = btn('pb-btn--outline', 'Close', () => close());
closeBtn.id = 'xi-close';
const exportActs = el('span', 'sode__xi-acts', backupBtn, exportBtn);
const titleEl = Object.assign(el('span', 'pb-modal__title', 'Export & Import'), { id: 'xi-title' });
const importer = importTab({
  setNote: (t, error) => setNote(t, error),
  // The review fills the whole body, so the tabs go while it shows.
  setReview: (title) => {
    titleEl.textContent = title || 'Export & Import';
    tabs.node.hidden = !!title;
  },
  backup: () => download(downloadBackup),
});
const importPanel = importer.panel;

const tabs = makeTabs(
  [
    { id: 'xi-export-tab', text: 'Export', panel: exportPanel },
    { id: 'xi-import-tab', text: 'Import', panel: importPanel },
  ],
  (id) => {
    exportActs.hidden = id !== 'xi-export-tab';
    importer.acts.hidden = id !== 'xi-import-tab';
  },
);
const body = el('div', 'pb-modal__body', tabs.node, exportPanel, importPanel, phone);
dialog.append(el('div', 'pb-modal__header', titleEl), body, el('div', 'pb-modal__footer', closeBtn, note, exportActs, importer.acts));
root.append(dialog);
document.body.append(root);

let open = false;
let back = null;
let list = null;
let pick = null;
const expanded = new Set();
let sizeTimer = 0;
let sizeSeq = 0;

const setNote = (t, error = false) => {
  note.textContent = t;
  note.classList.toggle('sode__error-text', error);
};

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

function itemRow(key, text, detail, size, checked, onChange, disabled = false) {
  return el('div', 'sode__xi-item', check(key, text, checked, false, onChange, disabled), el('span', 'sode__hint', detail), sizeEl(size));
}

function render() {
  if (!list) return;
  const focused = dialog.contains(document.activeElement) ? document.activeElement.dataset.key : '';
  pinsBox.checked = pick.pins;
  const by = R.routerBroughtBy(list, pick);
  const icons = R.iconsBroughtBy(list, pick);
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
        const bringer = pick.items[cat].has(it.id) ? null : icons.get(it.id);
        const row = itemRow(
          `item:${cat}:${it.id}`,
          it.name,
          detail,
          it.size,
          pick.items[cat].has(it.id) || !!bringer,
          (on) => {
            R.toggle(pick.items[cat], it.id, on);
            changed();
          },
          !!bringer,
        );
        if (bringer) row.append(el('span', 'sode__hint sode__xi-sub', `Brought by ${bringer.name}`));
        items.append(row);
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
  setNote('Reading…');
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

// make: resolves { name, size } of the saved zip (exportcalls.js).
async function download(make) {
  setNote('Making the file…');
  backupBtn.disabled = exportBtn.disabled = true;
  try {
    const got = await make();
    setNote(`Saved ${got.name} (${R.sizeText(got.size)}).`);
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

function openDialog() {
  back = document.activeElement;
  open = true;
  const desk = layout() !== 'phone';
  tabs.node.hidden = !desk;
  exportActs.hidden = !desk;
  importer.acts.hidden = true;
  phone.replaceChildren(desk ? '' : desktopOnly('export or import'));
  setNote('');
  if (desk) {
    tabs.select('xi-export-tab');
    exportBtn.disabled = true; // until the list comes
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
  importer.reset();
  exportBtn.textContent = 'Export';
  dlg.hide();
  if (back?.isConnected) back.focus();
}

const dlg = trapDialog(root, dialog, close);
$('xi-open')?.addEventListener('click', openDialog);
on('locking', () => {
  if (open) close();
});
