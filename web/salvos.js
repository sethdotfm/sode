// Salvos in the page: the buttons above Recent (both layouts) and the
// editor dialog. Pressing a button builds a plan and the layout showing opens
// its normal confirm; the confirm sends one batch. Storage and the pure
// parts are salvostore.js, the plan is plans.js.

import { el, S, on, off, search, srcItem, dstItem, nameProblem } from './core.js';
import { levelInput, parseLevels } from './levelview.js';
import { showResult, salvoPlan } from './plans.js';
import { globalSaver } from './globalsave.js';
import { trapDialog } from './widgets.js';
import {
  allSalvos, globalSalvos, localSalvos, findSalvo, normSalvo, salvoProblem, salvosProblem, replaceSalvo, saveLocalSalvo, deleteLocalSalvo,
  parseRefs, itemsFromTakes, latestFolderTakes,
} from './salvostore.js';

// ---- the layouts: each registers once what it has

const hosts = [];
// host: { active(), confirm(plan), patch() }. patch() gives the patch
// boxes as { src: item, dsts: [item], levels: [ui] } (levels [] = the
// default levels), or null while they hold no patch.
export const salvoHost = (host) => { hosts.push(host); };
const shown = () => hosts.find((h) => h.active());

// ---- the buttons

export function fire(scope, name) {
  const s = findSalvo(scope, name);
  const host = shown();
  if (!s || !host) return;
  if (!S.srcBy.size) return showResult('warn', 'Salvo not sent', 'The router’s names have not loaded yet.');
  const plan = salvoPlan(s, parseLevels('') || []);
  if (!plan.items.length) {
    return showResult('error', `Salvo ${s.name} not sent`, plan.warnings.length ? plan.warnings.join('; ') + '.' : 'It has no takes.');
  }
  host.confirm(plan);
}

function summary(s) {
  const n = s.items.reduce((a, it) => a + it.dsts.length, 0);
  return `${s.items.length} source${s.items.length === 1 ? '' : 's'}, ${n} destination${n === 1 ? '' : 's'}`;
}

function renderStrips() {
  const list = allSalvos();
  for (const box of document.querySelectorAll('[data-salvos]')) {
    const edit = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', list.length ? 'Edit' : 'Salvos…');
    edit.type = 'button';
    edit.setAttribute('aria-haspopup', 'dialog');
    edit.addEventListener('click', () => openSalvos());
    const chips = list.map((s) => {
      const b = el('button', 'pb-chip sode__set', s.name);
      b.type = 'button';
      b.title = `${summary(s)} · ${s.scope === 'global' ? 'Everyone' : 'This browser'}`;
      b.addEventListener('click', () => fire(s.scope, s.name));
      return b;
    });
    const label = list.length ? el('span', 'sode__label', 'Salvos') : null;
    box.replaceChildren(...[label, ...chips, edit].filter(Boolean));
  }
}
on('salvos', () => { renderStrips(); if (open) renderEditor(true); });
renderStrips();

// ---- the dialog

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__filters sode__salvos-dlg');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'salvos-title');
dialog.tabIndex = -1;

const pick = el('select', 'pb-select');
pick.setAttribute('aria-label', 'Salvo');
const btn = (cls, text, fn = () => {}) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};
const nameIn = el('input', 'pb-input');
nameIn.maxLength = 32;
nameIn.placeholder = 'Name';
nameIn.setAttribute('aria-label', 'Salvo name');
nameIn.autocomplete = 'off';
const scopeIn = el('select', 'pb-select');
scopeIn.setAttribute('aria-label', 'Save for');
scopeIn.append(new Option('This browser', 'local'), new Option('Everyone', 'global'));
const rowsBox = el('div', 'sode__salvo-rows');
const noteEl = el('span', 'sode__hint');
noteEl.setAttribute('aria-live', 'polite');
const pinIn = el('input', 'pb-input');
pinIn.type = 'password';
pinIn.placeholder = 'PIN';
pinIn.autocomplete = 'current-password';
pinIn.setAttribute('aria-label', 'Settings PIN');
const pinOk = btn('pb-btn--ok pb-btn--sm', 'Unlock');
const pinRow = el('div', 'sode__filters-set', pinIn, pinOk, btn('pb-btn--ghost pb-btn--sm', 'Cancel', () => saver.cancel()));
pinRow.hidden = true;
const delBtn = btn('pb-btn--outline pb-btn--sm', 'Delete', () => deleteClick());
const saveBtn = btn('pb-btn--commit pb-btn--sm', 'Save', () => save());

dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Salvos'), { id: 'salvos-title' })),
  el('div', 'pb-modal__body sode__filters-body',
    el('div', 'sode__filters-sets',
      el('div', 'sode__filters-set', el('span', 'pb-select-wrap', pick), btn('pb-btn--outline pb-btn--sm', 'New', () => load(null))),
      el('div', 'sode__filters-set', nameIn, el('span', 'pb-select-wrap sode__filters-scope', scopeIn), saveBtn, delBtn),
      pinRow, noteEl),
    rowsBox,
    el('div', 'sode__filters-set',
      btn('pb-btn--outline pb-btn--sm', 'Add take', () => { cur.items.push(blankRow()); renderRows(); focusLast(); }),
      btn('pb-btn--outline pb-btn--sm', 'Fill from patch boxes', fillFromBoxes),
      btn('pb-btn--outline pb-btn--sm', 'Fill from latest folder in Recent', fillFromHistory)),
    el('span', 'sode__hint', 'Sources and destinations: numbers (12, 20-24) or names, separated by commas. Levels blank = the default levels. Salvos are stored as numbers, so a renamed port stays in.')),
  el('div', 'pb-modal__footer', btn('pb-btn--ok', 'Done', () => closeSalvos())),
);
root.append(dialog);
document.body.append(root);

function setNote(text, error = false) {
  noteEl.textContent = text;
  noteEl.classList.toggle('sode__error-text', error);
}

// Salvos for everyone: the whole list is sent, after the PIN if one is set.
const saver = globalSaver({
  t: 'salvos', resultT: 'salvosResult', pinIn, pinRow, pinOk, setNote,
  isOpen: () => open, failTitle: 'Salvos not saved', okNote: 'Saved for everyone.',
  render: () => renderHead(true),
});

let open = false, back = null;
// cur: the salvo being edited: { scope, old, name, items: [{ src, dsts, levels }] } as typed text.
let cur = null, dirty = false, deleting = false;
const blankRow = () => ({ src: '', dsts: '', levels: '' });

const display = (n) => String(n + off());
function rowFromItem(it) {
  return { src: display(it.src), dsts: it.dsts.map(display).join(', '), levels: it.levels.length ? levelInput(it.levels) : '' };
}
function load(s) {
  deleting = false;
  dirty = false;
  cur = s
    ? { scope: s.scope, old: s.name, name: s.name, items: s.items.map(rowFromItem) }
    : { scope: 'local', old: '', name: '', items: [blankRow()] };
  renderEditor();
}

const lookup = (list) => (t) => search(list, t, 1)[0]?.n ?? null;
// A row's typed text as a take, plus a line saying what it means.
function parseRow(r) {
  const s = parseRefs(r.src, off(), lookup(S.srcs));
  const d = parseRefs(r.dsts, off(), lookup(S.dsts));
  let levels = [], badLevels = false;
  if (r.levels.trim()) {
    const l = parseLevels(r.levels);
    if (l) levels = l; else badLevels = true;
  }
  const bad = [...s.bad.map((t) => `source “${t}”`), ...d.bad.map((t) => `destination “${t}”`), ...(badLevels ? ['levels'] : [])];
  if (s.ns.length > 1) bad.push('more than one source');
  const item = { src: s.ns.length === 1 ? s.ns[0] : -1, dsts: d.ns, levels };
  const known = S.srcBy.has(item.src) && item.dsts.every((n) => S.dstBy.has(n));
  const text = bad.length ? `Not understood: ${bad.join(', ')}`
    : !item.dsts.length && item.src < 0 ? ''
      : `${item.src >= 0 ? srcItem(item.src).label : '?'} → ${item.dsts.map((n) => dstItem(n).label).join(', ') || '?'}${known ? '' : ' (not on the router now)'}`;
  return { item, text, error: bad.length > 0, known };
}

function renderRows() {
  rowsBox.replaceChildren(...cur.items.map((r, i) => {
    const hint = el('span', 'sode__hint sode__salvo-hint');
    const upd = () => {
      const p = parseRow(r);
      hint.textContent = p.text;
      hint.classList.toggle('sode__error-text', p.error || (!p.known && !!p.text));
    };
    const field = (key, ph, label) => {
      const inp = el('input', 'pb-input');
      inp.value = r[key];
      inp.placeholder = ph;
      inp.autocomplete = 'off';
      inp.spellcheck = false;
      inp.setAttribute('aria-label', label);
      inp.addEventListener('input', () => { r[key] = inp.value; dirty = true; upd(); setNote(''); });
      return inp;
    };
    upd();
    return el('div', 'sode__salvo-row',
      field('src', 'Source', `Take ${i + 1} source`),
      field('dsts', 'Destinations', `Take ${i + 1} destinations`),
      field('levels', 'Levels', `Take ${i + 1} levels`),
      btn('pb-btn--ghost pb-btn--sm', 'Remove', () => { cur.items.splice(i, 1); dirty = true; renderRows(); }),
      hint);
  }));
}
function focusLast() {
  const ins = rowsBox.querySelectorAll('.sode__salvo-row:last-child input');
  ins[0]?.focus();
}

// The salvo picker and the buttons around the name. keep: what is being
// typed (the name) stays as it is.
function renderHead(keep = false) {
  pick.replaceChildren(new Option('New salvo…', ''), ...allSalvos().map((s) => new Option(`${s.name} (${s.scope === 'global' ? 'Everyone' : 'This browser'})`, `${s.scope}:${s.name}`)));
  pick.value = cur.old ? `${cur.scope}:${cur.old}` : '';
  if (!keep) nameIn.value = cur.name;
  scopeIn.value = cur.scope;
  scopeIn.disabled = !!cur.old;
  delBtn.hidden = !cur.old;
  delBtn.textContent = deleting ? 'Really delete?' : 'Delete';
  saveBtn.disabled = saver.busy();
}

// keepEdit: the lists changed under us (a reconnect, another browser's
// save); only the picker is refreshed, never the rows or name being typed.
function renderEditor(keepEdit = false) {
  if (keepEdit && cur?.old && !findSalvo(cur.scope, cur.old) && !dirty) return load(null);
  renderHead(keepEdit);
  if (keepEdit) return;
  renderRows();
  setNote(cur.old ? (cur.scope === 'global' ? 'Everyone' : 'This browser') : '');
}
pick.addEventListener('change', () => {
  const v = pick.value;
  if (!v) return load(null);
  const i = v.indexOf(':');
  load(findSalvo(v.slice(0, i), v.slice(i + 1)));
});
nameIn.addEventListener('input', () => { cur.name = nameIn.value; dirty = true; setNote(''); });
scopeIn.addEventListener('change', () => { cur.scope = scopeIn.value; });

function fillFromBoxes() {
  const p = shown()?.patch();
  if (!p) return setNote('Pick a Source and a Destination in the main boxes first.', true);
  addRows([{ src: p.src.n, dsts: p.dsts.map((d) => d.n), levels: p.levels }], 'Added the patch in the boxes.');
}
function fillFromHistory() {
  const items = itemsFromTakes(latestFolderTakes(S.hist));
  if (!items.length) return setNote('No takes made in Sode yet.', true);
  addRows(items, `Added ${items.length === 1 ? 'the latest take' : 'the latest folder'} from Recent.`);
}
function addRows(items, note) {
  if (cur.items.length === 1 && !cur.items[0].src && !cur.items[0].dsts) cur.items = [];
  cur.items.push(...items.map(rowFromItem));
  dirty = true;
  renderRows();
  setNote(note);
}

// ---- saving

// The typed rows as a salvo, or an error line. next: the scope's list as
// it would be saved.
function built() {
  const items = [];
  for (const r of cur.items) {
    const p = parseRow(r);
    if (p.error) return { err: p.text };
    items.push(p.item);
  }
  const s = normSalvo({ name: cur.name, items });
  const list = cur.scope === 'global' ? globalSalvos() : localSalvos();
  const err = nameProblem(s.name, list.map((x) => x.name), { noun: 'salvo', old: cur.old }) || salvoProblem(s);
  if (err) return { err };
  const next = replaceSalvo(list, s, cur.old);
  const tooMuch = salvosProblem(next);
  return tooMuch ? { err: tooMuch } : { s, next };
}

function save() {
  const b = built();
  if (b.err) return setNote(b.err, true);
  if (cur.scope === 'local') {
    saveLocalSalvo(b.s, cur.old);
    cur.old = b.s.name;
    cur.name = b.s.name;
    dirty = false;
    renderEditor();
    return setNote('Saved for this browser.');
  }
  saver.change({ salvos: b.next }, () => { cur.old = b.s.name; cur.name = b.s.name; nameIn.value = cur.name; dirty = false; });
}

function deleteClick() {
  if (!cur.old) return;
  if (!deleting) { deleting = true; delBtn.textContent = 'Really delete?'; return; }
  deleting = false;
  const name = cur.old;
  if (cur.scope === 'local') {
    deleteLocalSalvo(name);
    load(null);
    return setNote('Deleted.');
  }
  const next = globalSalvos().filter((s) => s.name.toLowerCase() !== name.toLowerCase()).map(({ name: n, items }) => ({ name: n, items }));
  saver.change({ salvos: next }, () => load(null));
}

// ---- open / close

export const salvosOpen = () => open;

export function openSalvos(scope = '', name = '') {
  back = document.activeElement;
  const s = scope && findSalvo(scope, name);
  open = true;
  pinRow.hidden = true;
  load(s || null);
  root.hidden = false;
  dialog.focus();
}

function closeSalvos() {
  open = false;
  root.hidden = true;
  saver.close(); // a save already sent finishes first (its answer locks up after it)
  if (back?.isConnected) back.focus();
}

trapDialog(root, dialog, closeSalvos);
