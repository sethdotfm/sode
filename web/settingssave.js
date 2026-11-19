// The Settings save path: unsaved changes, the one Save (`settingsSave`) and its answer per field. The review is in
// savereview.js. A change is a difference from the form as opened or last saved. A save source (savesources.js:
// Manage users, the API section) gives its own changes. data-apart inputs are not counted.

import { $, el, S, on, onMsg, wsSend, sentence } from './core.js';
import { fmtTime } from './device.js';
import { layout } from './layout.js';
import { relock, unlocked, trusted, needsPin } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { askRecheck } from './recheck.js';
import { toGlobal } from './prefs.js';
import { reviewSave } from './savereview.js';
import { SAVE_FIELDS, diffRows, showValue, refusalNote, reconnects, critical, sectionOfKey } from './savereviewrules.js';
import { saveSources, sourceOf } from './savesources.js';
import { noteAuthors, rowId, splitSeen, sentKeys, unseenNotes, namesAuthors } from './stagedrules.js';
import { sectionView } from './settingsrolerules.js';
import { showPinFields, resetPinFields } from './settingspin.js';
import { form, isOpen, readSettings, formProblem } from './settingsform.js';
import { renderLock, showLockErr, sendUnlock } from './settingslock.js';

// A save is in flight: its own hello / locked arrive before the result, so do not flip to the PIN prompt for them.
export const saving = () => $('settings-save').disabled;

// The Advanced Settings PIN fields go on the `config` message, after the one Save. Their row sits in Locks.
const PIN = 'settingsPin';
const sectionOf = (key) => sectionOfKey(key) || 'locks';

let base = {}; // the fields as last opened or saved
let versions = {}; // each field's version as loaded
const refused = new Map(); // key -> refused result; marked until it saves or goes back
let savedAt = null;
let role = '';
let me = ''; // who stages an edit now: the user, a stepped-in user included, else Guest
let authors = new Map(); // stagedrules.js marks
let seq = 0;
let pending = null; // { id, sent, retried, pin }; sent: key -> the value sent

function pinRow() {
  if ($('cfg-pin').value) return { key: PIN, sub: null, was: S.pinSet ? 'Set' : 'Not set', now: 'Changed' };
  if ($('cfg-pin-clear').checked) return { key: PIN, sub: null, was: 'Set', now: 'Removed' };
  return null;
}

function changes() {
  const rows = diffRows(base, saveFields());
  for (const src of saveSources()) rows.push(...diffRows(src.base(), src.now()));
  const pin = pinRow();
  return pin ? [...rows, pin] : rows;
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
  [PIN]: 'cfg-pin-change',
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
  if (r.key === PIN) return 'Advanced Settings PIN';
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
  if (r.key === PIN) return v;
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

const kept = (r) => (sectionView(sectionOf(r.key), role, layout() === 'phone') === 'edit' ? '' : 'your Role cannot change this');

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
  return [...(names.length ? [`Unsaved changes in ${names.join(', ')}`] : []), ...unseenNotes(unseen, by)].join('. ');
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
  $('settings-unsaved').textContent = `${unsavedText()}. Save them, or exit again to discard them.`;
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

function sendFields(keys, sent, retried, pin) {
  const version = (k) => (sourceOf(k) ? sourceOf(k).version(k) : versions[k]) ?? '';
  const fields = Object.fromEntries(keys.map((k) => [k, { value: sent[k], version: version(k) }]));
  if (!keys.length) return void (pin && sendPin());
  const id = ++seq;
  if (!wsSend({ t: 'settingsSave', id, fields })) return void showSaveErr('Not connected to Sode.');
  pending = { id, sent, retried, pin };
  $('settings-save').disabled = true;
}

function sendPin() {
  if (!wsSend({ t: 'config', config: {}, pin: $('cfg-pin').value, clearPin: $('cfg-pin-clear').checked })) return void showSaveErr('Not connected to Sode.');
  $('settings-save').disabled = true;
}

function settingsSaveResult(m) {
  if (!pending || m.id !== pending.id) return;
  const p = pending;
  pending = null;
  $('settings-save').disabled = false;
  if (!m.results) {
    if (m.locked) relock();
    return showSaveErr(sentence(m.err) || 'Not saved.');
  }
  let saved = 0;
  const again = [];
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
      if (ok) sendFields(again, p.sent, true, p.pin);
      else done(saved, p);
    });
    return;
  }
  done(saved, p);
}

function done(saved, p) {
  if (saved) savedAt = new Date().toISOString();
  if (!isOpen()) return;
  paintMarks();
  if (refused.size) showSaveErr(`${refused.size} ${refused.size === 1 ? 'change is' : 'changes are'} not saved. Each one says why.`);
  else $('settings-err').hidden = true;
  if (p.pin) sendPin();
  renderSaveState();
}

// The Advanced Settings PIN change's answer.
function configResult(m) {
  $('settings-save').disabled = false;
  if (m.ok) {
    if (!isOpen()) return;
    // A changed PIN has relocked everyone, this view included.
    resetPinFields();
    savedAt = new Date().toISOString();
    renderSaveState();
    renderLock();
    return;
  }
  if (m.recheck) {
    askRecheck('Changing the Advanced Settings PIN').then((ok) => {
      if (ok) sendPin();
      else showSaveErr(sentence(m.err));
    });
    return;
  }
  if (m.locked) {
    relock();
    // Not trusted: Settings flips to its own PIN prompt. A trusted user's form never did, so
    // ask in the popup and save again.
    if (needsPin()) return showLockErr(m.err);
    reask('Saving that needs the Advanced Settings PIN.').then((ok) => {
      if (ok) sendPin();
    });
    return;
  }
  showSaveErr(sentence(m.err));
}

const askThenSave = (why) =>
  askPin(why).then((ok) => {
    if (ok) form.requestSubmit();
  });

async function review({ focusField, reveal }) {
  const now = readSettings();
  const bad = formProblem(now);
  if (bad) {
    showSaveErr(bad.text);
    focusField(bad.node);
    return;
  }
  const pin = $('cfg-pin').value;
  if (pin && [...pin].length < 4) {
    showSaveErr('The Advanced Settings PIN must be at least 4 characters.');
    showPinFields(true);
    focusField($('cfg-pin'));
    return;
  }
  if (pin !== $('cfg-pin2').value) {
    showSaveErr('The two PINs do not match.');
    showPinFields(true);
    reveal($('cfg-pin2'));
    $('cfg-pin2').select();
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
  const send = await reviewSave({
    rows,
    unseen: unseenNotes(unseen, by),
    by: namesAuthors(rows, by, me) ? by : null,
    kept,
    place,
    label,
    show,
    critical: (r) => critical(r.key),
    reconnects: (r) => reconnects(r.key),
    address: now.router,
    recheckNeeded: S.users.length > 0,
  });
  if (!send || !isOpen()) return;
  // Fields the page expects refused are sent too: the server's answer gives each its reason.
  // Edits the Role cannot see stay unsaved and stay staged; the rest are sent.
  const keys = sentKeys(all, seen).filter((k) => k !== PIN);
  sendFields(
    keys,
    valuesOf(keys),
    false,
    rows.some((r) => r.key === PIN && !kept(r)),
  );
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
    if ('locked' in form.dataset) return void sendUnlock();
    if (saving()) return;
    // A trusted user saves without the PIN, but not a change to the PIN itself: ask now instead
    // of after a refusal.
    if (S.pinSet && trusted() && !unlocked() && ($('cfg-pin').value || $('cfg-pin-clear').checked)) {
      return void askThenSave('Changing the PIN needs the Advanced Settings PIN.');
    }
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
  on('configResult', configResult);
}
