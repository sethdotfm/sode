// History's Filters dialog, for both layouts. The filter applies while you type. filterstore.js keeps it per browser.

import { $, el, S, on, nameProblem } from './core.js';
import { readStr, writeStr } from './store.js';
import {
  filterState,
  setFilter,
  filterCount,
  allSets,
  activeSet,
  adoptFilter,
  pickSet,
  saveLocalSet,
  deleteLocalSet,
  setsIn,
  setRef,
  filterSummary,
  toggleSet,
  activeStack,
  isActive,
} from './filterstore.js';
import { pref } from './device.js';
import { PREFS } from './prefs.js';
import { saveAsName } from './exportfmt.js';
import { scopeIds, scopeLabel, scopeOptions } from './scopes.js';
import { globalSaver, profileSaver, scopeWriter } from './globalsave.js';
import { profileName, editTarget } from './profile.js';
import { relabelLocalBookmarks } from './bookmarkstore.js';
import { newPresetId } from './presetref.js';
import { browserFilters, profileFilters } from './editortargets.js';
import { FILTER_DEFAULT, normFilter, snapshot, restore } from './filterstore.js';
import { trapDialog, labeled } from './widgets.js';
import { editor } from './filtereditor.js';
import { statusBadge, setStatus } from './brokenview.js';
import { STATUS_TEXT } from './brokenrules.js';
import { replaceBox } from './filterreplace.js';
import { inboxUser } from './inbox.js';
import { sendCopy } from './sendcopy.js';

const SCOPE_GROUPS = scopeIds(true).map((id) => [id, scopeLabel(id)]);
const root = $('filters');
const dialog = $('filters-dialog');
const MODE_KEY = 'sode-filter-mode';
let mode = readStr(MODE_KEY) === 'code' ? 'code' : 'visual';

// Bus 'openFilterEditor' swaps in a profile target (editortargets.js). The filter is then a draft
// (never the browser's own filter or active set). A name and Save replace the set bar, and a save
// goes to that user's profile.
const browser = browserFilters({
  read: filterState,
  write: setFilter,
  snapshot,
  restore,
  onClose: () => saver.close(), // a save already sent finishes first
});
let target = browser;
const curF = () => target.filter();
const putF = (next) => target.put(next);

const edOpts = { read: curF, write: putF, mode: () => mode };
const eds = { src: editor('src', edOpts), dst: editor('dst', edOpts) };

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

function buttonState() {
  const sets = activeStack().length;
  const parts = filterCount();
  if (sets) return { text: 'Filters', count: `${sets} sets`, aria: `Filters, ${sets} sets stacked`, on: true };
  if (parts) return { text: 'Filters', count: `${parts}`, aria: `Filters, ${parts} active`, on: true };
  return { text: 'Filters', aria: 'Filters, none active', on: false };
}

function renderButtons() {
  const { text, count, on: lit, aria } = buttonState();
  for (const b of document.querySelectorAll('[data-filters]')) {
    b.replaceChildren(text, ...(count ? [el('span', 'sode__filters-count', count)] : []));
    b.classList.toggle('sode__filters-btn--on', lit);
    b.setAttribute('aria-label', aria);
  }
  // * marks a single active set whose filter changed after it was picked.
  const sets = allSets();
  const a = activeSet();
  const chip = (s) => {
    const on = s.scope === 'all' ? !!a && a.scope === 'all' : isActive(s.scope, s.id);
    const edited = on && !!a && a.scope !== 'all' && a.edited;
    const b = el('button', 'pb-chip sode__set', s.name + (edited ? '*' : ''));
    const badge = s.scope === 'all' ? null : statusBadge(setStatus(s).status);
    if (badge) b.append(badge);
    b.type = 'button';
    b.setAttribute('aria-pressed', String(on));
    if (edited) b.setAttribute('aria-label', `${s.name} (edited)`);
    b.dataset.pbTip = filterSummary(s.f) + (edited ? ' (edited)' : '');
    b.classList.add('pb-tip--end');
    b.addEventListener('click', () => {
      toggleSet(s.scope, s.id);
      if (filtersOpen()) reload();
    });
    return b;
  };
  const groups = SCOPE_GROUPS.map(([scope, label]) => [label, sets.filter((s) => s.scope === scope)]).filter(([, g]) => g.length);
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
    const [pinned, sep, ...rest] = kids;
    box.replaceChildren(...(pinned ? [pinned, sep, el('div', 'sode__sets-run', ...rest)] : []));
  }
  if (filtersOpen()) renderSetBar();
}

let back = null;
const filtersOpen = () => !root.hidden;

export function openFilters() {
  back = document.activeElement;
  target = browser;
  target.begin();
  chrome();
  renderMode();
  loadDialog(filterState());
  $('flt-set-name-row').hidden = true;
  deleting = false;
  renderSetBar();
  dlg.show();
  dialog.focus();
}

// A set write has landed: Cancel must not roll the filter back past it.
function landed() {
  if (filtersOpen()) target.landed();
}

function cancelFilters() {
  target.cancel(flushEds, closeFilters);
}

function closeFilters(flush = true) {
  if (flush) flushEds();
  dlg.hide();
  target.close();
  target = browser;
  chrome();
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

function chrome() {
  const own = target.own;
  $('flt-set').closest('.sode__filters-sets').hidden = !own;
  tBar.hidden = own;
  tSave.hidden = own;
  $('flt-save').hidden = !own;
  $('flt-clear').hidden = !own;
  $('filters-title').textContent = target.title;
  tName.value = target.name;
  tNoteSet('');
  tName.classList.remove('pb-input--invalid');
}

const tgtSaver = profileSaver({
  target: () => target.edit,
  setNote: tNoteSet,
  isOpen: () => filtersOpen() && !!target.edit,
  failTitle: 'Filter set not saved',
  render: () => {
    tSave.disabled = tgtSaver.busy();
  },
});

function saveTarget() {
  flushEds();
  const { err, list } = target.build(tName.value.trim());
  if (err) {
    tName.classList.add('pb-input--invalid');
    tNoteSet(err, true);
    tName.focus();
    return;
  }
  tName.classList.remove('pb-input--invalid');
  saveIn('user', list, closeFilters);
}
tSave.addEventListener('click', saveTarget);
tName.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    e.preventDefault();
    saveTarget();
  }
});

// Edit makes the set active, so the dialog shows it.
on('editFilterSetAt', (m) => {
  pickSet(m.scope, m.id);
  openFilters();
});

on('openFilterEditor', (m) => {
  const t = editTarget(m, 'filterSets');
  if (!t) return;
  back = document.activeElement;
  target = profileFilters(t, { normalize: (f) => normFilter(f || FILTER_DEFAULT), saver: tgtSaver });
  chrome();
  renderMode();
  loadDialog(target.filter());
  dlg.show();
  dialog.focus();
});

for (const b of document.querySelectorAll('[data-filters]')) b.addEventListener('click', openFilters);
$('flt-done').addEventListener('click', cancelFilters);
// Save closes after the server answers; an error keeps it open.
$('flt-save').addEventListener('click', () =>
  saveSet(() => {
    if (filtersOpen()) closeFilters();
  }),
);
$('flt-visual').addEventListener('click', () => setMode('visual'));
$('flt-code').addEventListener('click', () => setMode('code'));
for (const k of ['sode', 'ext', 'hidden']) {
  $(`flt-${k}`).addEventListener('change', (e) => {
    putF({ ...curF(), [k]: e.target.checked });
    renderShow();
  });
}
function loadDialog(f) {
  renderShow();
  eds.src.load(f.src);
  eds.dst.load(f.dst);
  replace.paint();
}
const reload = () => loadDialog(curF());
// Only the browser's own dialog has an active set; a profile target edits a draft.
const replace = replaceBox({
  set: () => {
    const a = target === browser && activeSet();
    return a && a.scope !== 'all' ? allSets().find((s) => s.scope === a.scope && s.id === a.id) || null : null;
  },
  read: curF,
  put: putF,
  reload,
});
$('flt-src').before(replace.box);

$('flt-clear').addEventListener('click', () => {
  pickSet('all', 'All');
  reload();
});

const setKey = (s) => `${s.scope}:${s.id}`;
let deleting = false;
let nameDirty = false;
let moveTo = null;

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
      const st = s.scope === 'all' ? '' : setStatus(s).status;
      const o = el('option', null, s.name + (a && setKey(a) === setKey(s) && a.edited ? ' (edited)' : '') + (st ? `, ${STATUS_TEXT[st]}` : ''));
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
  if (!named) {
    nameDirty = false;
    cur.value = '';
  } else if (!nameDirty) cur.value = a.name;
  cur.disabled = busy || !named;
  const scopeBox = $('flt-set-cur-scope');
  scopeBox.replaceChildren(...scopeOptions(!!profileName()).map(([id, label]) => new Option(label, id)));
  if (named && ![...scopeBox.options].some((o) => o.value === moveTo)) moveTo = null;
  if (!named) moveTo = null;
  scopeBox.value = moveTo || (named && [...scopeBox.options].some((o) => o.value === a.scope) ? a.scope : 'local');
  scopeBox.disabled = busy || !named;
  $('flt-set-saveas').disabled = busy || stack.length > 0;
  $('flt-set-del').disabled = busy || !named;
  $('flt-set-del').textContent = deleting && named ? `Delete “${a.name}”?` : 'Delete';
  $('flt-set-del').classList.toggle('pb-btn--danger', deleting && named);
  // The saved set goes, not unsaved edits.
  $('flt-set-send').hidden = !inboxUser();
  $('flt-set-send').disabled = busy || !named;
  if (busy) return;
  if (stack.length) {
    return setNote(`Stacked: ${stack.map((s) => s.name).join(', ')} (${PREFS.setStack.values[pref('setStack')]})`);
  }
  if (named && moveTo) return setNote(`Save moves it from ${scopeLabel(a.scope)} to ${scopeLabel(moveTo)}.`);
  setNote(a && a.scope !== 'all' ? `${scopeLabel(a.scope)}${a.edited ? ' (edited)' : ''}` : '');
}
const setNote = (text, error = false) => {
  $('flt-set-note').textContent = text;
  $('flt-set-note').classList.toggle('sode__error-text', error);
};

const saver = globalSaver({
  t: 'filterSets',
  resultT: 'filterSetsResult',
  setNote,
  isOpen: filtersOpen,
  failTitle: 'Filter set not saved',
  render: renderSetBar,
});
const profSaver = profileSaver({ setNote, isOpen: filtersOpen, failTitle: 'Filter set not saved', render: renderSetBar });
const wireSet = (s) => ({ id: s.id, name: s.name, ...s.f });
const { saveIn, removeIn } = scopeWriter({
  global: saver,
  user: profSaver,
  target: () => target.saver,
  key: 'filterSets',
  globalKey: 'sets',
  listOf: (scope) => setsIn(scope).map(wireSet),
  keyOf: (s) => s.id,
  same: (a, b) => a === b,
  dropLocal: deleteLocalSet,
});
// fn maps the scope's sets as { id, name, f }; the result goes out in wire shape. The page makes a new set's id, so the
// active set can name it before the save lands.
const sharedWith = (scope, fn) => fn(setsIn(scope).map((s) => ({ id: s.id, name: s.name, f: s.f }))).map(wireSet);
const refTo = (scope, id, base) => ({ ...setRef(scope, id), base });

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
  if (e.key === 'Enter') {
    e.preventDefault();
    $('flt-save').click();
  }
});

function removeSet(scope, id, done = () => {}) {
  removeIn(scope, id, () => {
    landed();
    done();
  });
}

// A move copies to the other scope first, then removes here. done() runs after everything is
// written (or immediately if there is nothing to write); a refusal never calls it.
function saveSet(done = () => {}) {
  flushEds(); // edits apply 150 ms after typing: a.edited must see them
  const a = activeSet();
  if (!a || a.scope === 'all' || !(a.edited || nameDirty || moveTo)) return done();
  const box = $('flt-set-cur');
  const name = box.value.trim().slice(0, 32) || a.name;
  const renamed = name !== a.name;
  const moving = !!moveTo && moveTo !== a.scope;
  if (renamed || moving) {
    const into = moving ? moveTo : a.scope;
    const taken = allSets()
      .filter((x) => x.scope === into)
      .map((x) => x.name);
    const err = nameProblem(name, taken, { noun: 'filter set', old: moving ? '' : a.name, reserved: 'all' });
    if (err) {
      box.classList.add('pb-input--invalid');
      setNote(err, true);
      return box.focus();
    }
  }
  flushEds();
  // The set keeps its id through a rename and a move, so a rename moves no bookmark.
  if (moving) {
    const to = moveTo,
      from = a.scope;
    if (to === 'user' && !profileName()) return setNote('Sign in to save to Me.', true);
    nameDirty = false;
    moveTo = null;
    if (to === 'local') {
      saveLocalSet(name, a.id);
      return removeSet(from, a.id, done);
    }
    const f = filterState();
    return saveIn(
      to,
      sharedWith(to, (l) => [...l, { id: a.id, name, f }]),
      () => {
        adoptFilter(f, refTo(to, a.id, f));
        removeSet(from, a.id, done);
      },
    );
  }
  if (a.scope === 'local') {
    nameDirty = false;
    saveLocalSet(name, a.id);
    // The key stays; only the stored label follows, as for a preset.
    if (renamed) relabelLocalBookmarks(`filterSet:local:${a.id}`, name);
    landed();
    return done();
  }
  const f = filterState();
  saveIn(
    a.scope,
    sharedWith(a.scope, (l) => l.map((s) => (s.id === a.id ? { ...s, name, f } : s))),
    () => {
      nameDirty = false;
      adoptFilter(f, refTo(a.scope, a.id, f));
      landed();
      done();
    },
  );
}

$('flt-set-send').addEventListener('click', () => {
  const a = activeSet();
  const set = a && setsIn(a.scope).find((s) => s.id === a.id);
  if (set) sendCopy({ filterSets: [set] }, set.name);
});
$('flt-set-del').addEventListener('click', () => {
  const a = activeSet();
  if (!a || a.scope === 'all') return;
  if (!deleting) {
    deleting = true;
    renderSetBar();
    return;
  }
  deleting = false;
  removeIn(a.scope, a.id, () => {
    if (a.scope !== 'local') adoptFilter(filterState());
    landed();
    $('flt-set').focus();
  });
});

function askName() {
  const a = activeSet();
  const f = $('flt-set-name');
  const named = a && a.scope !== 'all';
  f.value = !named ? '' : saveAsName(a.name, a.scope);
  f.classList.remove('pb-input--invalid');
  const sel = $('flt-set-scope');
  sel.replaceChildren(...scopeOptions(!!profileName()).map(([id, label]) => new Option(label, id)));
  sel.value = 'local';
  $('flt-set-name-row').hidden = false;
  f.select();
}
function closeName() {
  $('flt-set-name-row').hidden = true;
  $('flt-set-saveas').focus();
}
function saveName() {
  const f = $('flt-set-name');
  const name = f.value.trim().slice(0, 32);
  const scope = $('flt-set-scope').value;
  const bad = (msg) => {
    f.classList.add('pb-input--invalid');
    setNote(msg, true);
    f.focus();
  };
  const err = nameProblem(name, [], { noun: 'filter set', old: '', reserved: 'all' });
  if (err) return bad(err);
  flushEds();
  closeName();
  if (scope === 'local') {
    saveLocalSet(name);
    return landed();
  }
  const fNow = filterState();
  if (scope === 'user' && !profileName()) return setNote('Sign in to save to Me.', true);
  // Save as… over a taken name keeps that set's id, as for a preset.
  const id = setsIn(scope).find((s) => s.name.toLowerCase() === name.toLowerCase())?.id || newPresetId();
  saveIn(
    scope,
    sharedWith(scope, (l) => [...l.filter((s) => s.id !== id), { id, name, f: fNow }]),
    () => {
      adoptFilter(fNow, refTo(scope, id, fNow));
      landed();
    },
  );
}
const flushEds = () => {
  eds.src.flush();
  eds.dst.flush();
};
$('flt-set-saveas').addEventListener('click', askName);
$('flt-set-name-ok').addEventListener('click', saveName);
$('flt-set-name-cancel').addEventListener('click', closeName);
$('flt-set-name').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    e.preventDefault();
    saveName();
  }
  if (e.key === 'Escape') {
    e.preventDefault();
    e.stopPropagation();
    closeName();
  }
});

const dlg = trapDialog(root, dialog, cancelFilters);
on('locking', () => filtersOpen() && cancelFilters());
// The filter changes live while the dialog is open, and a close goes back: the Palette does not open over it.
on('unsavedAsk', (a) => {
  if (filtersOpen()) a.unsaved = true;
});

on('filters', renderButtons);
on('prefs', () => {
  if (filtersOpen()) renderSetBar();
});
on('hello', () => {
  renderButtons();
  if (filtersOpen()) {
    renderShow();
    eds.src.preview();
    eds.dst.preview();
  }
});
on('names', () => {
  if (filtersOpen()) {
    eds.src.preview();
    eds.dst.preview();
  }
});
renderButtons();
