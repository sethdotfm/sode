// My user > Export & Import: the user's own profile, presets and filter sets (docs/agents/profiles.md). The other
// categories show greyed as Administrators only. Pure rules are in exportrules.js; the Import tab is importtab.js.

import { el, on, icon } from './core.js';
import { trapDialog } from './widgets.js';
import { makeTabs } from './tabs.js';
import { layout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import * as R from './exportrules.js';
import { post, call, downloadZip } from './exportcalls.js';
import { importTab } from './importtab.js';

const SIZE_WAIT_MS = 300;
const TITLE = 'Export & Import';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal pb-modal--md sode__xi-own'); // sode__xi stays unique to the Settings > Server dialog
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'own-xi-title');
dialog.tabIndex = -1;

const cats = el('div', 'sode__xi-cats');
const exportPanel = el('div', null, cats);
const phone = el('div');
const note = el('span', 'sode__hint');
note.id = 'own-xi-note';
note.setAttribute('role', 'status');
note.setAttribute('aria-live', 'polite');

const exportBtn = btn('pb-btn--ok', 'Export', () => download());
exportBtn.id = 'own-xi-export';
const closeBtn = btn('pb-btn--outline', 'Close', () => close());
const exportActs = el('span', 'sode__xi-acts', exportBtn);
const titleEl = Object.assign(el('span', 'pb-modal__title', TITLE), { id: 'own-xi-title' });
const importer = importTab({
  own: true,
  setNote: (t, error) => setNote(t, error),
  setReview: (title) => {
    titleEl.textContent = title || TITLE;
    tabs.node.hidden = !!title;
  },
  backup: () => {},
});

const tabs = makeTabs(
  [
    { id: 'own-xi-export-tab', text: 'Export', panel: exportPanel },
    { id: 'own-xi-import-tab', text: 'Import', panel: importer.panel },
  ],
  (id) => {
    exportActs.hidden = id !== 'own-xi-export-tab';
    importer.acts.hidden = id !== 'own-xi-import-tab';
  },
);
dialog.append(
  el('div', 'pb-modal__header', titleEl),
  el('div', 'pb-modal__body', tabs.node, exportPanel, importer.panel, phone),
  el('div', 'pb-modal__footer', closeBtn, note, exportActs, importer.acts),
);
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

function render() {
  if (!list) return;
  const focused = dialog.contains(document.activeElement) ? document.activeElement.dataset.key : '';
  const rows = [];
  for (const [cat, text] of R.OWN_CATEGORIES) {
    const items = list.categories.find((c) => c.name === cat)?.items || [];
    const state = R.ownState(list, pick, cat);
    const many = cat !== 'profile' && items.length > 0;
    const caret = btn('pb-btn--ghost pb-btn--icon pb-btn--sm sode__xi-caret', icon(expanded.has(cat) ? 'chevron-down' : 'chevron-right'), () => {
      if (expanded.has(cat)) expanded.delete(cat);
      else expanded.add(cat);
      render();
    });
    caret.dataset.key = `caret:${cat}`;
    caret.setAttribute('aria-expanded', String(expanded.has(cat)));
    caret.setAttribute('aria-label', `Show the ${text.toLowerCase()} items`);
    const head = el(
      'div',
      'sode__xi-head',
      many ? caret : el('span'),
      check(
        `cat:${cat}`,
        many && items.length > 1 ? `${text} ${pick.items[cat].size}/${items.length}` : text,
        state !== 'none',
        state === 'some',
        (on) => {
          pick.items[cat] = new Set(on ? items.map((i) => i.id) : []);
          changed();
        },
        !items.length,
      ),
      el('span', 'sode__hint', items.length ? '' : 'None'),
      el('span', 'sode__xi-size', R.sizeText(R.ownSize(list, pick, cat))),
    );
    head.dataset.cat = cat;
    rows.push(head);
    if (!many || !expanded.has(cat)) continue;
    rows.push(
      el(
        'div',
        'sode__xi-items',
        ...items.map((it) =>
          el(
            'div',
            'sode__xi-item',
            check(`item:${cat}:${it.id}`, it.name, pick.items[cat].has(it.id), false, (on) => {
              R.toggle(pick.items[cat], it.id, on);
              changed();
            }),
            el('span', 'sode__hint'),
            el('span', 'sode__xi-size', R.sizeText(it.size)),
          ),
        ),
      ),
    );
  }
  for (const [cat, text] of R.ADMIN_ONLY) {
    const head = el(
      'div',
      'sode__xi-head',
      el('span'),
      check(`admin:${cat}`, text, false, false, () => {}, true),
    );
    head.append(el('span', 'sode__hint', 'Administrators only'), el('span', 'sode__xi-size'));
    head.dataset.cat = `admin:${cat}`;
    rows.push(head);
  }
  cats.replaceChildren(...rows);
  if (focused) [...cats.querySelectorAll('[data-key]')].find((x) => x.dataset.key === focused)?.focus();
  exportBtn.disabled = !R.ownBody(pick).categories.length;
}

function changed() {
  render();
  clearTimeout(sizeTimer);
  const seq = ++sizeSeq;
  exportBtn.textContent = 'Export';
  const body = R.ownBody(pick);
  if (!body.categories.length) return;
  sizeTimer = setTimeout(async () => {
    try {
      const { size } = await (await post('api/own/export/size', body)).json();
      if (seq === sizeSeq && open) exportBtn.textContent = `Export ${R.sizeText(size)}`;
    } catch (e) {
      if (seq === sizeSeq && open) setNote(e.message, true);
    }
  }, SIZE_WAIT_MS);
}

async function load() {
  setNote('Reading…');
  try {
    const got = await (await call('api/own/export/items')).json();
    if (!open) return;
    list = got;
    pick = R.newOwnPick(list);
    setNote('');
    changed();
  } catch (e) {
    if (open) setNote(e.message, true);
  }
}

async function download() {
  setNote('Exporting…');
  exportBtn.disabled = true;
  try {
    const got = await downloadZip(R.ownBody(pick), 'api/own/export');
    setNote(`Saved ${got.name} (${R.sizeText(got.size)}).`);
  } catch (e) {
    setNote(e.message, true);
  } finally {
    exportBtn.disabled = !list || !R.ownBody(pick).categories.length;
  }
}

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
    tabs.select('own-xi-export-tab');
    exportBtn.disabled = true; // until the list comes
    load();
  } else exportPanel.hidden = importer.panel.hidden = true;
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
on('openOwnExport', openDialog);
on('locking', () => {
  if (open) close();
});
