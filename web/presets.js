// Presets in the page: the buttons above Recent (both layouts) and the
// editor dialog. Pressing a button builds a plan and the layout showing opens
// its normal confirm; the confirm sends one batch. Storage and the pure
// parts are presetstore.js, the plan is plans.js.

import { el, S, on, emit, off, search, srcItem, dstItem, nameProblem } from './core.js';
import { levelInput, parseLevels } from './levelview.js';
import { showResult, presetPlan } from './plans.js';
import { globalSaver } from './globalsave.js';
import { profileSaver } from './profilesaver.js';
import { profileName } from './profile.js';
import { trapDialog } from './widgets.js';
import { shareRow, shareLink, sharePresetDoc } from './transfer.js';
import { saveAsName } from './exportfmt.js';
import {
  allPresets, globalPresets, userPresets, localPresets, PRESET_SCOPES, findPreset, normPreset, presetProblem, presetsProblem, replacePreset, saveLocalPreset, deleteLocalPreset,
  pinnedPresets, saved,
  parseRefs, itemsFromTakes, latestFolderTakes,
} from './presetstore.js';

// ---- the layouts: each registers once what it has

const hosts = [];
// host: { active(), confirm(plan), patch() }. patch() gives the patch
// boxes as { src: item, dsts: [item], levels: [ui] } (levels [] = the
// default levels), or null while they hold no patch.
export const presetHost = (host) => { hosts.push(host); };
const shown = () => hosts.find((h) => h.active());
export const presetShown = () => !!shown();

// ---- the buttons

export function fire(scope, name) {
  const s = findPreset(scope, name);
  const host = shown();
  if (!s || !host) return;
  if (!S.srcBy.size) return showResult('warn', 'Preset not sent', 'The router’s names have not loaded yet.');
  const plan = presetPlan(s, parseLevels('') || []);
  if (!plan.items.length) {
    return showResult('error', `Preset ${s.name} not sent`, plan.warnings.length ? plan.warnings.join('; ') + '.' : 'It has no takes.');
  }
  host.confirm(plan);
}

function summary(s) {
  const n = s.items.reduce((a, it) => a + it.dsts.length, 0);
  return `${s.items.length} source${s.items.length === 1 ? '' : 's'}, ${n} destination${n === 1 ? '' : 's'}`;
}

const phone = () => document.documentElement.dataset.pbLayout === 'phone';
const askPopup = () => emit('presetPopup');

function renderStrips() {
  const all = allPresets();
  const list = pinnedPresets(all);
  for (const box of document.querySelectorAll('[data-presets]')) {
    const edit = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', all.length ? 'Edit' : 'Presets…');
    edit.type = 'button';
    edit.setAttribute('aria-haspopup', 'dialog');
    edit.addEventListener('click', () => openPresets());
    const chips = list.map((s) => {
      const b = el('button', 'pb-chip sode__set', s.name);
      b.type = 'button';
      b.title = `${summary(s)} · ${PRESET_SCOPES[s.scope]}`;
      b.addEventListener('click', () => fire(s.scope, s.name));
      return b;
    });
    // The popup (presetpopup.js) lists every preset, pinned or not.
    const more = all.length ? el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__presets-all', 'Presets') : null;
    if (more) {
      more.type = 'button';
      more.title = phone() ? 'All presets' : 'All presets (Ctrl+G)';
      more.setAttribute('aria-haspopup', 'dialog');
      more.addEventListener('click', askPopup);
    }
    box.replaceChildren(...[more, ...chips, edit].filter(Boolean));
  }
}
// Keyboard mode `p` (desktop): the first chip, then ←/→ between buttons;
// Enter presses one (the normal confirm), Esc goes back to Source.
on('presetStrip', () => {
  const b = document.querySelector('[data-presets] .sode__set') || document.querySelector('[data-presets] button');
  b?.focus();
});
document.addEventListener('keydown', (e) => {
  const box = e.target instanceof Element && e.target.closest('[data-presets]');
  if (!box || e.ctrlKey || e.metaKey || e.altKey) return;
  const bs = [...box.querySelectorAll('button')];
  const i = bs.indexOf(e.target);
  if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
    e.preventDefault();
    bs[(i + (e.key === 'ArrowRight' ? 1 : bs.length - 1)) % bs.length].focus();
  } else if (e.key === 'Escape') {
    e.preventDefault();
    document.getElementById('src')?.focus();
  }
});
on('presets', () => { renderStrips(); if (open) renderEditor(true); });
renderStrips();

// ---- the dialog

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__filters sode__presets-dlg');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'presets-title');
dialog.tabIndex = -1;

const pick = el('select', 'pb-select');
pick.setAttribute('aria-label', 'Preset');
const btn = (cls, text, fn = () => {}) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};
const nameIn = el('input', 'pb-input');
nameIn.maxLength = 32;
nameIn.placeholder = 'Name';
nameIn.setAttribute('aria-label', 'Preset name');
nameIn.autocomplete = 'off';
const scopeIn = el('select', 'pb-select');
scopeIn.setAttribute('aria-label', 'Save for');
// 'Me' is added while someone is logged in (see renderHead).
const scopeOpts = () => [new Option('This browser', 'local'), new Option('Everyone', 'global'),
  ...(profileName() ? [new Option('Me', 'user')] : [])];
scopeIn.append(...scopeOpts());
const pinnedIn = el('input', 'pb-check__input');
pinnedIn.type = 'checkbox';
pinnedIn.checked = true;
const pinnedBox = el('label', 'pb-check', pinnedIn, el('span', 'pb-check__label', 'Pinned (show above Recent)'));
const rowsBox = el('div', 'sode__preset-rows');
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
// A link to the saved preset (names ride along; opening it shows a review).
const shareUI = shareRow();
const saveAsBtn = btn('pb-btn--outline pb-btn--sm', 'Save as…', () => askSaveAs());
// Save as: the preset as typed, under another name, in the scope chosen here.
const asName = el('input', 'pb-input');
asName.maxLength = 32;
asName.placeholder = 'Name';
asName.setAttribute('aria-label', 'Name for the copy');
asName.autocomplete = 'off';
const asScope = el('select', 'pb-select');
asScope.setAttribute('aria-label', 'Save the copy for');
const asRow = el('div', 'sode__filters-set', asName, el('span', 'pb-select-wrap sode__filters-scope', asScope),
  btn('pb-btn--commit pb-btn--sm', 'Save', () => saveAs()), btn('pb-btn--ghost pb-btn--sm', 'Cancel', () => closeSaveAs()));
asRow.hidden = true;
const shareBtn = btn('pb-btn--outline pb-btn--sm', 'Share', () => {
  const p = cur?.old && findPreset(cur.scope, cur.old);
  if (p) shareLink(sharePresetDoc(p), shareUI);
});

dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Presets'), { id: 'presets-title' })),
  el('div', 'pb-modal__body sode__filters-body',
    el('div', 'sode__filters-sets',
      el('div', 'sode__filters-set', el('span', 'pb-select-wrap', pick), btn('pb-btn--outline pb-btn--sm', 'New', () => load(null))),
      el('div', 'sode__filters-set', nameIn, el('span', 'pb-select-wrap sode__filters-scope', scopeIn), saveBtn, saveAsBtn, shareBtn, delBtn),
      asRow, pinnedBox, pinRow, noteEl, shareUI.box),
    rowsBox,
    el('div', 'sode__filters-set',
      btn('pb-btn--outline pb-btn--sm', 'Add take', () => { cur.items.push(blankRow()); renderRows(); focusLast(); }),
      btn('pb-btn--outline pb-btn--sm', 'Fill from patch boxes', fillFromBoxes),
      btn('pb-btn--outline pb-btn--sm', 'Fill from latest folder in Recent', fillFromHistory)),
    el('span', 'sode__hint', 'Numbers (12, 20-24) or names, separated by commas.')),
  el('div', 'pb-modal__footer',
    btn('pb-btn--outline', 'Export / import…', () => emit('openTransfer', { scope: cur?.old ? cur.scope : 'local', user: profileName() || '', what: 'presets' })),
    btn('pb-btn--ok', 'Done', () => closePresets())),
);
root.append(dialog);
document.body.append(root);

function setNote(text, error = false) {
  noteEl.textContent = text;
  noteEl.classList.toggle('sode__error-text', error);
}

// Presets for everyone: the whole list is sent, after the PIN if one is set.
const saver = globalSaver({
  t: 'presets', resultT: 'presetsResult', pinIn, pinRow, pinOk, setNote,
  isOpen: () => open, failTitle: 'Presets not saved', okNote: 'Saved for everyone.',
  render: () => renderHead(true),
});
// Presets of the logged-in user ('Me'): their profile, no PIN row.
const profSaver = profileSaver({
  setNote, isOpen: () => open, failTitle: 'Presets not saved', okNote: 'Saved to your profile.',
  render: () => renderHead(true),
});
const listFor = (scope) => (scope === 'global' ? globalPresets() : scope === 'user' ? userPresets() : localPresets());

let open = false, back = null;
// cur: the preset being edited: { scope, old, name, items: [{ src, dsts, levels }] } as typed text.
let cur = null, dirty = false, deleting = false, discard = false;
// An edit: unsaved, and a close warning given earlier no longer covers it.
const touch = () => { dirty = true; discard = false; };
const blankRow = () => ({ src: '', dsts: '', levels: '' });

const display = (n) => String(n + off());
function rowFromItem(it) {
  return { src: display(it.src), dsts: it.dsts.map(display).join(', '), levels: it.levels.length ? levelInput(it.levels) : '' };
}
function load(s) {
  shareUI.box.hidden = true;
  asRow.hidden = true;
  deleting = false;
  dirty = false;
  discard = false;
  cur = s
    ? { scope: s.scope, old: s.name, name: s.name, pinned: s.pinned !== false, items: s.items.map(rowFromItem) }
    : { scope: 'local', old: '', name: '', pinned: true, items: [blankRow()] };
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
    const hint = el('span', 'sode__hint sode__preset-hint');
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
      inp.addEventListener('input', () => { r[key] = inp.value; touch(); upd(); setNote(''); });
      return inp;
    };
    upd();
    return el('div', 'sode__preset-row',
      field('src', 'Source', `Take ${i + 1} source`),
      field('dsts', 'Destinations', `Take ${i + 1} destinations`),
      field('levels', S.levels.length ? `Levels (${S.levels[0].label}-${S.levels[S.levels.length - 1].label})` : 'Levels', `Take ${i + 1} levels`),
      btn('pb-btn--ghost pb-btn--sm', 'Remove', () => { cur.items.splice(i, 1); touch(); renderRows(); }),
      hint);
  }));
}
function focusLast() {
  const ins = rowsBox.querySelectorAll('.sode__preset-row:last-child input');
  ins[0]?.focus();
}

// The preset picker and the buttons around the name. keep: what is being
// typed (the name) stays as it is.
function renderHead(keep = false) {
  pick.replaceChildren(new Option('New preset…', ''), ...allPresets().map((s) => new Option(`${s.name} (${PRESET_SCOPES[s.scope]})`, `${s.scope}:${s.name}`)));
  pick.value = cur.old ? `${cur.scope}:${cur.old}` : '';
  if (!keep) nameIn.value = cur.name;
  // Rebuilt so that 'Me' follows who is logged in; a Me preset of someone else is gone from the list anyway.
  scopeIn.replaceChildren(...scopeOpts());
  scopeIn.value = [...scopeIn.options].some((o) => o.value === cur.scope) ? cur.scope : 'local';
  cur.scope = scopeIn.value;
  pinnedIn.checked = cur.pinned;
  scopeIn.disabled = !!cur.old;
  delBtn.hidden = !cur.old;
  shareBtn.hidden = !cur.old;
  delBtn.textContent = deleting ? 'Really delete?' : 'Delete';
  saveBtn.disabled = saver.busy() || profSaver.busy();
}

// keepEdit: the lists changed under us (a reconnect, another browser's
// save); only the picker is refreshed, never the rows or name being typed.
function renderEditor(keepEdit = false) {
  if (keepEdit && cur?.old && !findPreset(cur.scope, cur.old) && !dirty) return load(null);
  renderHead(keepEdit);
  if (keepEdit) return;
  renderRows();
  setNote(cur.old ? PRESET_SCOPES[cur.scope] : '');
}
pick.addEventListener('change', () => {
  const v = pick.value;
  if (!v) return load(null);
  const i = v.indexOf(':');
  load(findPreset(v.slice(0, i), v.slice(i + 1)));
});
nameIn.addEventListener('input', () => { cur.name = nameIn.value; touch(); setNote(''); });
pinnedIn.addEventListener('change', () => { cur.pinned = pinnedIn.checked; touch(); });
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
  touch();
  renderRows();
  setNote(note);
}

// ---- saving

// The typed rows as a preset, or an error line. next: the scope's list as
// it would be saved.
function built() {
  const items = [];
  for (const r of cur.items) {
    const p = parseRow(r);
    if (p.error) return { err: p.text };
    items.push(p.item);
  }
  const s = { ...normPreset({ name: cur.name, items }), pinned: cur.pinned };
  const list = listFor(cur.scope);
  const err = nameProblem(s.name, list.map((x) => x.name), { noun: 'preset', old: cur.old }) || presetProblem(s);
  if (err) return { err };
  const next = replacePreset(list, s, cur.old);
  const tooMuch = presetsProblem(next);
  return tooMuch ? { err: tooMuch } : { s, next };
}

function save() {
  const b = built();
  if (b.err) return setNote(b.err, true);
  if (cur.scope === 'local') {
    saveLocalPreset(b.s, cur.old);
    cur.old = b.s.name;
    cur.name = b.s.name;
    dirty = false;
    renderEditor();
    return setNote('Saved for this browser.');
  }
  const sv = cur.scope === 'user' ? profSaver : saver;
  sv.change({ presets: b.next }, () => { cur.old = b.s.name; cur.name = b.s.name; nameIn.value = cur.name; dirty = false; });
}

// ---- save as

function askSaveAs() {
  asScope.replaceChildren(...scopeOpts());
  asScope.value = 'local';
  asName.value = saveAsName(cur.name, cur.old ? cur.scope : '');
  asName.classList.remove('pb-input--invalid');
  asRow.hidden = false;
  asName.select();
}
function closeSaveAs() {
  asRow.hidden = true;
  saveAsBtn.focus();
}
// The name is checked against the target scope; a name taken there is replaced.
function saveAs() {
  const name = asName.value.trim().slice(0, 32);
  const target = asScope.value;
  const bad = (msg) => { asName.classList.add('pb-input--invalid'); setNote(msg, true); asName.focus(); };
  const items = [];
  for (const r of cur.items) {
    const p = parseRow(r);
    if (p.error) return bad(p.text);
    items.push(p.item);
  }
  const err = nameProblem(name, [], { noun: 'preset' });
  if (err) return bad(err);
  const s = { ...normPreset({ name, items }), pinned: cur.pinned };
  const list = listFor(target);
  const taken = list.find((x) => x.name.toLowerCase() === name.toLowerCase())?.name || '';
  const perr = presetProblem(s);
  if (perr) return bad(perr);
  const next = replacePreset(list, s, taken);
  const tooMuch = presetsProblem(next);
  if (tooMuch) return bad(tooMuch);
  closeSaveAs();
  const loaded = () => { cur.scope = target; cur.old = s.name; cur.name = s.name; dirty = false; discard = false; renderHead(); };
  if (target === 'local') {
    saveLocalPreset(s, taken);
    loaded();
    return setNote('Saved for this browser.');
  }
  (target === 'user' ? profSaver : saver).change({ presets: next }, loaded);
}
asName.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); saveAs(); }
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeSaveAs(); }
});

function deleteClick() {
  if (!cur.old) return;
  if (!deleting) { deleting = true; delBtn.textContent = 'Really delete?'; return; }
  deleting = false;
  const name = cur.old;
  if (cur.scope === 'local') {
    deleteLocalPreset(name);
    load(null);
    return setNote('Deleted.');
  }
  const next = listFor(cur.scope).filter((s) => s.name.toLowerCase() !== name.toLowerCase()).map(saved);
  (cur.scope === 'user' ? profSaver : saver).change({ presets: next }, () => load(null));
}

// ---- open / close


export function openPresets(scope = '', name = '') {
  back = document.activeElement;
  const s = scope && findPreset(scope, name);
  open = true;
  pinRow.hidden = true;
  load(s || null);
  root.hidden = false;
  dialog.focus();
}

function closePresets() {
  if (open && dirty && !discard) { discard = true; return setNote('Unsaved changes. Close again to discard them, or Save.', true); }
  open = false;
  root.hidden = true;
  profSaver.close();
  saver.close(); // a save already sent finishes first (its answer locks up after it)
  if (back?.isConnected) back.focus();
}

trapDialog(root, dialog, closePresets);
