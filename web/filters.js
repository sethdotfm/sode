// Recent's Filters dialog, shared by both layouts: which kinds of entry
// show, and filter expressions for sources and destinations, edited as
// rule rows (Visual) or typed (Code). Everything applies as you type and
// is kept per browser (filterstore.js). Named sets (global from Settings, or this
// browser's own) are quick buttons in Recent's header. Share link puts the
// filter in a URL (transfer.js reads it back through a review sheet).

import { $, el, S, on, emit, off, search, item, labelNode, nameProblem } from './core.js';
import { readStr, writeStr } from './store.js';
import {
  filterState, setFilter, filterCount, allSets, activeSet, adoptFilter, pickSet,
  saveLocalSet, deleteLocalSet, renameLocalSet, setsIn, setRef, filterSummary, toggleSet, activeStack, isActive,
} from './filterstore.js';
import { pref } from './device.js';
import { PREFS } from './prefs.js';
import { histRows, setSearch, searchActive, searchRespectsFilters } from './history.js';
import { shareLink, copyLink } from './transfer.js';
import { filterShare, saveAsName } from './exportfmt.js';
import { scopeIds, scopeLabel, scopeOptions } from './scopes.js';
import { parseFilter, filterText, compileFilter } from './filter.js';
import { globalSaver, profileSaver } from './globalsave.js';
import { profileName, editTarget } from './profile.js';
import { FILTER_DEFAULT, normFilter, snapshot, restore } from './filterstore.js';
import { upsertEntry, filterEntry } from './profilesrules.js';
import { combobox, trapDialog, labeled } from './widgets.js';

const SCOPE_GROUPS = scopeIds(true).map((id) => [id, scopeLabel(id)]); // Default | Everyone | Me | This browser
const root = $('filters');
const dialog = $('filters-dialog');
const MODE_KEY = 'sode-filter-mode';
let mode = readStr(MODE_KEY) === 'code' ? 'code' : 'visual';

const KINDS = [
  ['name', 'Name contains'],
  ['exact', 'Name is'],
  ['num', 'Number is'],
  ['range', 'Number between'],
  ['re', 'Name matches regex'],
];

// ---- aimed at a profile (bus 'openFilterEditor', from the Profile dialog)
// The same dialog edits one filter set of a profile: the filter is a draft
// held here (never the browser's own filter or active set), the set bar is
// replaced by a name and Save, and a save goes to that user's profile.

let tgt = null; // editTarget(...) plus f: the draft
const curF = () => (tgt ? tgt.f : filterState());
const putF = (next) => { if (tgt) tgt.f = normFilter(next); else setFilter(next); };

// Whether a visual row says enough to be a rule.
function complete(r) {
  if (r.kind === 'num') return Number.isInteger(r.a);
  if (r.kind === 'range') return Number.isInteger(r.a) && Number.isInteger(r.b);
  if (r.kind === 're') {
    if (!r.text) return false;
    try { new RegExp(r.text, 'i'); return true; } catch (e) { return false; }
  }
  return !!r.text;
}

// One expression editor: key is 'src' or 'dst'.
function editor(key) {
  const noun = key === 'src' ? 'sources' : 'destinations';
  const box = $(`flt-${key}`);
  const ed = { text: '', rows: [], error: null };
  let timer = 0;

  // The expression changed: apply it if it parses, and redraw the preview.
  function changed(now = false) {
    const p = parseFilter(ed.text);
    ed.error = p.error ? `${p.error}${p.at != null ? ` at “${ed.text.slice(p.at, p.at + 12)}”` : ''}` : null;
    clearTimeout(timer);
    const go = () => {
      if (!ed.error) putF({ ...curF(), [key]: ed.text });
      preview();
    };
    if (now) go();
    else timer = setTimeout(go, 150);
    if (ed.error) preview();
  }

  function preview() {
    const note = box.querySelector('.sode__filters-preview');
    const err = box.querySelector('.pb-note');
    const input = box.querySelector('.sode__filters-code');
    input?.classList.toggle('pb-input--invalid', !!ed.error);
    err.hidden = !ed.error;
    err.textContent = ed.error ? `${ed.error}` : '';
    const list = key === 'src' ? S.srcs : S.dsts;
    const p = parseFilter(ed.text);
    if (p.error) return;
    if (!p.rules.length) { note.textContent = `All ${noun}`; return; }
    if (!list.length) { note.textContent = ''; return; }
    const fn = compileFilter(p.rules);
    const hits = list.filter((i) => fn(i.n + off(), i.name));
    const shown = hits.slice(0, 4).map((i) => i.label).join(', ');
    note.textContent = hits.length
      ? `${hits.length} of ${list.length}: ${shown}${hits.length > 4 ? ', …' : ''}`
      : `No ${noun} match`;
  }

  // Rows → text, keeping rows still being filled in out of it.
  function fromRows() {
    ed.text = filterText(ed.rows.filter(complete));
    changed();
  }

  function rowNode(r, i) {
    const sel = (opts, value, label, onChange) => {
      const s = el('select', 'pb-select');
      for (const [v, t] of opts) {
        const o = el('option', null, t);
        o.value = v;
        s.append(o);
      }
      s.value = value;
      s.setAttribute('aria-label', label);
      s.addEventListener('change', () => onChange(s.value));
      return el('span', 'pb-select-wrap', s);
    };
    const input = (value, label, onInput, numeric) => {
      const f = el('input', 'pb-input');
      f.value = value ?? '';
      f.spellcheck = false;
      f.autocomplete = 'off';
      if (numeric) f.inputMode = 'numeric';
      f.setAttribute('aria-label', label);
      f.addEventListener('input', () => {
        onInput(f.value);
        for (const x of f.closest('.sode__filters-rule').querySelectorAll('input')) {
          x.classList.toggle('pb-input--invalid', x.value.trim() !== '' && !complete(r));
        }
        fromRows();
      });
      return f;
    };
    const num = (v) => (/^\s*\d+\s*$/.test(v) ? Number(v) : NaN);
    const n = i + 1;
    const values = r.kind === 'range'
      ? [input(r.a, `Rule ${n} from`, (v) => { r.a = num(v); }, true), el('span', 'sode__hint', 'to'),
        input(r.b, `Rule ${n} to`, (v) => { r.b = num(v); }, true)]
      : r.kind === 'num' ? [input(r.a, `Rule ${n} number`, (v) => { r.a = num(v); }, true)]
      : [input(r.text, `Rule ${n} ${r.kind === 're' ? 'regex' : 'name'}`, (v) => { r.text = v; delete r.quoted; })];
    const remove = el('button', 'pb-btn pb-btn--ghost', 'Remove');
    remove.type = 'button';
    remove.setAttribute('aria-label', `Remove rule ${n}`);
    remove.addEventListener('click', () => {
      ed.rows.splice(i, 1);
      fromRows();
      render();
      box.querySelector('.sode__filters-add')?.focus();
    });
    return el('div', 'sode__filters-rule',
      sel([['in', 'Include'], ['out', 'Exclude']], r.not ? 'out' : 'in', `Rule ${n}: include or exclude`, (v) => { r.not = v === 'out'; fromRows(); }),
      sel(KINDS, r.kind, `Rule ${n}: what to match`, (v) => {
        // Keep what was typed where it still makes sense.
        if ((v === 'num' || v === 'range') !== (r.kind === 'num' || r.kind === 'range')) Object.assign(r, { a: undefined, b: undefined, text: '' });
        r.kind = v;
        fromRows();
        render();
        box.querySelectorAll('.sode__filters-rule')[i]?.querySelector('input')?.focus();
      }),
      el('span', 'sode__filters-vals', ...values),
      remove);
  }

  function render() {
    const head = el('div', 'sode__filters-ed-head', el('span', 'sode__label', key === 'src' ? 'Sources' : 'Destinations'));
    let body;
    if (mode === 'code') {
      const f = el('input', 'pb-input sode__code sode__filters-code');
      f.value = ed.text;
      f.placeholder = key === 'src' ? 'CAM, 1-12, !Black' : 'DSM, /^MV\\d+ PIP/, !65';
      f.spellcheck = false;
      f.autocomplete = 'off';
      f.setAttribute('aria-label', `${key === 'src' ? 'Sources' : 'Destinations'} filter`);
      f.addEventListener('input', () => { ed.text = f.value; changed(); });
      body = f;
    } else {
      const add = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__filters-add', '+ Add rule');
      add.type = 'button';
      add.addEventListener('click', () => {
        ed.rows.push({ not: false, kind: 'name', text: '' });
        render();
        const rules = box.querySelectorAll('.sode__filters-rule');
        rules[rules.length - 1]?.querySelector('input')?.focus();
      });
      body = el('div', 'sode__filters-rules',
        ...ed.rows.map(rowNode),
        el('div', null, add));
    }
    const err = el('div', 'pb-note pb-note--error');
    err.setAttribute('role', 'alert');
    err.hidden = true;
    box.replaceChildren(head, body, el('div', 'sode__hint sode__filters-preview'), err);
    preview();
  }

  return {
    // From the saved filter (opening, Clear all).
    load(text) {
      ed.text = text;
      const p = parseFilter(text);
      ed.rows = p.rules ? p.rules.map((r) => ({ ...r })) : [];
      ed.error = null;
      render();
      if (p.error) changed(true);
    },
    // Switching modes; false if the code doesn't parse, so can't be rows.
    toVisual() {
      const p = parseFilter(ed.text);
      if (p.error) { changed(true); return false; }
      ed.rows = p.rules.map((r) => ({ ...r }));
      return true;
    },
    render,
    preview,
    flush() {
      clearTimeout(timer);
      if (!ed.error) putF({ ...curF(), [key]: ed.text });
      preview();
    },
  };
}

const eds = { src: editor('src'), dst: editor('dst') };

function renderMode() {
  $('flt-visual').setAttribute('aria-pressed', String(mode === 'visual'));
  $('flt-code').setAttribute('aria-pressed', String(mode === 'code'));
  $('flt-syntax').hidden = mode !== 'code';
}
function setMode(m) {
  if (m === mode) return;
  if (m === 'visual' && !['src', 'dst'].map((k) => eds[k].toVisual()).every(Boolean)) return;
  mode = m;
  writeStr(MODE_KEY, m);
  renderMode();
  eds.src.render();
  eds.dst.render();
}

function renderShow() {
  const f = curF();
  $('flt-sode').checked = f.sode;
  $('flt-ext').checked = f.ext;
  $('flt-hidden').checked = f.hidden;
  $('flt-hidden').disabled = !f.ext;
  const g = (S.cfg?.hiddenDsts || '').trim();
  $('flt-hidden-note').replaceChildren(...(g ? [el('code', null, g)] : ['None set in Settings']));
}

// ---- the Filters buttons in Recent's headers

// What the Filters button says: stacked sets, else how many parts differ
// from the default.
function buttonState() {
  const sets = activeStack().length;
  const parts = filterCount();
  if (sets) return { text: `Filters · ${sets} sets`, aria: `Filters, ${sets} sets stacked`, on: true };
  if (parts) return { text: `Filters · ${parts}`, aria: `Filters, ${parts} active`, on: true };
  return { text: 'Filters', aria: 'Filters, none active', on: false };
}

function renderButtons() {
  const { text, aria, on: lit } = buttonState();
  for (const b of document.querySelectorAll('[data-filters]')) {
    b.textContent = text;
    b.classList.toggle('sode__filters-btn--on', lit);
    b.setAttribute('aria-label', aria);
  }
  // Quick buttons for the sets, once there's any besides All. Active ones
  // are selected (click again to take one off; none left is All); * when
  // a single one's filter has been edited since.
  const sets = allSets();
  const a = activeSet();
  const chip = (s) => {
    const on = s.scope === 'all' ? !!a && a.scope === 'all' : isActive(s.scope, s.name);
    const edited = on && !!a && a.scope !== 'all' && a.edited;
    const b = el('button', 'pb-chip sode__set', s.name + (edited ? '*' : ''));
    b.type = 'button';
    b.setAttribute('aria-pressed', String(on));
    if (edited) b.setAttribute('aria-label', `${s.name} (edited)`);
    b.dataset.pbTip = filterSummary(s.f) + (edited ? ' (edited)' : '');
    b.classList.add('pb-tip--end');
    b.addEventListener('click', () => {
      toggleSet(s.scope, s.name);
      if (filtersOpen()) reload();
    });
    return b;
  };
  // All | for everyone | this browser's own, each group labelled for
  // screen readers.
  const groups = SCOPE_GROUPS
    .map(([scope, label]) => [label, sets.filter((s) => s.scope === scope)]).filter(([, g]) => g.length);
  for (const box of document.querySelectorAll('[data-sets]')) {
    const kids = [];
    if (sets.length > 1) {
      for (const [label, g] of groups) {
        if (kids.length) {
          const bar = el('span', 'sode__sets-sep', '|');
          bar.setAttribute('aria-hidden', 'true');
          kids.push(bar);
        }
        const grp = el('span', 'sode__sets-group', ...g.map(chip));
        grp.setAttribute('role', 'group');
        grp.setAttribute('aria-label', label);
        kids.push(grp);
      }
    }
    // The first group (All) stays put, with its divider; the rest scroll.
    const [pinned, sep, ...rest] = kids;
    box.replaceChildren(...(pinned ? [pinned, sep, el('div', 'sode__sets-run', ...rest)] : []));
  }
  if (filtersOpen()) renderSetBar();
}

// ---- open / close

let back = null;
// The filter and set as they were when the dialog opened: Cancel puts them back
// (edits apply to Recent as they are made).
let openSnap = null;
export const filtersOpen = () => !root.hidden;

export function openFilters() {
  back = document.activeElement;
  tgt = null;
  openSnap = snapshot();
  chrome();
  const f = filterState();
  renderMode();
  renderShow();
  eds.src.load(f.src);
  eds.dst.load(f.dst);
  $('flt-share').hidden = true;
  $('flt-set-name-row').hidden = true;
  $('flt-pin-row').hidden = true;
  naming = false;
  deleting = false;
  renderSetBar();
  dlg.show();
  dialog.focus();
}

// A set write has landed (the server confirmed it, or it is this browser's):
// Cancel must not roll the filter back past it.
function landed() {
  if (filtersOpen() && !tgt) openSnap = snapshot();
}

// Cancel, Esc and a click outside: drop what was changed here, then close.
function cancelFilters() {
  if (tgt || !openSnap) return closeFilters();
  // Settle the editors' pending text first (it would otherwise be written
  // after the restore), put the filter back, then close without flushing.
  flushEds();
  restore(openSnap);
  closeFilters(false);
}

// flush: write the editors' pending text into the filter first (Done).
function closeFilters(flush = true) {
  if (flush) flushEds();
  dlg.hide();
  if (tgt) { // an editor aimed at a profile: nothing global to finish
    tgt = null;
    chrome();
    if (back?.isConnected) back.focus();
    return;
  }
  // A save already sent finishes first (its answer locks up after it);
  // one still waiting for the PIN is dropped.
  saver.close();
  if (back?.isConnected) back.focus();
}

const tName = el('input', 'pb-input');
tName.maxLength = 32;
tName.placeholder = 'Name';
tName.autocomplete = 'off';
tName.setAttribute('aria-label', 'Set name');
const tNote = el('span', 'sode__hint');
tNote.setAttribute('role', 'status');
tNote.setAttribute('aria-live', 'polite');
const tSave = el('button', 'pb-btn pb-btn--commit', 'Save');
tSave.type = 'button';
const tBar = el('div', 'sode__filters-set', labeled('Name', tName), tNote);
tBar.hidden = true;
$('flt-set').closest('.sode__filters-sets').before(tBar);
$('flt-done').after(tSave);
tSave.hidden = true;
const tNoteSet = (text, error = false) => {
  tNote.textContent = text;
  tNote.classList.toggle('sode__error-text', error);
};

// The dialog's look for the editor kind: target set or the usual one.
function chrome() {
  const t = !!tgt;
  $('flt-set').closest('.sode__filters-sets').hidden = t;
  tBar.hidden = !t;
  tSave.hidden = !t;
  $('flt-save').hidden = t;
  $('flt-clear').hidden = t;
  $('flt-share-btn').hidden = t;
  $('filters-title').textContent = t ? `${tgt.old ? 'Edit' : 'Add'} filter set for ${tgt.user}` : 'Filters';
  if (t) { tName.value = tgt.old; tNoteSet(''); tName.classList.remove('pb-input--invalid'); }
}

const tgtSaver = profileSaver({
  target: () => tgt,
  setNote: tNoteSet, isOpen: () => filtersOpen() && !!tgt, failTitle: 'Filter set not saved', render: () => { tSave.disabled = tgtSaver.busy(); },
});

function saveTarget() {
  if (!tgt) return;
  eds.src.flush();
  eds.dst.flush();
  const name = tName.value.trim();
  const bad = nameProblem(name, tgt.lists.map((x) => String(x?.name)), { noun: 'filter set', old: tgt.old, reserved: 'all' });
  if (bad) { tName.classList.add('pb-input--invalid'); tNoteSet(bad, true); tName.focus(); return; }
  tName.classList.remove('pb-input--invalid');
  const list = upsertEntry(tgt.lists, filterEntry(name, tgt.f), tgt.old);
  tgtSaver.change({ filterSets: list }, closeFilters);
}
tSave.addEventListener('click', saveTarget);
tName.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); saveTarget(); }
});

on('openFilterEditor', (m) => {
  const t = editTarget(m, 'filterSets');
  if (!t) return;
  back = document.activeElement;
  tgt = { ...t, f: normFilter(t.found || FILTER_DEFAULT) };
  chrome();
  renderMode();
  renderShow();
  eds.src.load(tgt.f.src);
  eds.dst.load(tgt.f.dst);
  $('flt-share').hidden = true;
  dlg.show();
  dialog.focus();
});

for (const b of document.querySelectorAll('[data-filters]')) b.addEventListener('click', openFilters);
$('flt-done').addEventListener('click', cancelFilters);
// Save writes the active set if it was edited, renamed or moved, then closes
// once the write has landed (for everyone or Me that is the server's answer;
// it stays open for a PIN or an error). With nothing to write the filter as
// shown simply stays.
$('flt-save').addEventListener('click', () => saveSet(() => { if (filtersOpen()) closeFilters(); }));
$('flt-visual').addEventListener('click', () => setMode('visual'));
$('flt-code').addEventListener('click', () => setMode('code'));
for (const k of ['sode', 'ext', 'hidden']) {
  $(`flt-${k}`).addEventListener('change', (e) => {
    putF({ ...curF(), [k]: e.target.checked });
    renderShow();
  });
}
// Loads the editors from the filter now (after a set is picked).
function reload() {
  const f = curF();
  renderShow();
  eds.src.load(f.src);
  eds.dst.load(f.dst);
  $('flt-share').hidden = true;
}

$('flt-clear').addEventListener('click', () => {
  pickSet('all', 'All');
  reload();
});

// ---- the set bar: pick, name, save, save as, delete. As in the preset
// editor, a saved set is renamed by editing its name and pressing Save. Works the same on
// this browser's sets and on sets for everyone; those are settings, saved
// on the server, so with a settings PIN they ask for it first.

const setKey = (s) => `${s.scope}:${s.name}`;
let deleting = false; // Delete was pressed once; the next press deletes
let naming = false;   // the Save as name row is open
let nameDirty = false; // the name box was edited (it keeps what is typed across redraws)
let moveTo = null;     // the scope chosen for the open set: Save moves it there

function renderSetBar() {
  const a = activeSet();
  const stack = activeStack();
  const select = $('flt-set');
  const groups = SCOPE_GROUPS;
  const sets = allSets();
  const opts = [];
  if (!a) {
    const o = el('option', null, stack.length ? stack.map((s) => s.name).join(' + ') : 'Unsaved');
    o.value = '';
    opts.push(o);
  }
  for (const [scope, label] of groups) {
    const these = sets.filter((s) => s.scope === scope);
    if (!these.length) continue;
    const g = el('optgroup');
    g.label = label;
    for (const s of these) {
      const o = el('option', null, s.name + (a && setKey(a) === setKey(s) && a.edited ? ' (edited)' : ''));
      o.value = setKey(s);
      g.append(o);
    }
    opts.push(g);
  }
  select.replaceChildren(...opts);
  select.value = a ? setKey(a) : '';
  const named = a && a.scope !== 'all';
  const busy = saver.busy() || profSaver.busy();
  const cur = $('flt-set-cur');
  if (!named) { nameDirty = false; cur.value = ''; } else if (!nameDirty) cur.value = a.name;
  cur.disabled = busy || !named;
  // Where the set lives; another choice is a move, made by Save (copy first, then remove).
  const scopeBox = $('flt-set-cur-scope');
  scopeBox.replaceChildren(...scopeOptions(!!profileName()).map(([id, label]) => new Option(label, id)));
  if (named && ![...scopeBox.options].some((o) => o.value === moveTo)) moveTo = null;
  if (!named) moveTo = null;
  scopeBox.value = moveTo || (named && [...scopeBox.options].some((o) => o.value === a.scope) ? a.scope : 'local');
  scopeBox.disabled = busy || !named;
  $('flt-set-saveas').disabled = busy || stack.length > 0;
  $('flt-share-btn').disabled = stack.length > 0;
  $('flt-set-del').disabled = busy || !named;
  $('flt-set-del').textContent = deleting && named ? `Delete “${a.name}”?` : 'Delete';
  $('flt-set-del').classList.toggle('pb-btn--danger', deleting && named);
  if (busy) return;
  if (stack.length) {
    return setNote(`Stacked: ${stack.map((s) => s.name).join(', ')} · ${PREFS.setStack.values[pref('setStack')]}. `
      + 'Changing a filter leaves the stack, and the share link is off.');
  }
  if (named && moveTo) return setNote(`Save moves it from ${scopeLabel(a.scope)} to ${scopeLabel(moveTo)}.`);
  setNote(a && a.scope !== 'all' ? `${scopeLabel(a.scope)}${a.edited ? ' · edited' : ''}` : '');
}
const setNote = (text, error = false) => {
  $('flt-set-note').textContent = text;
  $('flt-set-note').classList.toggle('sode__error-text', error);
};

// ---- sets for everyone: send the whole list, after the PIN if needed. An
// unlock for them lasts while the dialog is open (or until a save sent before
// it closed is answered).

const saver = globalSaver({
  t: 'filterSets', resultT: 'filterSetsResult', pinIn: $('flt-pin'), pinRow: $('flt-pin-row'), pinOk: $('flt-pin-ok'),
  setNote, isOpen: filtersOpen, failTitle: 'Filter set not saved', render: renderSetBar,
});
$('flt-pin-cancel').addEventListener('click', saver.cancel);
// Sets of the logged-in user ('Me') save to their profile: no PIN row.
const profSaver = profileSaver({ setNote, isOpen: filtersOpen, failTitle: 'Filter set not saved', render: renderSetBar });
// Saves the whole list of one shared scope ('global' or 'user').
const changeShared = (scope, list, done) => (scope === 'user'
  ? profSaver.change({ filterSets: list.map((s) => ({ name: s.name, ...s.f })) }, done)
  : saver.change({ sets: list.map((s) => ({ name: s.name, ...s.f })) }, done));
const sharedWith = (scope, fn) => fn(setsIn(scope).map((s) => ({ name: s.name, f: s.f })));
const refTo = (scope, name, base) => ({ ...setRef(scope, name), base });

// ---- the buttons

$('flt-set').addEventListener('change', (e) => {
  const [scope, ...rest] = e.target.value.split(':');
  if (!scope) return;
  deleting = false;
  nameDirty = false;
  moveTo = null;
  pickSet(scope, rest.join(':'));
  reload();
});

$('flt-set-cur').addEventListener('input', () => {
  const a = activeSet();
  nameDirty = !!a && $('flt-set-cur').value.trim() !== a.name;
  $('flt-set-cur').classList.remove('pb-input--invalid');
  renderSetBar();
});
$('flt-set-cur-scope').addEventListener('change', () => {
  const a = activeSet();
  const to = $('flt-set-cur-scope').value;
  moveTo = a && a.scope !== to ? to : null;
  renderSetBar();
});
$('flt-set-cur').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); $('flt-save').click(); }
});

// Takes a set out of one scope (a move's second step).
function removeSet(scope, name, done = () => {}) {
  if (scope === 'local') {
    deleteLocalSet(name);
    landed();
    return done();
  }
  changeShared(scope, sharedWith(scope, (l) => l.filter((s) => s.name !== name)), () => { landed(); done(); });
}

// Save writes the filter and, if the name was edited, the new name with it;
// if another scope was chosen, it copies the set there first, then removes it here.
// done() runs once everything is written, or at once when there is nothing to
// write; a refusal (a name, a PIN, an error) never calls it.
function saveSet(done = () => {}) {
  flushEds(); // edits are applied 150 ms after typing: a.edited must see them
  const a = activeSet();
  if (!a || a.scope === 'all' || !(a.edited || nameDirty || moveTo)) return done();
  const box = $('flt-set-cur');
  const name = box.value.trim().slice(0, 32) || a.name;
  const renamed = name !== a.name;
  const moving = !!moveTo && moveTo !== a.scope;
  if (renamed || moving) {
    const into = moving ? moveTo : a.scope;
    const taken = allSets().filter((x) => x.scope === into).map((x) => x.name);
    const err = nameProblem(name, taken, { noun: 'filter set', old: moving ? '' : a.name, reserved: 'all' });
    if (err) { box.classList.add('pb-input--invalid'); setNote(err, true); return box.focus(); }
  }
  flushEds();
  if (moving) {
    const to = moveTo, from = a.scope, was = a.name;
    if (to === 'user' && !profileName()) return setNote('Log in to save a set for yourself.', true);
    nameDirty = false;
    moveTo = null;
    if (to === 'local') {
      saveLocalSet(name);
      return removeSet(from, was, done);
    }
    const f = filterState();
    return changeShared(to, sharedWith(to, (l) => [...l, { name, f }]),
      () => { adoptFilter(f, refTo(to, name, f)); removeSet(from, was, done); });
  }
  if (a.scope === 'local') {
    if (renamed) renameLocalSet(a.name, name);
    nameDirty = false;
    saveLocalSet(name);
    landed();
    return done();
  }
  const f = filterState();
  changeShared(a.scope, sharedWith(a.scope, (l) => l.map((s) => (s.name === a.name ? { ...s, name, f } : s))),
    () => { nameDirty = false; adoptFilter(f, refTo(a.scope, name, f)); landed(); done(); });
}

$('flt-set-del').addEventListener('click', () => {
  const a = activeSet();
  if (!a || a.scope === 'all') return;
  if (!deleting) { deleting = true; renderSetBar(); return; }
  deleting = false;
  if (a.scope === 'local') {
    deleteLocalSet(a.name);
    landed();
    $('flt-set').focus();
    return;
  }
  changeShared(a.scope, sharedWith(a.scope, (l) => l.filter((s) => s.name !== a.name)), () => {
    adoptFilter(filterState());
    landed();
    $('flt-set').focus();
  });
});

function askName() {
  const a = activeSet();
  naming = true;
  const f = $('flt-set-name');
  const named = a && a.scope !== 'all';
  f.value = !named ? '' : saveAsName(a.name, a.scope);
  f.classList.remove('pb-input--invalid');
  // 'Me' is offered only while someone is logged in.
  const sel = $('flt-set-scope');
  sel.replaceChildren(...scopeOptions(!!profileName()).map(([id, label]) => new Option(label, id)));
  sel.value = 'local';
  $('flt-set-name-row').hidden = false;
  f.select();
}
function closeName() {
  $('flt-set-name-row').hidden = true;
  $('flt-set-saveas').focus();
  naming = false;
}
function saveName() {
  const f = $('flt-set-name');
  const name = f.value.trim().slice(0, 32);
  const scope = $('flt-set-scope').value;
  const bad = (msg) => { f.classList.add('pb-input--invalid'); setNote(msg, true); f.focus(); };
  // Save as replaces a name that is taken in that scope.
  const err = nameProblem(name, [], { noun: 'filter set', old: '', reserved: 'all' });
  if (err) return bad(err);
  flushEds();
  closeName();
  if (scope === 'local') {
    saveLocalSet(name);
    return landed();
  }
  const fNow = filterState();
  if (scope === 'user' && !profileName()) return setNote('Log in to save a set for yourself.', true);
  changeShared(scope, sharedWith(scope, (l) => [...l.filter((s) => s.name.toLowerCase() !== name.toLowerCase()), { name, f: fNow }]),
    () => { adoptFilter(fNow, refTo(scope, name, fNow)); landed(); });
}
const flushEds = () => { eds.src.flush(); eds.dst.flush(); };
$('flt-set-saveas').addEventListener('click', askName);
$('flt-set-name-ok').addEventListener('click', saveName);
$('flt-set-name-cancel').addEventListener('click', closeName);
$('flt-set-name').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); saveName(); }
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeName(); }
});

// Keys stay in the dialog (so the layouts' own keys don't act behind it).
const dlg = trapDialog(root, dialog, cancelFilters);

// ---- share links: the filter as one item in a #share= link (exportfmt.js);
// opening it shows the review sheet (transfer.js). The old ?f= link still opens.

const shareUI = { box: $('flt-share'), input: $('flt-share-url'), status: $('flt-share-status'), copy: $('flt-share-copy') };
$('flt-share-btn').addEventListener('click', () => {
  flushEds();
  const a = activeSet();
  shareLink(filterShare(a && a.scope !== 'all' ? a.name : '', filterState()), shareUI);
});
$('flt-share-copy').addEventListener('click', () => copyLink(shareUI));

// ---- when filters hide everything

// A note for an empty Recent whose entries the filters hide, or null.
// While searching it's about the search: still reading back through the
// log, or nothing found (and the filters may not even apply).
export function emptyNote(rows, fetching) {
  if (rows.length) return null;
  if (!S.hist.length) return el('div', 'sode__filters-empty', el('span', 'sode__hint', 'No history yet'));
  const searching = searchActive();
  if (searching && fetching) return el('div', 'sode__filters-empty', el('span', 'sode__hint', 'Searching…'));
  const clear = el('button', 'pb-btn pb-btn--outline pb-btn--sm', searching ? 'Clear search' : 'Clear filters');
  clear.type = 'button';
  clear.addEventListener('click', () => (searching ? clearSearch() : pickSet('all', 'All')));
  return el('div', 'sode__filters-empty', el('span', 'sode__hint', !searching && activeStack().length ? 'No matches in the stacked sets.' : 'No matches.'), clear);
}

// ---- quick search: the Filter by Source / Destination boxes above Recent,
// in both layouts, kept in step. Typing opens a list of matches like the
// patch boxes'; picking one filters history to exactly that source or
// destination. Names logged in history count too, so a source the router
// now calls something else is still found. Text that's an expression
// (ranges, lists, !, /regex/) applies as typed instead.

const looksLikeExpr = (t) => /[,!/"']/.test(t) || /^\s*\d+\s*(-|\s(thru|through|to)\s)\s*\d+\s*$/i.test(t) || /^\s*not\s+['"/\d]/i.test(t);
const MENU = 50;

// What a box can find: the router's items, plus names history logged for
// them that differ (another router, or renamed since).
// Built once per change to the names or history, not on every keystroke.
const candCache = { src: null, dst: null };
function candidates(k) {
  const live = k === 'src' ? S.srcs : S.dsts;
  const stamp = [live, S.hist, S.hist.length];
  const c = candCache[k];
  if (c && c.stamp.every((x, i) => x === stamp[i])) return c.list;
  const list = buildCandidates(k, live);
  candCache[k] = { stamp, list };
  return list;
}
function buildCandidates(k, live) {
  const by = k === 'src' ? S.srcBy : S.dstBy;
  const seen = new Set(live.map((i) => `${i.n}\u0000${i.name}`));
  const extra = [];
  for (const e of S.hist) {
    for (const x of [e, ...(e.follows || [])]) {
      const n = x[k], name = k === 'src' ? x.srcName : x.dstName;
      const key = `${n}\u0000${name || ''}`;
      if (seen.has(key) || (by.has(n) && !name)) continue;
      seen.add(key);
      extra.push(item(n, name || ''));
    }
  }
  return extra.length ? [...live, ...extra] : live;
}

const searchBoxes = [...document.querySelectorAll('[data-search]')];
// Per box: the picks (shown as chips; history shows any of them) and
// what's typed. Each input has its own list of matches.
const sb = { src: { picks: [], text: '' }, dst: { picks: [], text: '' } };
const lists = []; // every input's combobox

function menuOf(f) { return f.parentElement.querySelector('.sode__menu'); }

// What a box searches for: its picks, by number, and a typed expression.
function boxQuery(k) {
  const st = sb[k];
  const expr = looksLikeExpr(st.text) ? st.text.trim() : '';
  return [...st.picks.map((it) => it.num), expr].filter(Boolean).join(', ');
}
const applySearch = () => setSearch({ src: boxQuery('src'), dst: boxQuery('dst') });

function clearSearch() {
  for (const k of ['src', 'dst']) Object.assign(sb[k], { picks: [], text: '' });
  for (const f of document.querySelectorAll('[data-search] input')) f.value = '';
  for (const cb of lists) cb.close();
  applySearch();
}

// Adds a match as a pick, and clears the box for another.
function pick(k, f, cb, it) {
  const st = sb[k];
  if (!st.picks.some((p) => p.n === it.n)) st.picks.push(it);
  st.text = '';
  f.value = '';
  cb.close();
  applySearch();
}

function unpick(k, i) {
  sb[k].picks.splice(i, 1);
  applySearch();
}

function typed(k, f, cb) {
  sb[k].text = f.value;
  cb.refresh();
  applySearch();
}

for (const b of searchBoxes) {
  for (const k of ['src', 'dst']) {
    const f = b.querySelector(`[data-search-${k}]`);
    f.before(el('span', 'sode__search-chips'));
    const cb = combobox(f, menuOf(f), {
      // Plain text offers matches; history changes once one's picked.
      source: (t) => (!t.trim() || looksLikeExpr(t) ? [] : search(candidates(k), t, MENU)),
      onPick: (it) => pick(k, f, cb, it),
    });
    lists.push(cb);
    f.addEventListener('input', () => typed(k, f, cb));
    f.addEventListener('focus', () => { if (f.value && !looksLikeExpr(f.value)) typed(k, f, cb); });
    f.addEventListener('blur', () => cb.close());
    f.addEventListener('keydown', (e) => {
      const st = sb[k];
      if (cb.nav(e)) return; // as in the patch boxes
      // Enter, Tab or + adds the match; Backspace in an empty box takes the
      // last pick back off.
      if (cb.open && (e.key === 'Enter' || e.key === '+' || (e.key === 'Tab' && !e.shiftKey))) {
        if (cb.accept() && e.key !== 'Tab') e.preventDefault();
        return;
      }
      if (e.key === 'Backspace' && !f.value && st.picks.length) {
        e.preventDefault();
        unpick(k, st.picks.length - 1);
        return;
      }
      if (e.key !== 'Escape') return;
      // Esc closes the list, then clears the typing, then leaves the box
      // (picks stay; Clear removes them).
      e.preventDefault();
      e.stopPropagation();
      if (cb.open) cb.close();
      else if (f.value) { f.value = ''; typed(k, f, cb); }
      else f.blur();
    });
  }
  // Clear leaves focus alone (no keyboard popping up on a phone); the
  // button greys itself out, so let focus go rather than sit on it.
  b.querySelector('[data-search-clear]').addEventListener('click', (e) => {
    clearSearch();
    e.currentTarget.blur();
  });
}

function renderSearch() {
  const on = searchActive();
  const n = on ? histRows(S.hist).length : 0;
  const note = !on ? '' : `${n} found${S.histMore ? ' so far' : ''}${searchRespectsFilters() ? '' : ' · ignoring filters'}`;
  for (const b of searchBoxes) {
    for (const k of ['src', 'dst']) {
      const st = sb[k];
      const f = b.querySelector(`[data-search-${k}]`);
      // Keep the other layout's box in step (not the one being typed in).
      if (document.activeElement !== f && f.value !== st.text) f.value = st.text;
      const err = looksLikeExpr(st.text) ? parseFilter(st.text).error : null;
      f.classList.toggle('pb-input--invalid', !!err);
      f.title = err || '';
      f.previousElementSibling.replaceChildren(...st.picks.map((it, i) => {
        const x = el('button', 'pb-chip__remove', '×');
        x.type = 'button';
        x.tabIndex = -1;
        x.setAttribute('aria-label', `Remove ${it.label}`);
        x.addEventListener('mousedown', (e) => e.preventDefault());
        x.addEventListener('click', () => unpick(k, i));
        return el('span', 'pb-chip pb-chip--static sode__search-chip', labelNode(it), x);
      }));
    }
    b.querySelector('[data-search-clear]').disabled = !on && !['src', 'dst'].some((k) => sb[k].text || sb[k].picks.length);
    b.querySelector('[data-search-note]').textContent = note;
  }
}
on('search', renderSearch);
on('history', () => { if (searchActive()) renderSearch(); });

on('filters', renderButtons);
on('prefs', () => { if (filtersOpen()) renderSetBar(); });
on('hello', () => {
  renderButtons();
  if (filtersOpen()) { renderShow(); eds.src.preview(); eds.dst.preview(); }
});
on('names', () => { if (filtersOpen()) { eds.src.preview(); eds.dst.preview(); } });
renderButtons();
