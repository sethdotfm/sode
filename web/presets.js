// Presets in the page: the buttons above Recent (both layouts) and the
// editor dialog. Pressing a button builds a plan and the layout showing opens
// its normal confirm; the confirm sends one batch. Storage and the pure
// parts are presetstore.js, the plan is plans.js.

import { el, S, on, emit, off, search, srcItem, dstItem, nameProblem, icon } from './core.js';
import { mac } from './keys.js';
import { levelInput, parseLevels } from './levelview.js';
import { showResult, presetPlan, canRedo } from './plans.js';
import { startPick, endPick } from './recentpick.js';
import { globalSaver, profileSaver } from './globalsave.js';
import { profileName, editTarget } from './profile.js';
import { trapDialog, labeled } from './widgets.js';
import { shareRow, shareLink, sharePresetDoc } from './transfer.js';
import { saveAsName } from './exportfmt.js';
import { scopeLabel, scopeOptions } from './scopes.js';
import {
  allPresets, presetsIn, findPreset, normPreset, presetProblem, presetsProblem, replacePreset, saveLocalPreset, deleteLocalPreset,
  pinnedPresets, saved, clearPinOverride,
  parseRefs, itemsFromTakes, presetTip, isBlankRow, fillFrom,
} from './presetstore.js';

// ---- the layouts: each registers once what it has

const hosts = [];
// host: { active(), confirm(plan), patch(), clear(), focus(), fill?(take) }.
// patch() gives the patch boxes as { src: item, dsts: [item], levels: [ui] }
// (levels [] = the default levels), or null while they hold no patch. clear()
// empties the boxes, focus() starts at Source. fill(take), same shape as
// patch(), puts a patch in the boxes: only the desktop has it.
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

const phone = () => document.documentElement.dataset.pbLayout === 'phone';
const askPopup = () => emit('presetPopup');

// Shift-click (or Shift+Enter in the popup): the preset's patch goes into the
// patch boxes and nothing is sent. The boxes hold one source, so a preset of
// several takes fills the first one that still exists, and says so; what of
// that take no longer exists is left out, and said too.
export function fillBoxes(scope, name) {
  const s = findPreset(scope, name);
  const host = shown();
  if (!s || !host) return;
  if (!host.fill) return showResult('info', `Preset ${s.name}`, 'Filling the boxes from a preset works on the desktop layout.');
  if (!S.srcBy.size) return showResult('warn', 'Preset not filled', 'The router’s names have not loaded yet.');
  const r = fillFrom(s, { src: (n) => S.srcBy.has(n), dst: (n) => S.dstBy.has(n), level: (u) => S.levelBy.has(u) }, parseLevels('') || []);
  if (!r.take) return showResult('error', `Preset ${s.name} not filled`, 'Nothing in it exists on the router now.');
  host.fill({ src: srcItem(r.take.src), dsts: r.take.dsts.map((n) => dstItem(n)), levels: r.take.levels });
  const notes = [];
  if (r.many) notes.push(`The boxes hold one source, so only one of its ${s.items.length} takes was filled.`);
  if (r.partial) notes.push('Some of its destinations or levels no longer exist on the router and were left out.');
  if (notes.length) showResult('info', `Preset ${s.name}`, notes.join(' '));
}

function renderStrips() {
  const all = allPresets();
  const list = pinnedPresets(all);
  for (const box of document.querySelectorAll('[data-presets]')) {
    const edit = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__chipbtn', all.length ? 'Edit' : 'Presets…');
    edit.type = 'button';
    edit.setAttribute('aria-haspopup', 'dialog');
    edit.addEventListener('click', () => openPresets());
    const chips = list.map((s, i) => {
      const b = el('button', 'pb-chip sode__set', s.name);
      // Option/Alt+1-9 fire the first nine: the key slides in over the chip while Option is held.
      if (i < 9) {
        const k = `${mac ? '⌥' : 'Alt+'}${i + 1}`;
        b.append(el('span', 'sode__chip-key', el('kbd', 'sode__key-note', k)));
        b.setAttribute('aria-keyshortcuts', `Alt+${i + 1}`);
      }
      b.type = 'button';
      // Names come from the router and change (a re-poll, a late load): the tip is
      // worked out when the pointer or focus arrives, not only when the strip is drawn.
      const tip = () => { b.dataset.pbTip = presetTip(s, (n) => srcItem(n).label, (n) => dstItem(n).label); };
      tip();
      b.addEventListener('pointerenter', tip);
      b.addEventListener('focus', tip);
      b.addEventListener('click', (e) => (e.shiftKey ? fillBoxes(s.scope, s.name) : fire(s.scope, s.name)));
      return b;
    });
    // The popup (presetpopup.js) lists every preset, pinned or not.
    // A plain chip with a list icon first in the strip (not the selected style: it
    // opens a list, it is not a state), so it is the first stop for the arrow keys.
    const more = all.length ? el('button', 'pb-chip sode__set sode__presets-all', icon('list'), 'Show all') : null;
    if (more) {
      more.type = 'button';
      more.title = phone() ? 'Show all presets' : 'Show all presets (Ctrl+G)';
      more.setAttribute('aria-haspopup', 'dialog');
      more.addEventListener('click', askPopup);
    }
    // A rule after Show all sets it apart from the pinned chips.
    const sep = () => {
      const x = el('span', 'sode__sets-sep', '|');
      x.setAttribute('aria-hidden', 'true');
      return x;
    };
    // Laid out like Recent's bar: the title, then Edit and the chips (Show all
    // pinned, the pinned presets scrolling).
    box.replaceChildren(
      el('span', 'sode__label', 'Presets'),
      edit,
      el('div', 'sode__sets', ...(more ? [more, ...(chips.length ? [sep()] : [])] : []), el('div', 'sode__sets-run', ...chips)),
    );
  }
}
// Keyboard mode `p` (desktop): the first preset chip (not Show all), then ←/→
// between buttons in DOM order (Show all, the chips, Edit);
// Enter presses one (the normal confirm), Esc goes back to Source.
on('presetStrip', () => {
  const b = document.querySelector('[data-presets] .sode__set:not(.sode__presets-all)') || document.querySelector('[data-presets] .sode__set') || document.querySelector('[data-presets] button');
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
on('presets', () => { renderStrips(); if (open && !tgt) renderEditor(true); });
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
const scopeOpts = () => scopeOptions(!!profileName()).map(([id, label]) => new Option(label, id));
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
const pinOk = btn('pb-btn--ok', 'Unlock');
const pinRow = el('div', 'sode__filters-set', labeled('Settings PIN', pinIn), pinOk, btn('pb-btn--ghost', 'Cancel', () => saver.cancel()));
pinRow.hidden = true;
const delBtn = btn('pb-btn--outline', 'Delete', () => deleteClick());
const saveBtn = btn('pb-btn--commit', 'Save', () => save());
// A link to the saved preset (names ride along; opening it shows a review).
const shareUI = shareRow();
const saveAsBtn = btn('pb-btn--outline', 'Save as…', () => askSaveAs());
// Save as: the preset as typed, under another name, in the scope chosen here.
const asName = el('input', 'pb-input');
asName.maxLength = 32;
asName.placeholder = 'Name';
asName.setAttribute('aria-label', 'Name for the copy');
asName.autocomplete = 'off';
const asScope = el('select', 'pb-select');
asScope.setAttribute('aria-label', 'Save the copy for');
const asRow = el('div', 'sode__filters-set', labeled('Name of the copy', asName), labeled('Save the copy for', el('span', 'pb-select-wrap', asScope), 'sode__filters-scope'),
  btn('pb-btn--commit', 'Save', () => saveAs()), btn('pb-btn--ghost', 'Cancel', () => closeSaveAs()));
asRow.hidden = true;
const shareBtn = btn('pb-btn--outline', 'Share', () => {
  const p = cur?.old && findPreset(cur.from, cur.old);
  if (p) shareLink(sharePresetDoc(p), shareUI);
});

const pickRow = el('div', 'sode__filters-set', labeled('Preset', el('span', 'pb-select-wrap', pick)), btn('pb-btn--outline', 'New', () => load(null)));
const scopeWrap = labeled('Save for', el('span', 'pb-select-wrap', scopeIn), 'sode__filters-scope');
const titleEl = Object.assign(el('span', 'pb-modal__title', 'Edit Presets'), { id: 'presets-title' });
// To the organizer (the Presets popup): pin, order, fire. Closing here keeps the unsaved-changes warning.
const organizerBtn = btn('pb-btn--outline pb-btn--sm sode__head-link', 'Preset Organizer', () => { closePresets(); if (!open) emit('presetPopup'); });
const transferBtn = btn('pb-btn--outline', 'Import & Export…', () => emit('openTransfer', { scope: cur?.old ? cur.from : 'local', user: profileName() || '', what: 'presets' }));
const cancelBtn = btn('pb-btn--outline', 'Cancel', () => closePresets());
dialog.append(
  el('div', 'pb-modal__header', titleEl, organizerBtn),
  el('div', 'pb-modal__body sode__filters-body',
    el('div', 'sode__filters-sets',
      pickRow,
      el('div', 'sode__filters-set', labeled('Name', nameIn), scopeWrap, saveAsBtn, shareBtn, delBtn),
      asRow, pinnedBox, pinRow, shareUI.box),
    el('div', 'sode__filters-set',
      btn('pb-btn--outline', 'Add row', () => { cur.items.push(blankRow()); renderRows(); focusLast(); }),
      btn('pb-btn--outline', 'Add from Patch Boxes…', addFromBoxes),
      btn('pb-btn--outline', 'Add from Recent…', fillFromRecent)),
    rowsBox),
  el('div', 'pb-modal__footer', transferBtn, noteEl, cancelBtn, saveBtn),
);
root.append(dialog);
document.body.append(root);

function setNote(text, error = false) {
  noteEl.textContent = text;
  noteEl.classList.toggle('sode__error-text', error);
  paintSave();
}

// Save is green when what is typed can be saved, red when it can't (or the last
// try said why), plain while nothing has changed.
function paintSave() {
  if (!cur) return;
  let state = 'commit';
  if (noteEl.classList.contains('sode__error-text')) state = 'danger';
  else if (dirty) state = built(!!cur.old && !!cur.from && cur.scope !== cur.from).err ? 'danger' : 'ok';
  for (const s of ['commit', 'danger', 'ok']) saveBtn.classList.toggle(`pb-btn--${s}`, s === state);
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

// Aimed at a profile (bus 'openPresetEditor', from the Profile dialog): the
// same editor on one preset of that user's profile, with the picker, scope,
// Save as, Share, Delete and Export hidden; the save goes to that profile.
// editTarget(...) from profile.js; null for the usual editor.
let tgt = null;
const tgtSaver = profileSaver({
  target: () => tgt,
  setNote, isOpen: () => open && !!tgt, failTitle: 'Presets not saved', okNote: 'Saved.',
  render: () => renderHead(true),
});

let open = false, back = null;
// cur: the preset being edited: { scope, old, name, items: [{ src, dsts, levels }] } as typed text.
let cur = null, dirty = false, deleting = false, discard = false;
// An edit: unsaved, and a close warning given earlier no longer covers it.
const touch = () => { dirty = true; discard = false; paintSave(); };
// Examples: a source takes one number or name; destinations take a list.
const SRC_EXAMPLE = '12 or Cam 1';
const DST_EXAMPLE = '12, 20-24, Monitor 1';
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
    ? { scope: s.scope, from: s.scope, old: s.name, name: s.name, pinned: s.pinned !== false, items: s.items.map(rowFromItem) }
    : { scope: 'local', from: '', old: '', name: '', pinned: true, items: [blankRow()] };
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
    const field = (key, ph, label, text) => {
      const inp = el('input', 'pb-input');
      inp.value = r[key];
      inp.placeholder = ph;
      inp.autocomplete = 'off';
      inp.spellcheck = false;
      inp.setAttribute('aria-label', label);
      inp.addEventListener('input', () => { r[key] = inp.value; touch(); upd(); setNote(''); });
      return labeled(text, inp);
    };
    upd();
    // Labels show on the first row only; later rows keep their aria-labels.
    return el('div', `sode__preset-row${i ? ' sode__preset-row--bare' : ''}`,
      field('src', SRC_EXAMPLE, `Take ${i + 1} source`, 'Source'),
      field('dsts', DST_EXAMPLE, `Take ${i + 1} destinations`, 'Destinations'),
      field('levels', S.levels.length ? `Levels (${S.levels[0].label}-${S.levels[S.levels.length - 1].label})` : 'Levels', `Take ${i + 1} levels`, 'Levels'),
      btn('pb-btn--ghost', 'Remove', () => { cur.items.splice(i, 1); touch(); renderRows(); }),
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
  pick.replaceChildren(new Option('New preset…', ''), ...allPresets().map((s) => new Option(`${s.name} (${scopeLabel(s.scope)})`, `${s.scope}:${s.name}`)));
  pick.value = cur.old ? `${cur.from}:${cur.old}` : '';
  if (!keep) nameIn.value = cur.name;
  // Rebuilt so that 'Me' follows who is logged in; a Me preset of someone else is gone from the list anyway.
  scopeIn.replaceChildren(...scopeOpts());
  scopeIn.value = [...scopeIn.options].some((o) => o.value === cur.scope) ? cur.scope : 'local';
  cur.scope = scopeIn.value;
  pinnedIn.checked = cur.pinned;
  delBtn.hidden = !cur.old;
  shareBtn.hidden = !cur.old;
  delBtn.textContent = deleting ? 'Really delete?' : 'Delete';
  saveBtn.disabled = saver.busy() || profSaver.busy() || tgtSaver.busy();
  const t = !!tgt;
  pickRow.hidden = t;
  scopeWrap.hidden = t;
  pinRow.hidden = pinRow.hidden || t;
  saveAsBtn.hidden = t;
  transferBtn.hidden = t;
  if (t) { delBtn.hidden = true; shareBtn.hidden = true; asRow.hidden = true; shareUI.box.hidden = true; }
  paintSave();
  titleEl.textContent = t ? `${tgt.old ? 'Edit' : 'Add'} preset for ${tgt.user}` : 'Edit Presets';
  organizerBtn.hidden = !!t;
}

// keepEdit: the lists changed under us (a reconnect, another browser's
// save); only the picker is refreshed, never the rows or name being typed.
function renderEditor(keepEdit = false) {
  if (keepEdit && cur?.old && !findPreset(cur.from, cur.old) && !dirty) return load(null);
  renderHead(keepEdit);
  if (keepEdit) return;
  renderRows();
  setNote(cur.old && !tgt ? scopeLabel(cur.from) : '');
}
pick.addEventListener('change', () => {
  const v = pick.value;
  if (!v) return load(null);
  const i = v.indexOf(':');
  load(findPreset(v.slice(0, i), v.slice(i + 1)));
});
nameIn.addEventListener('input', () => { cur.name = nameIn.value; touch(); setNote(''); });
pinnedIn.addEventListener('change', () => { cur.pinned = pinnedIn.checked; touch(); });
// A saved preset can change scope: Save copies it to the new one, then removes it from the old.
scopeIn.addEventListener('change', () => {
  cur.scope = scopeIn.value;
  if (!cur.old) return;
  touch();
  setNote(cur.scope === cur.from ? scopeLabel(cur.from) : `Save moves it from ${scopeLabel(cur.from)} to ${scopeLabel(cur.scope)}.`);
});

// Add from Patch Boxes…: the editor steps aside, the patch boxes stay live and
// the bar's Add (or Enter) turns what is in them into a row; Done comes back.
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
  shown().focus(); // start at Source
}
// The editor steps aside while Recent is in pick mode (recentpick.js); Add or
// Cancel brings it back. A lock ends the pick first (see 'locking' below).
let picking = false;
function fillFromRecent() {
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
    onAdd: (takes) => {
      back();
      const items = itemsFromTakes(takes);
      addRows(items, `Added ${takes.length} take${takes.length === 1 ? '' : 's'} from Recent.`);
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

// ---- saving

// The typed rows as a preset, or an error line. next: the scope's list as
// it would be saved.
// moving: saved under another scope, so it is a new preset there (a name taken there is refused).
function built(moving = false) {
  const items = [];
  for (const r of cur.items.filter((x) => !isBlankRow(x))) {
    const p = parseRow(r);
    if (p.error) return { err: p.text };
    items.push(p.item);
  }
  const s = { ...normPreset({ name: cur.name, items }), pinned: cur.pinned };
  const list = tgt ? tgt.lists : presetsIn(cur.scope);
  const err = nameProblem(s.name, list.map((x) => x.name), { noun: 'preset', old: moving ? '' : cur.old }) || presetProblem(s);
  if (err) return { err };
  const next = replacePreset(list, s, moving ? '' : cur.old);
  const tooMuch = presetsProblem(next);
  return tooMuch ? { err: tooMuch } : { s, next };
}

// Takes preset `name` out of one scope (a move's second step).
function removeFrom(scope, name, done) {
  if (scope === 'local') { deleteLocalPreset(name); return done(); }
  const next = presetsIn(scope).filter((x) => x.name.toLowerCase() !== name.toLowerCase()).map(saved);
  (scope === 'user' ? profSaver : saver).change({ presets: next }, done);
}

function save() {
  const moving = !!cur.old && !!cur.from && cur.scope !== cur.from;
  const b = built(moving);
  if (b.err) return setNote(b.err, true);
  if (tgt) {
    return tgtSaver.change({ presets: b.next }, () => { dirty = false; closePresets(); });
  }
  const from = cur.from, was = cur.old, to = cur.scope;
  // Copy first, remove after: a refused or lost save never loses the preset.
  const landed = () => {
    cur.from = to; cur.old = b.s.name; cur.name = b.s.name; nameIn.value = cur.name; dirty = false;
    if (!moving) return finish();
    if (from === 'global') clearPinOverride(was);
    removeFrom(from, was, () => { cur.scope = to; finish(); });
  };
  // Save is the footer's: once the preset has landed (and a move has removed
  // the old copy) the dialog closes.
  const finish = () => { dirty = false; closePresets(); };
  if (to === 'local') {
    saveLocalPreset(b.s, moving ? '' : cur.old);
    return landed();
  }
  const sv = to === 'user' ? profSaver : saver;
  sv.change({ presets: b.next }, landed);
}

// ---- save as

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
// The name is checked against the target scope; a name taken there is replaced.
function saveAs() {
  const name = asName.value.trim().slice(0, 32);
  const target = asScope.value;
  const bad = (msg) => { asName.classList.add('pb-input--invalid'); setNote(msg, true); asName.focus(); };
  const items = [];
  for (const r of cur.items.filter((x) => !isBlankRow(x))) {
    const p = parseRow(r);
    if (p.error) return bad(p.text);
    items.push(p.item);
  }
  const err = nameProblem(name, [], { noun: 'preset' });
  if (err) return bad(err);
  const s = { ...normPreset({ name, items }), pinned: cur.pinned };
  const list = presetsIn(target);
  const taken = list.find((x) => x.name.toLowerCase() === name.toLowerCase())?.name || '';
  const perr = presetProblem(s);
  if (perr) return bad(perr);
  const next = replacePreset(list, s, taken);
  const tooMuch = presetsProblem(next);
  if (tooMuch) return bad(tooMuch);
  const loaded = () => { closeSaveAs(); cur.scope = target; cur.from = target; cur.old = s.name; cur.name = s.name; dirty = false; discard = false; renderHead(); };
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
  if (cur.from === 'local') {
    deleteLocalPreset(name);
    load(null);
    return setNote('Deleted.');
  }
  const next = presetsIn(cur.from).filter((s) => s.name.toLowerCase() !== name.toLowerCase()).map(saved);
  (cur.from === 'user' ? profSaver : saver).change({ presets: next }, () => load(null));
}

// ---- open / close


export function openPresets(scope = '', name = '') {
  back = document.activeElement;
  tgt = null;
  const s = scope && findPreset(scope, name);
  open = true;
  pinRow.hidden = true;
  load(s || null);
  dlg.show();
  dialog.focus();
}

function closePresets() {
  if (open && dirty && !discard) { discard = true; return setNote('Unsaved changes. Close again to discard them, or Save.', true); }
  open = false;
  dlg.hide();
  if (tgt) { // nothing global to finish
    tgt = null;
    if (back?.isConnected) back.focus();
    return;
  }
  profSaver.close();
  saver.close(); // a save already sent finishes first (its answer locks up after it)
  if (back?.isConnected) back.focus();
}

const dlg = trapDialog(root, dialog, closePresets);

// A lock closes the editor like the other dialogs do, without the
// unsaved-changes warning (the page is covered; the edits can't be reached).
on('locking', () => {
  if (picking) { picking = false; endPick(); }
  if (!open) return;
  discard = true;
  closePresets();
});

on('openPresetEditor', (m) => {
  tgt = editTarget(m, 'presets');
  if (!tgt) return;
  back = document.activeElement;
  open = true;
  pinRow.hidden = true;
  const found = tgt.found;
  load(found ? { scope: 'user', ...normPreset(found), pinned: found.pinned !== false } : null);
  dlg.show();
  dialog.focus();
});
