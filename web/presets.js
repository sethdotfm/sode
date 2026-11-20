// Presets in the page: firing, filling the boxes and the editor dialog. The strip above History is the Bookmarks
// strip (bookmarks.js). Storage and pure parts are presetstore.js, the plan is plans.js.

import { el, S, on, emit, off, search, srcItem, dstItem, nameProblem } from './core.js';
import { btn } from './formbits.js';
import { levelInput, parseLevels } from './levelview.js';
import { showResult, presetPlan, canRedo } from './plans.js';
import { startPick, endPick } from './historypick.js';
import { globalSaver, profileSaver, scopeWriter } from './globalsave.js';
import { profileName, editTarget } from './profile.js';
import { browserPresets, profilePresets } from './editortargets.js';
import { labeled } from './widgets.js';
import { modal } from './modal.js';
import { itemLinkBox } from './itemlinkview.js';
import { saveAsName } from './exportfmt.js';
import { scopeLabel, scopeOptions } from './scopes.js';
import { carryNames } from './presetref.js';
import { presetStatus } from './brokenview.js';
import { STATUS_TEXT } from './brokenrules.js';
import { renameLocalBookmarks } from './bookmarkstore.js';
import {
  allPresets,
  presetsIn,
  findPreset,
  normPreset,
  storedPreset,
  presetProblem,
  presetsProblem,
  replacePreset,
  saveLocalPreset,
  deleteLocalPreset,
  saved,
  parseRefs,
  itemsFromEntries,
  isBlankRow,
  fillFrom,
} from './presetstore.js';

const hosts = [];
// host: { active(), confirm(plan), patch(), picks(), clear(), focus(), fill?(take) }. patch() gives {
// src: item, dsts: [item], levels: [ui] } (levels [] = default), or null while the boxes hold
// no patch. picks() gives the same with src null while Source has no pick. fill(take) is desktop only.
export const presetHost = (host) => {
  hosts.push(host);
};
const shown = () => hosts.find((h) => h.active());
export const presetShown = () => !!shown();
// The layout's host, for the Grid popup (gridpopup.js), or undefined while neither layout shows.
export const boxHost = () => shown();

export function fire(scope, name) {
  const s = findPreset(scope, name);
  const host = shown();
  if (!s || !host) return;
  if (!S.srcBy.size) return showResult('warn', 'Preset not sent', 'Router names not read yet.');
  const plan = presetPlan(s, parseLevels('') || []);
  emit('itemUsed', { kind: 'preset', scope: s.scope, id: s.id, name: s.name });
  if (!plan.items.length) {
    return showResult('error', `Preset ${s.name} not sent`, plan.warnings.length ? plan.warnings.join('; ') + '.' : 'It has no rows.');
  }
  host.confirm(plan);
}

// An option holds text only, so the status is words.
const statusSuffix = (s) => {
  const st = presetStatus(s).status;
  return st ? `, ${STATUS_TEXT[st]}` : '';
};

// Shift-click (or Shift+Enter in the popup) fills the boxes and sends nothing. The boxes hold one
// source, so a preset with more rows fills the first row that the router still has.
export function fillBoxes(scope, name) {
  const s = findPreset(scope, name);
  const host = shown();
  if (!s || !host) return;
  if (!host.fill) return showResult('info', `Preset ${s.name}`, 'Fill needs the desktop layout.');
  if (!S.srcBy.size) return showResult('warn', 'Preset not filled', 'Router names not read yet.');
  const r = fillFrom(s, { src: (n) => S.srcBy.has(n), dst: (n) => S.dstBy.has(n), level: (u) => S.levelBy.has(u) }, parseLevels('') || []);
  if (!r.fill) return showResult('error', `Preset ${s.name} not filled`, 'Nothing in it exists on the router now.');
  host.fill({ src: srcItem(r.fill.src), dsts: r.fill.dsts.map((n) => dstItem(n)), levels: r.fill.levels });
  const notes = [];
  if (r.many) notes.push(`Filled 1 of ${s.items.length} rows.`);
  if (r.partial) notes.push('Left out: destinations or levels that are not on the router.');
  if (notes.length) showResult('info', `Preset ${s.name}`, notes.join(' '));
}

// Keyboard mode `p` (desktop): first bookmark chip, ←/→ in DOM order (Edit, Show all, chips);
// Enter presses, Esc goes back to Source.
on('presetStrip', () => {
  const b =
    document.querySelector('[data-presets] .sode__set:not(.sode__presets-all)') ||
    document.querySelector('[data-presets] .sode__set') ||
    document.querySelector('[data-presets] button');
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
on('presets', () => {
  if (open && target.own) renderEditor(true);
});

const pick = el('select', 'pb-select');
pick.setAttribute('aria-label', 'Preset');
const nameIn = el('input', 'pb-input');
nameIn.maxLength = 32;
nameIn.autocomplete = 'off';
const scopeIn = el('select', 'pb-select');
scopeIn.setAttribute('aria-label', 'Save for');
const scopeOpts = () => scopeOptions(!!profileName()).map(([id, label]) => new Option(label, id));
scopeIn.append(...scopeOpts());
const rowsBox = el('div', 'sode__preset-rows');
const noteEl = el('span', 'sode__hint');
noteEl.setAttribute('aria-live', 'polite');
const delBtn = btn('pb-btn--outline', 'Delete', () => deleteClick());
const saveBtn = btn('pb-btn--commit', 'Save', () => save());
const linkUI = itemLinkBox();
const saveAsBtn = btn('pb-btn--outline', 'Save as…', () => askSaveAs());
const asName = el('input', 'pb-input');
asName.maxLength = 32;
asName.autocomplete = 'off';
const asScope = el('select', 'pb-select');
asScope.setAttribute('aria-label', 'Save the copy for');
const asRow = el(
  'div',
  'sode__filters-set',
  labeled('Name of the copy', asName),
  labeled('Save the copy for', el('span', 'pb-select-wrap', asScope), 'sode__filters-scope'),
  btn('pb-btn--commit', 'Save', () => saveAs()),
  btn('pb-btn--ghost', 'Cancel', () => closeSaveAs()),
);
asRow.hidden = true;
// Only presets on the server have an Item link: a This browser preset is in no other browser.
const hasItemLink = () => !!cur?.old && (cur.from === 'global' || cur.from === 'user');
const linkBtn = btn('pb-btn--outline', 'Item link', () => {
  const p = hasItemLink() && findPreset(cur.from, cur.old);
  if (p?.id) linkUI.showPreset(p.id);
});

const pickRow = el(
  'div',
  'sode__filters-set',
  labeled('Preset', el('span', 'pb-select-wrap', pick)),
  btn('pb-btn--outline', 'New', () => load(null)),
);
const scopeWrap = labeled('Save for', el('span', 'pb-select-wrap', scopeIn), 'sode__filters-scope');
const organizerBtn = btn('pb-btn--outline pb-btn--sm sode__head-link', 'Preset Organizer', () => {
  closePresets();
  if (!open) emit('presetPopup');
});
const cancelBtn = btn('pb-btn--outline', 'Cancel', () => closePresets());
const dlg = modal({
  titleId: 'presets-title',
  title: 'Edit Presets',
  cls: 'sode__filters sode__presets-dlg',
  head: [organizerBtn],
  body: el(
    'div',
    'pb-modal__body sode__filters-body',
    el('div', 'sode__filters-sets', pickRow, el('div', 'sode__filters-set', labeled('Name', nameIn), scopeWrap, saveAsBtn, linkBtn, delBtn), asRow, linkUI.box),
    el(
      'div',
      'sode__filters-set',
      btn('pb-btn--outline', 'Add row', () => {
        cur.items.push(blankRow());
        renderRows();
        focusLast();
      }),
      btn('pb-btn--outline', 'Add from Patch Boxes…', addFromBoxes),
      btn('pb-btn--outline', 'Add from History…', fillFromHistory),
    ),
    rowsBox,
  ),
  foot: [noteEl, cancelBtn, saveBtn],
  onEsc: () => closePresets(),
});
const { dialog, title: titleEl } = dlg;

function setNote(text, error = false) {
  noteEl.textContent = text;
  noteEl.classList.toggle('sode__error-text', error);
  paintSave();
}

function paintSave() {
  if (!cur) return;
  let state = 'commit';
  if (noteEl.classList.contains('sode__error-text')) state = 'danger';
  else if (dirty) state = built(!!cur.old && !!cur.from && cur.scope !== cur.from).err ? 'danger' : 'ok';
  for (const s of ['commit', 'danger', 'ok']) saveBtn.classList.toggle(`pb-btn--${s}`, s === state);
}

const saver = globalSaver({
  t: 'presets',
  resultT: 'presetsResult',
  setNote,
  isOpen: () => open,
  failTitle: 'Presets not saved',
  okNote: 'Saved for Everyone.',
  render: () => renderHead(true),
});
const profSaver = profileSaver({
  setNote,
  isOpen: () => open,
  failTitle: 'Presets not saved',
  okNote: 'Saved for Me.',
  render: () => renderHead(true),
});

// 'openPresetEditor' swaps in a profile target (editortargets.js): one preset of that user's Profile, saved there.
const browser = browserPresets({
  list: (scope) => presetsIn(scope),
  note: (c) => (c.old ? scopeLabel(c.from) : ''),
  save: saveHere,
  onClose() {
    profSaver.close();
    saver.close();
  },
});
let target = browser;
const tgtSaver = profileSaver({
  target: () => target.edit,
  setNote,
  isOpen: () => open && !!target.edit,
  failTitle: 'Presets not saved',
  okNote: 'Saved.',
  render: () => renderHead(true),
});
const { saveIn, removeIn } = scopeWriter({
  global: saver,
  user: profSaver,
  target: () => target.saver,
  key: 'presets',
  listOf: (scope) => presetsIn(scope).map(saved),
  same: (a, b) => a.toLowerCase() === b.toLowerCase(),
  dropLocal: deleteLocalPreset,
});

let open = false,
  back = null;
let cur = null,
  dirty = false,
  deleting = false,
  discard = false;
const touch = () => {
  dirty = true;
  discard = false;
  paintSave();
};
const SRC_EXAMPLE = '12 or Cam 1';
const DST_EXAMPLE = '12, 20-24, Monitor 1';
const blankRow = () => ({ src: '', dsts: '', levels: '' });

const display = (n) => String(n + off());
function rowFromItem(it) {
  return { src: display(it.src), dsts: it.dsts.map(display).join(', '), levels: it.levels.length ? levelInput(it.levels) : '' };
}
function load(s) {
  linkUI.box.hidden = true;
  asRow.hidden = true;
  deleting = false;
  dirty = false;
  discard = false;
  cur = s
    ? { id: s.id, was: s.items, scope: s.scope, from: s.scope, old: s.name, name: s.name, items: s.items.map(rowFromItem) }
    : { id: '', was: [], scope: 'local', from: '', old: '', name: '', items: [blankRow()] };
  renderEditor();
}

const lookup = (list) => (t) => search(list, t, 1)[0]?.n ?? null;
function parseRow(r) {
  const s = parseRefs(r.src, off(), lookup(S.srcs));
  const d = parseRefs(r.dsts, off(), lookup(S.dsts));
  let levels = [],
    badLevels = false;
  if (r.levels.trim()) {
    const l = parseLevels(r.levels);
    if (l) levels = l;
    else badLevels = true;
  }
  const bad = [...s.bad.map((t) => `source “${t}”`), ...d.bad.map((t) => `destination “${t}”`), ...(badLevels ? ['levels'] : [])];
  if (s.ns.length > 1) bad.push('more than one source');
  const item = { src: s.ns.length === 1 ? s.ns[0] : -1, dsts: d.ns, levels };
  const known = S.srcBy.has(item.src) && item.dsts.every((n) => S.dstBy.has(n));
  const text = bad.length
    ? `Not understood: ${bad.join(', ')}`
    : !item.dsts.length && item.src < 0
      ? ''
      : `${item.src >= 0 ? srcItem(item.src).label : '?'} → ${item.dsts.map((n) => dstItem(n).label).join(', ') || '?'}${known ? '' : ' (not on the router now)'}`;
  return { item, text, error: bad.length > 0, known };
}

function renderRows() {
  rowsBox.replaceChildren(
    ...cur.items.map((r, i) => {
      const hint = el('span', 'sode__hint sode__preset-hint');
      const upd = () => {
        const p = parseRow(r);
        hint.textContent = p.text;
        hint.classList.toggle('sode__error-text', p.error || (!p.known && !!p.text));
      };
      const field = (key, ph, label, text) => {
        const inp = el('input', 'pb-input');
        inp.value = r[key];
        inp.placeholder = ph;
        inp.autocomplete = 'off';
        inp.spellcheck = false;
        inp.setAttribute('aria-label', label);
        inp.addEventListener('input', () => {
          r[key] = inp.value;
          touch();
          upd();
          setNote('');
        });
        return labeled(text, inp);
      };
      upd();
      return el(
        'div',
        `sode__preset-row${i ? ' sode__preset-row--bare' : ''}`,
        field('src', SRC_EXAMPLE, `Row ${i + 1} source`, 'Source'),
        field('dsts', DST_EXAMPLE, `Row ${i + 1} destinations`, 'Destinations'),
        field('levels', S.levels.length ? `${S.levels[0].label}-${S.levels[S.levels.length - 1].label}` : '', `Row ${i + 1} levels`, 'Levels'),
        btn('pb-btn--ghost', 'Remove', () => {
          cur.items.splice(i, 1);
          touch();
          renderRows();
        }),
        hint,
      );
    }),
  );
}
function focusLast() {
  const ins = rowsBox.querySelectorAll('.sode__preset-row:last-child input');
  ins[0]?.focus();
}

// keep: what is being typed (the name) stays as it is.
function renderHead(keep = false) {
  pick.replaceChildren(
    new Option('New preset…', ''),
    ...allPresets().map((s) => new Option(`${s.name} (${scopeLabel(s.scope)})${statusSuffix(s)}`, `${s.scope}:${s.name}`)),
  );
  pick.value = cur.old ? `${cur.from}:${cur.old}` : '';
  if (!keep) nameIn.value = cur.name;
  scopeIn.replaceChildren(...scopeOpts());
  scopeIn.value = [...scopeIn.options].some((o) => o.value === cur.scope) ? cur.scope : 'local';
  cur.scope = scopeIn.value;
  delBtn.hidden = !cur.old;
  linkBtn.hidden = !hasItemLink();
  delBtn.textContent = deleting ? 'Confirm delete' : 'Delete';
  saveBtn.disabled = saver.busy() || profSaver.busy() || tgtSaver.busy();
  const own = target.own;
  pickRow.hidden = !own;
  scopeWrap.hidden = !own;
  saveAsBtn.hidden = !own;
  delBtn.hidden ||= !own;
  linkBtn.hidden ||= !own;
  asRow.hidden ||= !own;
  linkUI.box.hidden ||= !own;
  paintSave();
  titleEl.textContent = target.title;
  organizerBtn.hidden = !own;
}

// keepEdit: the lists changed. Only the picker redraws, never the rows or the name.
function renderEditor(keepEdit = false) {
  if (keepEdit && cur?.old && !findPreset(cur.from, cur.old) && !dirty) return load(null);
  renderHead(keepEdit);
  if (keepEdit) return;
  renderRows();
  setNote(target.note(cur));
}
pick.addEventListener('change', () => {
  const v = pick.value;
  if (!v) return load(null);
  const i = v.indexOf(':');
  load(findPreset(v.slice(0, i), v.slice(i + 1)));
});
nameIn.addEventListener('input', () => {
  cur.name = nameIn.value;
  touch();
  setNote('');
});
scopeIn.addEventListener('change', () => {
  cur.scope = scopeIn.value;
  if (!cur.old) return;
  touch();
  setNote(cur.scope === cur.from ? scopeLabel(cur.from) : `Save moves it from ${scopeLabel(cur.from)} to ${scopeLabel(cur.scope)}.`);
});

// The editor hides while the patch boxes stay live (historypick.js).
let added = 0;
function addFromBoxes() {
  if (!shown()) return setNote('The patch boxes are not showing.', true);
  picking = true;
  added = 0;
  dlg.hide();
  startPick({
    mode: 'boxes',
    label: cur.name,
    onCancel: () => {
      picking = false;
      if (!open) return;
      dlg.show();
      dialog.focus();
      if (added) setNote(`Added ${added} row${added === 1 ? '' : 's'} from the patch boxes.`);
    },
    onAddPatch: () => {
      const host = shown();
      const p = host?.patch();
      if (!p) return 'Pick a Source and a Destination first.';
      addRows([{ src: p.src.n, dsts: p.dsts.map((d) => d.n), levels: p.levels }], '');
      host.clear();
      added++;
      return `Added ${added} row${added === 1 ? '' : 's'}`;
    },
  });
  shown().focus();
}
// A lock ends the pick first ('locking' below).
let picking = false;
function fillFromHistory() {
  if (!S.hist.length) return setNote('No takes made in Sode yet.', true);
  picking = true;
  dlg.hide();
  const back = () => {
    picking = false;
    if (!open) return;
    dlg.show();
    dialog.focus();
  };
  startPick({
    label: cur.name,
    ok: canRedo,
    onCancel: back,
    onAdd: (entries) => {
      back();
      const items = itemsFromEntries(entries);
      addRows(items, `Added ${entries.length} ${entries.length === 1 ? 'entry' : 'entries'} from History.`);
    },
  });
}
function addRows(items, note) {
  if (cur.items.length === 1 && !cur.items[0].src && !cur.items[0].dsts) cur.items = [];
  cur.items.push(...items.map(rowFromItem));
  touch();
  renderRows();
  setNote(note);
}

// The preset the editor holds as name, in list: { s, next } or { err }. dupes: names it must not have; replace: the entry it overwrites.
// id: '' for a Save as, so it gets a new one.
function build(name, list, { dupes, old, replace, id = cur.id }) {
  const items = [];
  for (const r of cur.items.filter((x) => !isBlankRow(x))) {
    const p = parseRow(r);
    if (p.error) return { err: p.text };
    items.push(p.item);
  }
  const s = normPreset({ id, name, items: carryNames(items, cur.was) });
  const err = nameProblem(s.name, dupes, { noun: 'preset', old }) || presetProblem(s);
  if (err) return { err };
  const next = replacePreset(list, s, replace);
  const tooMuch = presetsProblem(next);
  return tooMuch ? { err: tooMuch } : { s, next };
}

function built(moving = false) {
  const list = target.list(cur.scope);
  const old = moving ? '' : cur.old;
  return build(cur.name, list, { dupes: list.map((x) => x.name), old, replace: old });
}

function save() {
  const moving = !!cur.old && !!cur.from && cur.scope !== cur.from;
  const b = built(moving);
  if (b.err) return setNote(b.err, true);
  target.save(b, moving);
}

// Saves the built preset into This browser, everyone's or Me, and moves it when the scope changed.
function saveHere(b, moving) {
  const from = cur.from,
    was = cur.old,
    to = cur.scope;
  // Copy first, remove after: a refused or lost save never loses the preset.
  const landed = () => {
    cur.from = to;
    cur.id = b.s.id || cur.id;
    cur.old = b.s.name;
    cur.name = b.s.name;
    nameIn.value = cur.name;
    dirty = false;
    if (!moving) return finish();
    removeIn(from, was, () => {
      cur.scope = to;
      finish();
    });
  };
  const finish = () => {
    dirty = false;
    closePresets();
  };
  if (to === 'local') {
    saveLocalPreset(b.s, moving ? '' : cur.old);
    // The server renames Everyone's and Me bookmarks in its own save.
    if (!moving && was && was !== b.s.name) renameLocalBookmarks(`preset:local:${cur.id}`, `preset:local:${cur.id}`, b.s.name);
    return landed();
  }
  saveIn(to, b.next, landed);
}

function saveThere(b) {
  saveIn(cur.scope, b.next, () => {
    dirty = false;
    closePresets();
  });
}

function askSaveAs() {
  asScope.replaceChildren(...scopeOpts());
  asScope.value = 'local';
  asName.value = saveAsName(cur.name, cur.old ? cur.from : '');
  asName.classList.remove('pb-input--invalid');
  asRow.hidden = false;
  asName.select();
}
function closeSaveAs() {
  asRow.hidden = true;
  saveAsBtn.focus();
}
function saveAs() {
  const name = asName.value.trim().slice(0, 32);
  const dest = asScope.value;
  const bad = (msg) => {
    asName.classList.add('pb-input--invalid');
    setNote(msg, true);
    asName.focus();
  };
  const list = presetsIn(dest);
  const taken = list.find((x) => x.name.toLowerCase() === name.toLowerCase())?.name || '';
  const b = build(name, list, { dupes: [], replace: taken, id: '' });
  if (b.err) return bad(b.err);
  const { s, next } = b;
  const loaded = () => {
    closeSaveAs();
    cur.scope = dest;
    cur.from = dest;
    cur.id = s.id;
    cur.old = s.name;
    cur.name = s.name;
    dirty = false;
    discard = false;
    renderHead();
  };
  if (dest === 'local') {
    saveLocalPreset(s, taken);
    loaded();
    return setNote('Saved for This browser.');
  }
  saveIn(dest, next, loaded);
}
asName.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    e.preventDefault();
    saveAs();
  }
  if (e.key === 'Escape') {
    e.preventDefault();
    e.stopPropagation();
    closeSaveAs();
  }
});

function deleteClick() {
  if (!cur.old) return;
  if (!deleting) {
    deleting = true;
    delBtn.textContent = 'Confirm delete';
    return;
  }
  deleting = false;
  const name = cur.old;
  const local = cur.from === 'local';
  removeIn(cur.from, name, () => {
    load(null);
    if (local) setNote('Deleted.');
  });
}

export function openPresets(scope = '', name = '') {
  back = document.activeElement;
  target = browser;
  const s = scope && findPreset(scope, name);
  open = true;
  load(s || null);
  dlg.show();
  dialog.focus();
}

function closePresets() {
  if (open && dirty && !discard) {
    discard = true;
    return setNote('Unsaved changes. Close again to discard them, or Save.', true);
  }
  open = false;
  dlg.hide();
  target.close();
  target = browser;
  if (back?.isConnected) back.focus();
}

// A lock closes the editor without the unsaved-changes warning (the page is covered).
// The Palette does not open over unsaved edits in the editor.
on('unsavedAsk', (a) => {
  if (open && dirty) a.unsaved = true;
});
on('locking', () => {
  if (picking) {
    picking = false;
    endPick();
  }
  if (!open) return;
  discard = true;
  closePresets();
});

on('editPresetAt', (m) => openPresets(m.scope, m.name));

on('openPresetEditor', (m) => {
  const t = editTarget(m, 'presets');
  if (!t) return;
  target = profilePresets(t, { save: saveThere, saver: tgtSaver });
  back = document.activeElement;
  open = true;
  const found = t.found;
  load(found ? { scope: 'user', ...storedPreset(found) } : null);
  dlg.show();
  dialog.focus();
});
