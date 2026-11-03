// Salvos in the page: the buttons above Recent (both layouts) and the
// editor dialog. Pressing a button builds a plan and the layout opens its
// normal confirm ('salvoFire'); the confirm sends one batch. Storage and
// the pure parts are salvostore.js.

import { el, S, on, onMsg, emit, off, wsSend, sentence, search, srcItem, dstItem } from './core.js';
import { levelInput, parseLevels } from './levelview.js';
import { showResult } from './plans.js';
import { unlock, relock, unlocked } from './pin.js';
import {
  allSalvos, globalSalvos, localSalvos, findSalvo, salvoPlan, normSalvo, nameProblem, salvoProblem, saveLocalSalvo, deleteLocalSalvo,
  parseRefs, itemsFromTakes, latestFolderTakes, MAX_ITEMS, MAX_DSTS,
} from './salvostore.js';

// ---- the buttons

const defaults = () => parseLevels('') || [];

export function fire(scope, name) {
  const s = findSalvo(scope, name);
  if (!s) return;
  if (!S.srcBy.size) return showResult('warn', 'Salvo not sent', 'The router’s names have not loaded yet.');
  const plan = salvoPlan(s, defaults());
  if (!plan.items.length) {
    return showResult('error', `Salvo ${s.name} not sent`, plan.missing.length ? plan.missing.join('; ') + '.' : 'It has no takes.');
  }
  emit('salvoFire', plan); // desktop.js or mobile.js, whichever is showing
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

// ---- where the patch boxes are: each layout says what it has

const providers = [];
// fn() -> { src: item, dsts: [item], levelsText: string } or null.
export const patchProvider = (fn) => { providers.push(fn); };
const currentPatch = () => { for (const fn of providers) { const p = fn(); if (p) return p; } return null; };

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
const btn = (cls, text, fn) => {
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
const pinOk = btn('pb-btn--ok pb-btn--sm', 'Unlock', () => sendPin());
const pinRow = el('div', 'sode__filters-set', pinIn, pinOk, btn('pb-btn--ghost pb-btn--sm', 'Cancel', () => cancelPin()));
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

let open = false, back = null;
// cur: the salvo being edited: { scope, old, name, items: [{ src, dsts, levels }] } as typed text.
let cur = null, dirty = false, deleting = false, saving = null, opSeq = 0;
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

function setNote(text, error = false) {
  noteEl.textContent = text;
  noteEl.classList.toggle('sode__error-text', error);
}

// keepEdit: the lists changed under us; keep what is being typed.
function renderEditor(keepEdit = false) {
  const list = allSalvos();
  if (keepEdit && cur?.old && !findSalvo(cur.scope, cur.old) && !dirty) load(null);
  pick.replaceChildren(new Option('New salvo…', ''), ...list.map((s) => new Option(`${s.name} (${s.scope === 'global' ? 'Everyone' : 'This browser'})`, `${s.scope}:${s.name}`)));
  pick.value = cur.old ? `${cur.scope}:${cur.old}` : '';
  nameIn.value = cur.name;
  scopeIn.value = cur.scope;
  scopeIn.disabled = !!cur.old;
  delBtn.hidden = !cur.old;
  delBtn.textContent = deleting ? 'Really delete?' : 'Delete';
  saveBtn.disabled = !!saving;
  renderRows();
  if (!keepEdit) setNote(cur.old ? (cur.scope === 'global' ? 'Everyone' : 'This browser') : '');
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
  const p = currentPatch();
  if (!p) return setNote('Pick a Source and a Destination in the main boxes first.', true);
  const it = { src: p.src.n, dsts: p.dsts.map((d) => d.n), levels: [] };
  const lv = p.levelsText.trim() ? parseLevels(p.levelsText) : [];
  if (lv) it.levels = lv;
  addRows([it], 'Added the patch in the boxes.');
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

// The typed rows as a salvo, or an error line.
function built() {
  const items = [];
  for (const r of cur.items) {
    const p = parseRow(r);
    if (p.error) return { err: p.text };
    items.push(p.item);
  }
  const s = normSalvo({ name: cur.name, items });
  const list = cur.scope === 'global' ? globalSalvos() : localSalvos();
  const err = nameProblem(list, s.name, cur.old) || salvoProblem(s);
  if (err) return { err };
  if (s.items.some((it) => it.dsts.length > MAX_DSTS) || s.items.length > MAX_ITEMS) return { err: 'Too many takes.' };
  return { s, list };
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
  const next = b.list.map(({ name, items }) => ({ name, items }));
  const at = next.findIndex((x) => x.name.toLowerCase() === cur.old.toLowerCase());
  const entry = { name: b.s.name, items: b.s.items };
  if (cur.old && at >= 0) next[at] = entry; else next.push(entry);
  changeGlobal(next, () => { cur.old = b.s.name; cur.name = b.s.name; dirty = false; });
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
  changeGlobal(next, () => load(null));
}

// Salvos for everyone: the whole list is sent, after the PIN if one is set.
function changeGlobal(list, done) {
  saving = { run: () => send(list), done };
  if (S.pinSet && !unlocked()) return askPin();
  saving.run();
}
function send(list) {
  saving.id = ++opSeq;
  if (!wsSend({ t: 'salvos', id: saving.id, salvos: list })) {
    saving = null;
    return setNote('Not connected to Sode.', true);
  }
  saveBtn.disabled = true;
  setNote('Saving…');
}
onMsg('salvosResult', (m) => {
  if (!saving || m.id !== saving.id) return;
  const s = saving;
  if (m.locked) {
    relock();
    if (open) { saving = { run: s.run, done: s.done }; return askPin('Enter the PIN again.'); }
  }
  saving = null;
  if (m.ok) s.done();
  if (open) {
    renderEditor();
    setNote(m.ok ? 'Saved for everyone.' : sentence(m.err), !m.ok);
    return;
  }
  if (!m.ok) showResult('error', 'Salvos not saved', sentence(m.err) || 'Settings are locked.');
  relock();
});

function askPin(err = '') {
  pinRow.hidden = false;
  pinIn.value = '';
  setNote(err || 'Enter the settings PIN.', !!err);
  pinIn.focus();
}
function cancelPin() {
  pinRow.hidden = true;
  saving = null;
  setNote('');
}
async function sendPin() {
  pinOk.disabled = true;
  const r = await unlock(pinIn.value);
  if (r.superseded) return;
  pinOk.disabled = false;
  if (r.stale) return;
  if (!r.ok) { setNote(r.err, true); pinIn.select(); return; }
  pinRow.hidden = true;
  saving?.run();
}
pinIn.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); sendPin(); }
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); cancelPin(); }
});

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
  pinRow.hidden = true;
  // A save already sent finishes first (its answer locks up after it).
  if (!saving?.id) { saving = null; relock(); }
  if (back?.isConnected) back.focus();
}

// Keys stay in the dialog: Esc closes, Tab wraps.
root.addEventListener('keydown', (e) => {
  e.stopPropagation();
  if (e.key === 'Escape') { e.preventDefault(); closeSalvos(); return; }
  if (e.key !== 'Tab') return;
  const all = [...dialog.querySelectorAll('button, input, select')].filter((x) => !x.disabled && x.offsetParent);
  const first = all[0], last = all[all.length - 1];
  if (e.shiftKey && (document.activeElement === first || document.activeElement === dialog)) { e.preventDefault(); last.focus(); }
  else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
});
