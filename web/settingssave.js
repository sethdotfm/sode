// The Settings save path: unsaved changes, the one Save (`settingsSave`) and its answer per field. The review is in
// savereview.js. A change is a difference from the form as opened or last saved. A save source (savesources.js:
// Manage users, the API section) gives its own changes. data-apart inputs are not counted.

import { $, el, S, on, onMsg, wsSend, sentence } from './core.js';
import { fmtTime } from './device.js';
import { layout } from './layout.js';
import { askRecheck } from './recheck.js';
import { askApproval } from './approval.js';
import { toGlobal } from './prefs.js';
import { reviewSave } from './savereview.js';
import { askInterruptions, initInterruptions } from './interruptions.js';
import { SAVE_FIELDS, diffRows, showValue, refusalNote, reconnects, critical, sectionOfKey } from './savereviewrules.js';
import { saveSources, sourceOf } from './savesources.js';
import { noteAuthors, rowId, splitSeen, sentKeys, unseenNotes, namesAuthors } from './stagedrules.js';
import { sectionView, approvalWho } from './settingsrolerules.js';
import { form, isOpen, readSettings, formProblem } from './settingsform.js';

// A save is in flight.
export const saving = () => $('settings-save').disabled;

const sectionOf = sectionOfKey;

let base = {}; // the fields as last opened or saved
let versions = {}; // each field's version as loaded
const refused = new Map(); // key -> refused result; marked until it saves or goes back
let savedAt = null;
let role = '';
let me = ''; // who stages an edit now: the user, a stepped-in user included, else Guest
let authors = new Map(); // stagedrules.js marks
let seq = 0;
let pending = null; // { id, sent, retried, answer }; sent: key -> the value sent; answer: the Approval popup's

function changes() {
  const rows = diffRows(base, saveFields());
  for (const src of saveSources()) rows.push(...diffRows(src.base(), src.now()));
  return rows;
}

// A data-apart input with a data-key saves on its own, so it is never a change here.
const saveFields = () => Object.fromEntries(Object.entries(readSettings()).filter(([k]) => k in SAVE_FIELDS));

export const dirty = () => isOpen() && changes().length > 0;

// Called on each edit, never on a Step in or Step out, so an edit keeps the name of who made it.
const noteEdits = () => {
  authors = noteAuthors(authors, changes(), me);
};
const by = (r) => authors.get(rowId(r))?.by || me;
// Edits in a section the saver's Role cannot see show only as a count, and Save skips them (spec #543, Step in).
const seen = (r) => sectionView(sectionOf(r.key), role) !== 'hidden';
const warned = () => !$('settings-unsaved').hidden;

// ---- what each change is called on the page

const SPECIAL = {
  levelColors: 'cfg-colors',
  levelNumbering: 'cfg-lvlnum',
  defaultLevels: 'cfg-default',
  browserDefaults: 'gcfg-theme',
  locks: 'cfg-lock-main',
  newUserDefaults: 'cfg-new-persistent',
};
const keyNode = (key) => (SPECIAL[key] ? $(SPECIAL[key]) : form.querySelector(`[data-key="${key}"]`));
const fieldset = (key) => form.querySelector(`:scope > fieldset[data-section="${sectionOf(key)}"]`);
const sections = () => [...form.querySelectorAll(':scope > fieldset[data-section]')];
const sectionName = (fs) => fs.querySelector(':scope > legend').firstChild.textContent.trim();
const checkText = (node) => node?.closest('.pb-check')?.querySelector('.pb-check__label')?.textContent;
const forText = (node) => node && form.querySelector(`label[for="${node.id}"]`)?.textContent;

function place(r) {
  const fs = fieldset(r.key);
  return { topic: $(`topic-${fs.dataset.topic}`)?.textContent.trim() || '', section: sectionName(fs), order: sections().indexOf(fs) };
}

function label(r) {
  const src = sourceOf(r.key);
  if (src) return src.label(r);
  if (r.key === 'locks') return checkText($(`cfg-lock-${r.sub}`)) || r.sub;
  if (r.key === 'newUserDefaults') return `New user defaults: ${checkText($(r.sub === 'persistent' ? 'cfg-new-persistent' : 'cfg-new-no-auto-lock')) || r.sub}`;
  if (r.key === 'browserDefaults') return forText(form.querySelector(`select[data-gpref="${r.sub}"]`)) || r.sub;
  if (r.key === 'levelColors') return $(`cfg-color-${r.sub + 1}`)?.getAttribute('aria-label') || `Level ${r.sub + 1} color`;
  if (r.key === 'levelNumbering') return $('cfg-lvlnum-label').textContent;
  const node = keyNode(r.key);
  return (forText(node) || checkText(node) || r.key).trim();
}

function show(r, v) {
  const src = sourceOf(r.key);
  if (src) return src.show(r, v);
  if (r.key === 'levelColors' && r.sub != null) return v || 'Default';
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    return Object.entries(v)
      .map(([sub, x]) => `${label({ key: r.key, sub })}: ${show({ key: r.key, sub }, x)}`)
      .join(', ');
  }
  if (r.key === 'levelNumbering') return form.querySelector(`label[for="cfg-lvlnum-${v}"]`)?.textContent || v;
  const sel = r.key === 'browserDefaults' ? form.querySelector(`select[data-gpref="${r.sub}"]`) : keyNode(r.key);
  if (sel?.tagName === 'SELECT') {
    const same = r.key === 'browserDefaults' ? (o) => toGlobal(r.sub, o.value) === v : (o) => o.value === String(v);
    const o = [...sel.options].find(same);
    if (o) return o.textContent;
  }
  return showValue(v);
}

// A read section (Shown) saves through an Approval, so its rows are sent.
const kept = (r) => (sectionView(sectionOf(r.key), role, layout() === 'phone') === 'hidden' ? 'your Role cannot change this' : '');

// ---- marks, the exit warning and the footer text

function paintMarks() {
  for (const m of form.querySelectorAll('.sode__refused')) m.remove();
  const open = new Set(changes().map((r) => r.key));
  for (const [key, r] of refused) {
    if (!open.has(key)) {
      refused.delete(key);
      continue;
    }
    const row = { key, sub: null };
    const note = refusalNote(r, (v) => show(row, v));
    const mark = el('div', 'pb-note pb-note--warn sode__refused', sourceOf(key) ? `${label(row)}: ${note}` : note);
    mark.dataset.refused = key;
    // A source's field has no input in the form: its mark goes at the end of its section.
    if (sourceOf(key)) {
      fieldset(key).append(mark);
      continue;
    }
    const node = keyNode(key);
    (node.closest('.sode__field') || node.closest('.pb-check') || node).after(mark);
  }
}

function unsavedText() {
  const { shown, unseen } = splitSeen(changes(), seen);
  const slugs = new Set(shown.map((r) => sectionOf(r.key)));
  const names = sections()
    .filter((fs) => slugs.has(fs.dataset.section))
    .map(sectionName);
  const what = [...(names.length ? [`Unsaved changes in ${names.join(', ')}`] : []), ...unseenNotes(unseen, by)].join('. ');
  // Save skips edits the Role cannot see, so with only those it is not offered.
  return `${what}. ${names.length ? 'Save them, or exit' : 'Exit'} again to discard them.`;
}

export function markClean(saved) {
  for (const src of saveSources()) src.reset();
  base = saveFields();
  versions = { ...S.versions };
  refused.clear();
  authors = new Map();
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
  $('settings-exit').classList.replace('pb-btn--outline', 'pb-btn--danger');
  renderSaveState();
  return true;
}

export function unwarn() {
  $('settings-unsaved').hidden = true;
  $('settings-exit').textContent = 'Exit';
  $('settings-exit').classList.replace('pb-btn--danger', 'pb-btn--outline');
  renderSaveState();
}

function renderSaveState() {
  const path = S.configPath ? el('code', null, S.configPath) : null;
  $('settings-path').replaceChildren(
    ...(dirty()
      ? warned()
        ? []
        : ['Unsaved changes']
      : !path
        ? [savedAt ? `Applied ${fmtTime(savedAt)}; not saved: ` : 'Not saved: ', el('code', null, 'SODE_CONFIG'), ' unset']
        : savedAt
          ? [`Saved ${fmtTime(savedAt)} to `, path]
          : ['Saved to ', path]),
  );
}

export function showSaveErr(text) {
  $('settings-err').textContent = text;
  $('settings-err').hidden = false;
}

// ---- sending and the answers

// The value of each key as it is now, from the form or its source.
function valuesOf(keys) {
  const form = readSettings();
  const out = {};
  for (const k of keys) {
    const src = sourceOf(k);
    out[k] = src ? src.now()[k] : form[k];
  }
  return out;
}

function fieldsOf(keys, sent) {
  const version = (k) => (sourceOf(k) ? sourceOf(k).version(k) : versions[k]) ?? '';
  return Object.fromEntries(keys.map((k) => [k, { value: sent[k], version: version(k) }]));
}

function sendFields(keys, sent, retried, approval = null, answer = null) {
  const fields = fieldsOf(keys, sent);
  if (!keys.length) return;
  const id = ++seq;
  if (!wsSend({ t: 'settingsSave', id, fields, ...(approval && { approval }) })) {
    if (!answer) showSaveErr('Not connected to Sode.');
    return false;
  }
  pending = { id, sent, retried, answer };
  $('settings-save').disabled = true;
  return true;
}

function settingsSaveResult(m) {
  if (!pending || m.id !== pending.id) return;
  const p = pending;
  pending = null;
  $('settings-save').disabled = false;
  if (p.answer && !m.results && m.approval) return void p.answer.fail(m.err);
  p.answer?.done();
  if (!m.results) return showSaveErr(sentence(m.err) || 'Not saved.');
  let saved = 0;
  const again = [];
  const approve = [];
  for (const [key, r] of Object.entries(m.results)) {
    const src = sourceOf(key);
    if (r.status === 'refused') {
      // Their value is now on the mark and in the review's Was, so the next Save overwrites it on purpose.
      if (r.reason === 'conflict' && r.version) {
        if (src) src.conflict(key, r);
        else {
          versions[key] = r.version;
          base[key] = r.value;
        }
      }
      if (r.reason === 'needs PIN recheck' && !p.retried) again.push(key);
      if (r.reason === 'needs Approval' && !p.answer) approve.push(key);
      refused.set(key, r);
      continue;
    }
    if (src) src.saved(key, p.sent[key], r);
    else {
      base[key] = p.sent[key];
      if (r.version) versions[key] = r.version;
    }
    refused.delete(key);
    if (r.status === 'saved') saved++;
  }
  if (again.length) {
    askRecheck('Saving these changes').then((ok) => {
      if (ok) sendFields(again, p.sent, true);
      else done(saved);
    });
    return;
  }
  done(saved);
  if (approve.length && isOpen()) {
    const who = approvalWho(approve.map(sectionOf));
    askApproval('Saving these changes', who, (ap, answer) => sendFields(approve, p.sent, true, ap, answer));
  }
}

function done(saved) {
  if (saved) savedAt = new Date().toISOString();
  if (!isOpen()) return;
  paintMarks();
  if (refused.size) showSaveErr(`${refused.size} ${refused.size === 1 ? 'change is' : 'changes are'} not saved. Each one says why.`);
  else $('settings-err').hidden = true;
  renderSaveState();
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
  if (!all.length) return void showSaveErr('There are no changes to save.');
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
    kept,
    place,
    label,
    show,
    // A row that goes through an Approval needs no recheck of this browser's user: an Administrator approver's counts.
    critical: (r) => critical(r.key) && sectionView(sectionOf(r.key), role) === 'edit',
    criticalAction: (r) => critical(r.key),
    role,
    reconnects: (r) => reconnects(r.key),
    address: now.router,
    recheckNeeded: S.users.length > 0,
  });
  if (!send || !isOpen()) return;
  sendFields(keys, valuesOf(keys), false);
}

// focusField and reveal come from the table of contents (settingstoc.js), which opens a field's page on a phone.
export function initSave(toc) {
  for (const ev of ['input', 'change']) {
    form.addEventListener(ev, () => {
      noteEdits();
      $('settings-err').hidden = true;
      if (refused.size) paintMarks();
      return warned() ? unwarn() : renderSaveState();
    });
  }
  window.addEventListener('beforeunload', (e) => {
    if (dirty()) e.preventDefault();
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
  on('saveSourceChanged', () => {
    if (!isOpen()) return;
    noteEdits();
    if (refused.size) paintMarks();
    return warned() ? unwarn() : renderSaveState();
  });
  onMsg('settingsSaveResult', settingsSaveResult);
  initInterruptions();
}
