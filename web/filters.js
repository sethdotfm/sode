// Recent's Filters dialog, shared by both layouts. Applies as you type; kept per browser
// (filterstore.js). Share link: transfer.js reads it back through a review sheet.

import { $, el, S, on, emit, nameProblem } from './core.js';
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
  renameLocalSet,
  setsIn,
  setRef,
  filterSummary,
  toggleSet,
  activeStack,
  isActive,
} from './filterstore.js';
import { pref } from './device.js';
import { PREFS } from './prefs.js';
import { shareLink, copyLink } from './transfer.js';
import { filterShare, saveAsName } from './exportfmt.js';
import { scopeIds, scopeLabel, scopeOptions } from './scopes.js';
import { globalSaver, profileSaver, scopeWriter } from './globalsave.js';
import { profileName, editTarget } from './profile.js';
import { FILTER_DEFAULT, normFilter, snapshot, restore } from './filterstore.js';
import { upsertEntry, filterEntry } from './profilesrules.js';
import { trapDialog, labeled } from './widgets.js';
import { editor } from './filtereditor.js';

const SCOPE_GROUPS = scopeIds(true).map((id) => [id, scopeLabel(id)]);
const root = $('filters');
const dialog = $('filters-dialog');
const MODE_KEY = 'sode-filter-mode';
let mode = readStr(MODE_KEY) === 'code' ? 'code' : 'visual';

// Aimed at a profile (bus 'openFilterEditor'): the filter is a draft held here (never the
// browser's own filter or active set); a name and Save replace the set bar and a save goes to
// that user's profile.

let tgt = null;
const curF = () => (tgt ? tgt.f : filterState());
const putF = (next) => {
  if (tgt) tgt.f = normFilter(next);
  else setFilter(next);
};

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
  // Active sets are selected (click again to take one off; none left is All); * when a single
  // one's filter was edited since.
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
// The filter and set as they were when the dialog opened: Cancel restores them (edits apply to
// Recent as they are made).
let openSnap = null;
export const filtersOpen = () => !root.hidden;

export function openFilters() {
  back = document.activeElement;
  tgt = null;
  openSnap = snapshot();
  chrome();
  renderMode();
  loadDialog(filterState());
  $('flt-set-name-row').hidden = true;
  $('flt-pin-row').hidden = true;
  deleting = false;
  renderSetBar();
  dlg.show();
  dialog.focus();
}

// A set write has landed: Cancel must not roll the filter back past it.
function landed() {
  if (filtersOpen() && !tgt) openSnap = snapshot();
}

function cancelFilters() {
  if (tgt || !openSnap) return closeFilters();
  // Settle the editors' pending text first (it would otherwise be written after the restore).
  flushEds();
  restore(openSnap);
  closeFilters(false);
}

function closeFilters(flush = true) {
  if (flush) flushEds();
  dlg.hide();
  if (tgt) {
    tgt = null;
    chrome();
    if (back?.isConnected) back.focus();
    return;
  }
  // A save already sent finishes first (its answer locks up after it); one waiting for the PIN
  // is dropped.
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

function chrome() {
  const t = !!tgt;
  $('flt-set').closest('.sode__filters-sets').hidden = t;
  tBar.hidden = !t;
  tSave.hidden = !t;
  $('flt-save').hidden = t;
  $('flt-clear').hidden = t;
  $('flt-share-btn').hidden = t;
  $('filters-title').textContent = t ? `${tgt.old ? 'Edit' : 'Add'} filter set for ${tgt.user}` : 'Filters';
  if (t) {
    tName.value = tgt.old;
    tNoteSet('');
    tName.classList.remove('pb-input--invalid');
  }
}

const tgtSaver = profileSaver({
  target: () => tgt,
  setNote: tNoteSet,
  isOpen: () => filtersOpen() && !!tgt,
  failTitle: 'Filter set not saved',
  render: () => {
    tSave.disabled = tgtSaver.busy();
  },
});

function saveTarget() {
  if (!tgt) return;
  eds.src.flush();
  eds.dst.flush();
  const name = tName.value.trim();
  const bad = nameProblem(
    name,
    tgt.lists.map((x) => String(x?.name)),
    { noun: 'filter set', old: tgt.old, reserved: 'all' },
  );
  if (bad) {
    tName.classList.add('pb-input--invalid');
    tNoteSet(bad, true);
    tName.focus();
    return;
  }
  tName.classList.remove('pb-input--invalid');
  const list = upsertEntry(tgt.lists, filterEntry(name, tgt.f), tgt.old);
  saveIn('user', list, closeFilters);
}
tSave.addEventListener('click', saveTarget);
tName.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    e.preventDefault();
    saveTarget();
  }
});

on('openFilterEditor', (m) => {
  const t = editTarget(m, 'filterSets');
  if (!t) return;
  back = document.activeElement;
  tgt = { ...t, f: normFilter(t.found || FILTER_DEFAULT) };
  chrome();
  renderMode();
  loadDialog(tgt.f);
  dlg.show();
  dialog.focus();
});

for (const b of document.querySelectorAll('[data-filters]')) b.addEventListener('click', openFilters);
$('flt-done').addEventListener('click', cancelFilters);
// Save closes once the write has landed (the server's answer for everyone or Me; stays open for
// a PIN or an error).
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
// Show filter f in the editors and hide the share row.
function loadDialog(f) {
  renderShow();
  eds.src.load(f.src);
  eds.dst.load(f.dst);
  $('flt-share').hidden = true;
}
const reload = () => loadDialog(curF());

$('flt-clear').addEventListener('click', () => {
  pickSet('all', 'All');
  reload();
});

// ---- the set bar. A saved set is renamed by editing its name and pressing Save; sets for everyone are settings saved on the server, so with a PIN they ask first.

const setKey = (s) => `${s.scope}:${s.name}`;
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
  $('flt-share-btn').disabled = stack.length > 0;
  $('flt-set-del').disabled = busy || !named;
  $('flt-set-del').textContent = deleting && named ? `Delete “${a.name}”?` : 'Delete';
  $('flt-set-del').classList.toggle('pb-btn--danger', deleting && named);
  if (busy) return;
  if (stack.length) {
    return setNote(
      `Stacked: ${stack.map((s) => s.name).join(', ')} (${PREFS.setStack.values[pref('setStack')]}). ` +
        'Changing a filter leaves the stack, and the share link is off.',
    );
  }
  if (named && moveTo) return setNote(`Save moves it from ${scopeLabel(a.scope)} to ${scopeLabel(moveTo)}.`);
  setNote(a && a.scope !== 'all' ? `${scopeLabel(a.scope)}${a.edited ? ' (edited)' : ''}` : '');
}
const setNote = (text, error = false) => {
  $('flt-set-note').textContent = text;
  $('flt-set-note').classList.toggle('sode__error-text', error);
};

// ---- sets for everyone: send the whole list, after the PIN if needed. An unlock lasts while the dialog is open (or until a save sent before it closed is answered).

const saver = globalSaver({
  t: 'filterSets',
  resultT: 'filterSetsResult',
  pinIn: $('flt-pin'),
  pinRow: $('flt-pin-row'),
  pinOk: $('flt-pin-ok'),
  setNote,
  isOpen: filtersOpen,
  failTitle: 'Filter set not saved',
  render: renderSetBar,
});
$('flt-pin-cancel').addEventListener('click', saver.cancel);
const profSaver = profileSaver({ setNote, isOpen: filtersOpen, failTitle: 'Filter set not saved', render: renderSetBar });
const wireSet = (s) => ({ name: s.name, ...s.f });
const { saveIn, removeIn } = scopeWriter({
  global: saver,
  user: profSaver,
  target: () => tgt && tgtSaver,
  key: 'filterSets',
  globalKey: 'sets',
  listOf: (scope) => setsIn(scope).map(wireSet),
  same: (a, b) => a === b,
  dropLocal: deleteLocalSet,
});
// fn maps the scope's sets as { name, f }; the result goes out in wire shape.
const sharedWith = (scope, fn) => fn(setsIn(scope).map((s) => ({ name: s.name, f: s.f }))).map(wireSet);
const refTo = (scope, name, base) => ({ ...setRef(scope, name), base });

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

function removeSet(scope, name, done = () => {}) {
  removeIn(scope, name, () => {
    landed();
    done();
  });
}

// A move copies to the other scope first, then removes here. done() runs once everything is
// written (or at once if nothing to write); a refusal never calls it.
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
  if (moving) {
    const to = moveTo,
      from = a.scope,
      was = a.name;
    if (to === 'user' && !profileName()) return setNote('Log in to save a set for yourself.', true);
    nameDirty = false;
    moveTo = null;
    if (to === 'local') {
      saveLocalSet(name);
      return removeSet(from, was, done);
    }
    const f = filterState();
    return saveIn(
      to,
      sharedWith(to, (l) => [...l, { name, f }]),
      () => {
        adoptFilter(f, refTo(to, name, f));
        removeSet(from, was, done);
      },
    );
  }
  if (a.scope === 'local') {
    if (renamed) renameLocalSet(a.name, name);
    nameDirty = false;
    saveLocalSet(name);
    landed();
    return done();
  }
  const f = filterState();
  saveIn(
    a.scope,
    sharedWith(a.scope, (l) => l.map((s) => (s.name === a.name ? { ...s, name, f } : s))),
    () => {
      nameDirty = false;
      adoptFilter(f, refTo(a.scope, name, f));
      landed();
      done();
    },
  );
}

$('flt-set-del').addEventListener('click', () => {
  const a = activeSet();
  if (!a || a.scope === 'all') return;
  if (!deleting) {
    deleting = true;
    renderSetBar();
    return;
  }
  deleting = false;
  removeIn(a.scope, a.name, () => {
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
  if (scope === 'user' && !profileName()) return setNote('Log in to save a set for yourself.', true);
  saveIn(
    scope,
    sharedWith(scope, (l) => [...l.filter((s) => s.name.toLowerCase() !== name.toLowerCase()), { name, f: fNow }]),
    () => {
      adoptFilter(fNow, refTo(scope, name, fNow));
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

// ---- share links: the filter as one item in a #share= link (exportfmt.js). The old ?f= link still opens.

const shareUI = { box: $('flt-share'), input: $('flt-share-url'), status: $('flt-share-status'), copy: $('flt-share-copy') };
$('flt-share-btn').addEventListener('click', () => {
  flushEds();
  const a = activeSet();
  shareLink(filterShare(a && a.scope !== 'all' ? a.name : '', filterState()), shareUI);
});
$('flt-share-copy').addEventListener('click', () => copyLink(shareUI));

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
