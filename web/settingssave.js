// The Settings save path: unsaved changes, the one Save (`settingsSave`) and its answer per field. The review is in
// savereview.js. A change is a difference from the form as opened or last saved. Each save source (savesources.js,
// and formSource here for the form) gives its own changes. data-apart inputs are not counted.

import { $, el, S, on, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
import { fmtTime } from './device.js';
import { layout } from './layout.js';
import { askRecheck } from './recheck.js';
import { askApproval } from './approval.js';
import { requesting, cancelRequest } from './approvalcards.js';
import { toGlobal } from './prefs.js';
import { reviewSave } from './savereview.js';
import { askInterruptions, initInterruptions } from './interruptions.js';
import {
  SAVE_FIELDS,
  diffRows,
  showValue,
  refusalNote,
  reconnects,
  reconnectTargets,
  reconnectQuestion,
  critical,
  sectionOfKey,
  unsavedWords,
} from './savereviewrules.js';
import { routerDisplay, listKey } from './routerlistrules.js';
import { saveSources, sourceOf } from './savesources.js';
import { noteAuthors, rowId, splitSeen, sentKeys, unseenNotes, namesAuthors } from './stagedrules.js';
import { sectionView, approvalWho } from './settingsrolerules.js';
import { presetMarks, presetGaps, gapText } from './accesspresetrules.js';
import { form, isOpen, readSettings, formProblem } from './settingsform.js';

const saving = () => $('settings-save').disabled;

let base = {}; // the fields as last opened or saved
let versions = {}; // each field's version as loaded
const refused = new Map(); // key -> refused result; marked until it saves or goes back
let savedAt = null;
let role = '';
let me = ''; // who stages an edit now: the user, a stepped-in user included, else Guest
let authors = new Map(); // stagedrules.js marks
let marks = new Map(); // accesspresetrules.js marks. The Access preset that staged a row.
let applied = null; // {name, target}. The last Access preset applied, until nothing differs from it.
let seq = 0;
let requested = null; // { id, sent, keys }: a save the server holds for Request approval.
let pending = null; // { id, sent, retried, answer }; sent: key -> the value sent; answer: the Approval popup's

const formOwns = (key) => key in SAVE_FIELDS;

// A data-apart input with a data-key saves on its own, so it is never a change here.
const saveFields = () => Object.fromEntries(Object.entries(readSettings()).filter(([k]) => k in SAVE_FIELDS));

// The form first: the review and the sent fields keep that order.
const sources = () => [formSource, ...saveSources()];
const srcOf = (key) => sourceOf(key) || formSource;

const changes = () => sources().flatMap((src) => diffRows(src.base(), src.now()));

export const dirty = () => isOpen() && changes().length > 0;
// The Palette does not open over unsaved Settings.
on('unsavedAsk', (a) => {
  if (dirty()) a.unsaved = true;
});

// Called on each edit, never on a Step in or Step out, so an edit keeps the name of who made it.
const noteEdits = () => {
  authors = noteAuthors(authors, changes(), me);
  marks = presetMarks(marks, changes(), null, null);
};
const markOf = (r) => marks.get(rowId(r))?.preset || '';

export function markPreset(name, target) {
  noteEdits();
  marks = presetMarks(marks, changes(), name, target);
  applied = { name, target };
  paintMarks();
  return warned() ? unwarn() : renderSaveState();
}
const by = (r) => authors.get(rowId(r))?.by || me;
// Save skips edits in a section the saver's Role cannot see; they show only as a count.
const seen = (r) => sectionView(sectionOfKey(r.key), role) !== 'hidden';
const warned = () => !$('settings-unsaved').hidden;

const SPECIAL = {
  levelColors: 'cfg-colors',
  browserDefaults: 'gcfg-theme',
  locks: 'cfg-lock-main',
  newUserDefaults: 'cfg-new-persistent',
};
// key is a form key.
const keyNode = (key) => (SPECIAL[key] ? $(SPECIAL[key]) : form.querySelector(`[data-key="${key}"]`));
// The form's own section of a node: the top fieldset around it.
const topFieldset = (node) => {
  let fs = node?.closest('fieldset[data-section]');
  while (fs && fs.parentElement !== form) fs = fs.parentElement.closest('fieldset[data-section]');
  return fs || null;
};
// The section where a key's field is: its node's, else the section of the key.
const fieldset = (key) =>
  topFieldset(srcOf(key).node?.(key)) || topFieldset(keyNode(key)) || form.querySelector(`:scope > fieldset[data-section="${sectionOfKey(key)}"]`);
const sections = () => [...form.querySelectorAll(':scope > fieldset[data-section]')];
const sectionName = (fs) => fs.querySelector(':scope > legend').firstChild.textContent.trim();
const checkText = (node) => node?.closest('.pb-check')?.querySelector('.pb-check__label')?.textContent;
const forText = (node) => node && form.querySelector(`label[for="${node.id}"]`)?.textContent;

function place(r) {
  const fs = fieldset(r.key);
  return { topic: $(`topic-${fs.dataset.topic}`)?.textContent.trim() || '', section: sectionName(fs), order: sections().indexOf(fs) };
}

const label = (r) => srcOf(r.key).label(r);
const show = (r, v) => srcOf(r.key).show(r, v);

function formLabel(r) {
  if (r.key === 'locks') return checkText($(`cfg-lock-${r.sub}`)) || r.sub;
  if (r.key === 'newUserDefaults') return `New user defaults: ${checkText($(r.sub === 'persistent' ? 'cfg-new-persistent' : 'cfg-new-no-auto-lock')) || r.sub}`;
  if (r.key === 'browserDefaults') return forText(form.querySelector(`select[data-gpref="${r.sub}"]`)) || r.sub;
  if (r.key === 'levelColors') return $(`cfg-color-${r.sub + 1}`)?.getAttribute('aria-label') || `Position ${r.sub + 1} color`;
  const node = keyNode(r.key);
  return (forText(node) || checkText(node) || r.key).trim();
}

function formShow(r, v) {
  if (r.key === 'levelColors' && r.sub != null) return v || 'Default';
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    return Object.entries(v)
      .map(([sub, x]) => `${label({ key: r.key, sub })}: ${show({ key: r.key, sub }, x)}`)
      .join(', ');
  }
  const sel = r.key === 'browserDefaults' ? form.querySelector(`select[data-gpref="${r.sub}"]`) : keyNode(r.key);
  if (sel?.tagName === 'SELECT') {
    const same = r.key === 'browserDefaults' ? (o) => toGlobal(r.sub, o.value) === v : (o) => o.value === String(v);
    const o = [...sel.options].find(same);
    if (o) return o.textContent;
  }
  return showValue(v);
}

// The form as a save source (savesources.js). node(key) places its marks; a source without one marks its section.
const formSource = {
  owns: formOwns,
  base: () => base,
  now: saveFields,
  version: (key) => versions[key],
  label: formLabel,
  show: formShow,
  node: keyNode,
  saved(key, value, r) {
    base[key] = value;
    if (r.version) versions[key] = r.version;
  },
  conflict: (key, r) => formSource.saved(key, r.value, r),
  reset() {
    base = saveFields();
    versions = { ...S.versions };
  },
};

// A read section (Shown) saves through an Approval, so its rows are sent.
const kept = (r) => (sectionView(sectionOfKey(r.key), role, layout() === 'phone') === 'hidden' ? 'your Role is not allowed to change this' : '');

function paintMarks() {
  for (const m of form.querySelectorAll('.sode__refused, .sode__preset-mark')) m.remove();
  paintPresetMarks();
  const open = new Set(changes().map((r) => r.key));
  for (const [key, r] of refused) {
    if (!open.has(key)) {
      refused.delete(key);
      continue;
    }
    const row = { key, sub: null };
    const note = refusalNote(r, (v) => show(row, v));
    const node = srcOf(key).node?.(key);
    const mark = el('div', 'pb-note pb-note--warn sode__refused', node ? note : `${label(row)}: ${note}`);
    mark.dataset.refused = key;
    if (node) (node.closest('.sode__field') || node.closest('.pb-check') || node).after(mark);
    else fieldset(key).append(mark);
  }
}

// One badge per field, or per section for a save source's fields, with the name of the preset that staged it.
function paintPresetMarks() {
  const done = new Set();
  for (const r of changes()) {
    const name = markOf(r);
    const src = srcOf(r.key);
    const at = name && (src.node ? src.node(r.key)?.closest('.sode__field, .pb-check') : fieldset(r.key));
    if (!at || done.has(at)) continue;
    done.add(at);
    const mark = el('span', 'pb-badge sode__preset-mark', name);
    // In the label: next to a field or a check, the badge would be one more grid or flex item and break the row.
    const host = src.node
      ? (at.matches('.pb-check') ? at.querySelector('.pb-check__label') : at.querySelector(':scope > label, :scope > .sode__label')) || at
      : at;
    host.append(mark);
  }
}

function unsavedText() {
  const { shown, unseen } = splitSeen(changes(), seen);
  const slugs = new Set(shown.map((r) => fieldset(r.key)?.dataset.section || sectionOfKey(r.key)));
  const names = sections()
    .filter((fs) => slugs.has(fs.dataset.section))
    .map(sectionName);
  return [unsavedWords(names), ...unseenNotes(unseen, by).map((t) => `${t}.`)].filter(Boolean).join(' ');
}

export function markClean(saved) {
  // The form last: a source's reset refills its inputs.
  for (const src of [...saveSources(), formSource]) src.reset();
  refused.clear();
  authors = new Map();
  marks = new Map();
  applied = null;
  paintMarks();
  savedAt = saved ? new Date().toISOString() : null;
  unwarn();
}

// True when it warned: the first try to leave with unsaved changes warns, a second one goes.
export function warnUnsaved() {
  if (!dirty() || warned()) return false;
  $('settings-unsaved').textContent = unsavedText();
  $('settings-unsaved').hidden = false;
  $('settings-exit').textContent = 'Discard and exit';
  renderSaveState();
  return true;
}

export function unwarn() {
  $('settings-unsaved').hidden = true;
  $('settings-exit').textContent = 'Exit';
  renderSaveState();
}

function renderSaveState() {
  // The bdi keeps the path in its own order inside the code's right-to-left cut (base.css #settings-path).
  const path = S.configPath ? el('code', { 'data-pb-tip': S.configPath }, el('bdi', { dir: 'ltr' }, S.configPath)) : null;
  // The hint stays while the Exit warning shows, so the bar keeps its shape.
  $('settings-path').replaceChildren(
    ...(dirty()
      ? ['Unsaved changes']
      : !path
        ? [savedAt ? `Applied ${fmtTime(savedAt)}. Not saved to the file: ` : 'Not saved: ', el('code', null, 'SODE_CONFIG'), ' unset']
        : savedAt
          ? [`Saved ${fmtTime(savedAt)} to `, path]
          : ['Saved to ', path]),
  );
}

export function showSaveErr(text) {
  $('settings-err').textContent = text;
  $('settings-err').hidden = false;
}

function valuesOf(keys) {
  const nows = new Map();
  const now = (src) => nows.get(src) || nows.set(src, src.now()).get(src);
  return Object.fromEntries(keys.map((k) => [k, now(srcOf(k))[k]]));
}

const fieldsOf = (keys, sent) => Object.fromEntries(keys.map((k) => [k, { value: sent[k], version: srcOf(k).version(k) ?? '' }]));

// The Access preset that staged one of these fields, for the Settings saves row.
function presetOf(keys) {
  const r = changes().find((r) => keys.includes(r.key) && markOf(r));
  return r && { accessPreset: markOf(r) };
}

function sendFields(keys, sent, retried, approval = null, answer = null) {
  const fields = fieldsOf(keys, sent);
  if (!keys.length) return;
  const id = ++seq;
  if (!wsSend({ t: 'settingsSave', id, fields, ...(approval && { approval }), ...presetOf(keys) })) {
    if (!answer) showSaveErr(LINK_DOWN);
    return false;
  }
  pending = { id, sent, retried, answer };
  $('settings-save').disabled = true;
  return true;
}

// Request approval: an edit to what the request covers withdraws it, so no one approves something else.
function withdrawIfEdited() {
  if (!requested || !requesting()) return;
  const now = valuesOf(requested.keys);
  if (requested.keys.some((k) => JSON.stringify(now[k]) !== JSON.stringify(requested.sent[k]))) cancelRequest();
}

function requestFor(keys, sent) {
  return {
    action: () => {
      const id = ++seq;
      requested = { id, sent, keys, retried: true };
      return { t: 'settingsSave', id, fields: fieldsOf(keys, sent), ...presetOf(keys) };
    },
    onEnd: (end) => {
      if (end !== 'approved') requested = null;
    },
  };
}

function settingsSaveResult(m) {
  if (requested && m.id === requested.id) {
    const p = requested;
    requested = null;
    return void settled(m, p);
  }
  if (!pending || m.id !== pending.id) return;
  const p = pending;
  pending = null;
  $('settings-save').disabled = false;
  settled(m, p);
}

function settled(m, p) {
  if (p.answer && !m.results && m.approval) return void p.answer.fail(m.err);
  p.answer?.done();
  if (!m.results) return showSaveErr(sentence(m.err) || 'Not saved.');
  let saved = 0;
  const again = [];
  const approve = [];
  for (const [key, r] of Object.entries(m.results)) {
    const src = srcOf(key);
    if (r.status === 'refused') {
      // Their value is now on the mark and in the review's Was, so the next Save overwrites it on purpose.
      if (r.reason === 'conflict' && r.version) src.conflict(key, r);
      if (r.reason === 'needs PIN recheck' && !p.retried) again.push(key);
      if (r.reason === 'needs Approval' && !p.answer) approve.push(key);
      refused.set(key, r);
      continue;
    }
    src.saved(key, p.sent[key], r);
    refused.delete(key);
    if (r.status === 'saved') saved++;
  }
  if (again.length) {
    askRecheck('Saving these changes', 'Save').then((ok) => {
      if (ok) sendFields(again, p.sent, true);
      else done(saved);
    });
    return;
  }
  done(saved);
  if (approve.length && isOpen()) {
    const who = approvalWho(approve.map(sectionOfKey));
    askApproval('Saving these changes', who, (ap, answer) => sendFields(approve, p.sent, true, ap, answer), requestFor(approve, p.sent));
  }
}

const liveNow = () => Object.assign({}, ...sources().map((src) => src.base()));

function done(saved) {
  if (saved) savedAt = new Date().toISOString();
  if (!isOpen()) return;
  paintMarks();
  const gaps = applied ? presetGaps(applied.target, liveNow()) : [];
  if (refused.size) {
    showSaveErr(`${refused.size} ${refused.size === 1 ? 'change' : 'changes'} not saved.`);
    if (gaps.length) $('settings-err').append(stillDiffers(gaps));
  } else $('settings-err').hidden = true;
  if (!gaps.length || !refused.size) applied = null;
  renderSaveState();
}

// After a partly refused save: what still differs from the applied preset, and what each does now.
function stillDiffers(gaps) {
  const items = gaps.map((g) => {
    const r = { key: g.key, sub: g.sub };
    return el('li', null, `${label(r)}: ${show(r, g.live)}. ${gapText(g)}.`);
  });
  return el('div', null, el('p', null, `Still different from ${applied.name}:`), el('ul', null, ...items));
}

// addressAfter()(uid): the address a router has after the save, from the detail views of Settings > Routers
// (settingsrouterdetail.js).
function addressAfter() {
  const list = Object.assign({}, ...sources().map((src) => src.now()));
  return (uid) => list[listKey(uid, 'address')] ?? S.routers.find((r) => r.uid === uid)?.address ?? '';
}

async function review({ focusField }) {
  const now = readSettings();
  const bad = formProblem(now);
  if (bad) {
    showSaveErr(bad.text);
    focusField(bad.node);
    return;
  }
  const srcBad = saveSources()
    .map((src) => src.problem())
    .find(Boolean);
  if (srcBad) return void showSaveErr(srcBad);
  noteEdits();
  const all = changes();
  if (!all.length) return void showSaveErr('No changes');
  const { shown: rows, unseen } = splitSeen(all, seen);
  $('settings-err').hidden = true;
  // Fields the page expects refused are sent too: the server's answer gives each its reason.
  // Edits the Role cannot see stay unsaved and stay staged; the rest are sent.
  const keys = sentKeys(all, seen);
  const interruptions = await askInterruptions(fieldsOf(keys, valuesOf(keys)));
  if (!isOpen()) return;
  const send = await reviewSave({
    interruptions,
    rows,
    unseen: unseenNotes(unseen, by),
    by: namesAuthors(rows, by, me) ? by : null,
    preset: markOf,
    kept,
    place,
    label,
    show,
    // A row that goes through an Approval needs no recheck of this browser's user: an Administrator approver's counts.
    critical: (r) => critical(r.key) && sectionView(sectionOfKey(r.key), role) === 'edit',
    criticalAction: (r) => critical(r.key),
    role,
    reconnects: (r) => reconnects(r.key),
    question: (saves) =>
      reconnectQuestion(
        reconnectTargets(
          saves.map((r) => r.key),
          S.routers,
          routerDisplay,
          addressAfter(),
        ),
        S.routers.length > 1,
      ),
    recheckNeeded: S.users.length > 0,
  });
  if (!send || !isOpen()) return;
  sendFields(keys, valuesOf(keys), false);
}

// focusField and reveal come from the table of contents (settingstoc.js), which opens a field's page on a phone.
export function initSave(toc) {
  const edited = () => {
    noteEdits();
    withdrawIfEdited();
    if (refused.size || marks.size) paintMarks();
    return warned() ? unwarn() : renderSaveState();
  };
  for (const ev of ['input', 'change']) {
    form.addEventListener(ev, () => {
      $('settings-err').hidden = true;
      edited();
    });
  }
  // A save in flight can reload the page: a new default router or prefix reloads every page (core.js). The save
  // goes on at the server, so the page does not ask.
  window.addEventListener('beforeunload', (e) => {
    if (dirty() && !pending) e.preventDefault();
  });
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    if (saving()) return;
    review(toc);
  });
  on('access', (a) => {
    role = a.role || '';
    me = a.user || 'Guest';
  });
  on('wsclose', () => {
    pending = null;
  });
  // Manage users is a dialog outside the form, so its edits come as this bus event.
  on('saveSourceChanged', () => isOpen() && edited());
  onMsg('settingsSaveResult', settingsSaveResult);
  initInterruptions();
}
