// Settings > Export & import dialog, and share links. Export writes YAML (the server only converts: POST api/yaml); import reads a file or
// pasted text into the review sheet (reviewsheet.js), which a share link also opens. Names ride beside numbers so another router's numbers can still be matched.
// Pure rules are in exportfmt.js. Other modules open this with emit('openTransfer', { scope, user }).

import { $, el, S, on, sentence } from './core.js';
import { trapDialog, labeled } from './widgets.js';
import { showResult } from './plans.js';
import { PREFS } from './prefs.js';
import { ownPref } from './device.js';
import { profile, profileName, fetchProfile } from './profile.js';
import { localSets, globalSets, userSets, normFilter, snapshot, restore, adoptFilter } from './filterstore.js';
import { localPresets, globalPresets, userPresets, normPreset } from './presetstore.js';
import { openReview } from './reviewsheet.js';
import * as fmt from './exportfmt.js';

const site = () => S.cfg?.brandText || location.hostname;
const srcName = (n) => S.srcBy.get(n)?.name || '';
const dstName = (n) => S.dstBy.get(n)?.name || '';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};
const dot = (kind) => {
  const d = el('span', `pb-dot pb-dot--${kind}`);
  d.setAttribute('aria-hidden', 'true');
  return d;
};

// ---- share links

export function shareRow() {
  const input = el('input', 'pb-input sode__code');
  input.readOnly = true;
  input.setAttribute('aria-label', 'Share link');
  const status = el('span', 'pb-status');
  const ui = { input, status, copy: btn('pb-btn--outline', 'Copy link', () => copyLink(ui)) };
  ui.box = el('div', 'sode__filters-share', labeled('Share link', input), ui.copy, status);
  ui.box.hidden = true;
  return ui;
}

export function shareLink(doc, ui) {
  ui.box.hidden = false;
  ui.input.value = fmt.shareUrl(`${location.origin}${location.pathname}`, doc);
  ui.input.select();
  ui.status.replaceChildren();
  copiedOff(ui);
}

// With no clipboard the link stays selected, to copy by hand.
const COPIED_MS = 2000;
function copiedOff(ui) {
  clearTimeout(ui.timer);
  if (!ui.copy?.classList.contains('pb-btn--ok')) return;
  ui.copy.classList.replace('pb-btn--ok', 'pb-btn--outline');
  ui.copy.textContent = 'Copy link';
  ui.copy.style.minWidth = '';
}
export async function copyLink(ui) {
  let ok = false;
  try {
    await navigator.clipboard.writeText(ui.input.value);
    ok = true;
  } catch {
    /* no clipboard */
  }
  if (!ok) {
    ui.input.select();
    copiedOff(ui);
    return ui.status.replaceChildren(dot('info'), 'Copy it above');
  }
  ui.status.replaceChildren();
  if (!ui.copy) return;
  clearTimeout(ui.timer);
  ui.copy.classList.replace('pb-btn--outline', 'pb-btn--ok');
  ui.copy.style.minWidth = `${ui.copy.offsetWidth}px`; // keeps Copy link's width
  ui.copy.textContent = 'Copied!';
  ui.timer = setTimeout(() => copiedOff(ui), COPIED_MS);
}

export const sharePresetDoc = (p) => fmt.presetShare(p, srcName, dstName);

// onLocked: a lock closed the sheet; the link waits to be shown again after unlocking.
function reviewShare(share, source = '', onLocked = null) {
  openReview({
    doc: fmt.docOfShare(share),
    source: source || (share.kind === 'preset' ? 'A shared preset' : 'A shared filter'),
    title: share.kind === 'preset' ? 'Review shared preset' : 'Review shared filter',
    applyNow: share.kind === 'filter' ? applyNow : null,
    onLocked,
  });
}

function applyNow(f, name) {
  const prev = snapshot();
  adoptFilter(normFilter(f));
  const undo = btn('pb-btn--outline pb-btn--sm', 'Undo', () => {
    restore(prev);
    showResult('ok', 'Filters restored', '');
  });
  showResult('info', name ? `Shared filter “${name}” applied` : 'Shared filter applied', () => el('span', 'sode__filters-empty', undo));
}

// A link in the address (#share=, or the old ?f=) stays PENDING until the sheet can show: names are in and no lock covers the page
// (a new browser with the main lock on gets `hello` before the lock screen, so the sheet would open and be closed again).
// The address is cleaned only once the sheet is up, so a reload while locked keeps the link. A hashchange link goes the same way.
let pending = null; // the share document waiting to be shown
let access = null; // lock.js's lock state, once it has said
let namesIn = false;

let lockUp = false; // lock.js's overlay is up (its 'lockcover' event)
const pageLocked = () => lockUp;
const clean = () => {
  const r = fmt.readLink(location.search, location.hash);
  if (r.doc || r.error) history.replaceState(null, '', location.pathname + r.search + r.hash);
};

function tryOpen() {
  if (!pending || !fmt.mayOpenSheet(access, pageLocked(), namesIn)) return;
  const doc = pending;
  pending = null;
  clean();
  reviewShare(doc, '', () => {
    // A lock closed it: wait for the page to be uncovered, and keep the link in the address.
    pending = doc;
    history.replaceState(null, '', `${location.pathname}${location.search}#share=${fmt.encodeShare(doc)}`);
  });
}

function takeLink() {
  const r = fmt.readLink(location.search, location.hash);
  if (!r.doc && !r.error) return;
  if (r.error) {
    history.replaceState(null, '', location.pathname + r.search + r.hash);
    return showResult('warn', 'Invalid share link', r.error);
  }
  pending = r.doc;
  tryOpen();
}

export function applySharedLink() {
  on('helloData', () => {
    namesIn = true;
    tryOpen();
  });
  on('access', (l) => {
    access = l;
    tryOpen();
  });
  on('lockcover', ({ on: up }) => {
    lockUp = up;
    tryOpen();
  });
  window.addEventListener('hashchange', takeLink);
  takeLink();
}

// ---- the dialog

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__filters sode__tr');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'tr-title');
dialog.tabIndex = -1;

const exBox = el('div', 'sode__filters-sets');
const imBox = el('div', 'sode__filters-sets');
const noteEl = el('span', 'sode__hint');
noteEl.setAttribute('aria-live', 'polite');
dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Export & import'), { id: 'tr-title' })),
  el('div', 'pb-modal__body sode__filters-body', exBox, imBox, noteEl),
  el(
    'div',
    'pb-modal__footer',
    btn('pb-btn--ok', 'Close', () => close()),
  ),
);
root.append(dialog);
document.body.append(root);

const setNote = (t, error = false) => {
  noteEl.textContent = t;
  noteEl.classList.toggle('sode__error-text', error);
};

let open = false,
  back = null;
// user: a profile named by openTransfer (someone else's, read by name; the logged-in user's own is just 'user').
let ex = null;
let other = null; // { name, state: 'wait' | 'ok' | 'err', profile, err }

const newEx = () => ({ src: 'local', prefs: true, setsMode: 'all', setsPick: new Set(), presetsMode: 'all', presetsPick: new Set() });

function dataOf(src) {
  if (src === 'global') return { prefs: {}, sets: globalSets(), presets: globalPresets(), user: '' };
  if (src === 'other') {
    const p = other?.profile;
    return {
      prefs: { ...p?.prefs },
      sets: (p?.filterSets || []).filter((s) => s?.name).map((s) => ({ name: String(s.name), f: normFilter(s) })),
      presets: (p?.presets || []).filter((s) => s?.name).map((s) => normPreset(s)),
      user: other?.name || '',
    };
  }
  if (src === 'user') return { prefs: { ...profile().prefs }, sets: userSets(), presets: userPresets(), user: profileName() };
  const own = {};
  for (const n of Object.keys(PREFS)) if (ownPref(n)) own[n] = ownPref(n);
  return { prefs: own, sets: localSets(), presets: localPresets(), user: '' };
}
const validPrefs = (m) => Object.fromEntries(Object.entries(m).filter(([n, v]) => Object.hasOwn(PREFS, n) && Object.hasOwn(PREFS[n].values, v)));

function sourceOptions() {
  const o = [];
  if (other) o.push(['other', `${other.name}’s profile`]);
  o.push(['local', 'This browser']);
  if (profileName()) o.push(['user', 'Me']);
  o.push(['global', 'Everyone']);
  return o;
}

function pickList(kind, items) {
  const mode = kind === 'sets' ? ex.setsMode : ex.presetsMode;
  const picked = kind === 'sets' ? ex.setsPick : ex.presetsPick;
  const label = kind === 'sets' ? 'Filter sets' : 'Presets';
  const sel = el('select', 'pb-select');
  sel.setAttribute('aria-label', label);
  sel.append(new Option(`All ${label.toLowerCase()} (${items.length})`, 'all'), new Option(`Chosen ${label.toLowerCase()}`, 'chosen'));
  sel.disabled = !items.length;
  sel.value = items.length ? mode : 'all';
  sel.addEventListener('change', () => {
    if (kind === 'sets') ex.setsMode = sel.value;
    else ex.presetsMode = sel.value;
    renderExport();
  });
  const out = [el('div', 'sode__filters-set', el('span', 'sode__label', label), el('span', 'pb-select-wrap', sel))];
  if (!items.length) out.push(el('span', 'sode__hint', `No ${label.toLowerCase()} here.`));
  else if (mode === 'chosen') {
    for (const it of items) {
      const c = el('input', 'pb-check__input');
      c.type = 'checkbox';
      c.checked = picked.has(it.name);
      c.addEventListener('change', () => {
        if (c.checked) picked.add(it.name);
        else picked.delete(it.name);
      });
      out.push(el('label', 'pb-check sode__filters-indent', c, el('span', 'pb-check__label', it.name)));
    }
  }
  return out;
}

function renderExport() {
  if (!ex) return;
  const opts = sourceOptions();
  if (!opts.some((o) => o[0] === ex.src)) ex.src = opts[0][0];
  const d = dataOf(ex.src);
  const src = el('select', 'pb-select');
  src.setAttribute('aria-label', 'Export from');
  for (const [v, t] of opts) src.append(new Option(t, v));
  src.value = ex.src;
  src.addEventListener('change', () => {
    ex.src = src.value;
    setNote('');
    renderExport();
  });
  const prefs = validPrefs(d.prefs);
  const pc = el('input', 'pb-check__input');
  pc.type = 'checkbox';
  pc.checked = ex.prefs && Object.keys(prefs).length > 0;
  pc.disabled = !Object.keys(prefs).length;
  pc.addEventListener('change', () => {
    ex.prefs = pc.checked;
  });
  const prefNote =
    ex.src === 'global' ? 'Everyone’s defaults are set in Settings.' : !Object.keys(prefs).length ? 'None set here.' : `${Object.keys(prefs).length} set`;
  const name = el('input', 'pb-input');
  name.setAttribute('aria-label', 'File name');
  name.autocomplete = 'off';
  name.value = ex.file || '';
  name.placeholder = fmt.defaultFileName({ site: site(), user: d.user });
  name.addEventListener('input', () => {
    ex.file = name.value;
  });
  name.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      download();
    }
  });
  const wait = ex.src === 'other' && other?.state !== 'ok';
  const go = btn('pb-btn--outline', 'Download', () => download());
  go.disabled = wait;
  exBox.replaceChildren(
    el('strong', null, 'Export'),
    el('div', 'sode__filters-set', el('span', 'sode__label', 'From'), el('span', 'pb-select-wrap', src)),
    ...(wait
      ? [el('span', 'sode__hint sode__error-text', other?.state === 'err' ? other.err : 'Reading their profile…')]
      : [
          el('label', 'pb-check', pc, el('span', 'pb-check__label', 'This browser settings'), el('span', 'sode__hint', ` ${prefNote}`)),
          ...pickList('sets', d.sets),
          ...pickList('presets', d.presets),
        ]),
    el('div', 'sode__filters-set', name, go),
    el(
      'span',
      'sode__hint',
      'Blank: the file is named with the date, this site and the profile. Names are saved beside the numbers so another router can match them.',
    ),
  );
}

function chosen(items, mode, picked) {
  return mode === 'chosen' ? items.filter((x) => picked.has(x.name)) : items;
}

async function download() {
  const d = dataOf(ex.src);
  const sets = chosen(d.sets, ex.setsMode, ex.setsPick);
  const presets = chosen(d.presets, ex.presetsMode, ex.presetsPick);
  const prefs = ex.prefs ? validPrefs(d.prefs) : {};
  if (!sets.length && !presets.length && !Object.keys(prefs).length) return setNote('Nothing chosen to export.', true);
  const now = new Date();
  const doc = fmt.buildExport({ prefs, filterSets: sets, presets, site: site(), user: d.user, now, srcName, dstName });
  const file = fmt.fileNameFor(ex.file, fmt.defaultFileName({ now, site: site(), user: d.user }));
  setNote('Preparing the file…');
  const r = await convert('api/yaml', JSON.stringify(doc), 'application/json');
  if (!r.ok) return setNote(r.err, true);
  const a = el('a');
  a.href = URL.createObjectURL(new Blob([r.text], { type: 'application/yaml' }));
  a.download = file;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 10000);
  setNote(`Saved ${file}.`);
}

async function convert(url, body, type) {
  try {
    const r = await fetch(url, { method: 'POST', headers: { 'Content-Type': type }, body });
    const text = await r.text();
    if (!r.ok) return { ok: false, text: '', err: sentence(text) || 'The file could not be converted.' };
    return { ok: true, text, err: '' };
  } catch {
    return { ok: false, text: '', err: 'Could not reach Sode.' };
  }
}

// ---- import

const paste = el('textarea', 'pb-input sode__code');
paste.rows = 5;
paste.spellcheck = false;
paste.setAttribute('aria-label', 'Pasted export text');
paste.placeholder = 'Or paste the YAML here';
const picker = el('input');
picker.type = 'file';
picker.accept = '.yaml,.yml,.txt,text/yaml,text/plain';
picker.setAttribute('aria-label', 'Export file');
picker.className = 'pb-input';

function renderImport() {
  imBox.replaceChildren(
    el('strong', null, 'Import'),
    el('div', 'sode__filters-set', picker),
    paste,
    el(
      'div',
      'sode__filters-set',
      btn('pb-btn--outline', 'Review import', () => review()),
    ),
    el('span', 'sode__hint', 'Nothing is saved until you have looked at it in the review.'),
  );
}

async function review() {
  let text = paste.value;
  const f = picker.files?.[0];
  if (f) {
    if (f.size > 1 << 20) return setNote('That file is over 1 MiB.', true);
    try {
      text = await f.text();
    } catch {
      return setNote('Could not read that file.', true);
    }
  }
  if (!text.trim()) return setNote('Choose a file or paste the export first.', true);
  setNote('Reading…');
  const r = await convert('api/yaml?from=yaml', text, 'text/plain');
  if (!r.ok) return setNote(r.err, true);
  let doc;
  try {
    doc = JSON.parse(r.text);
  } catch {
    return setNote('The file could not be read.', true);
  }
  const bad = fmt.checkDoc(doc);
  if (bad) return setNote(bad, true);
  const user = other && other.name !== profileName() ? other.name : '';
  close();
  openReview({ doc, scope: user || ex?.src === 'user' ? 'user' : 'local', user });
}

// ---- open / close

function openTransfer({ scope = '', user = '', what = '' } = {}) {
  back = document.activeElement;
  ex = newEx();
  // what: the kind the caller is about is the one exported; the other kinds start as none chosen.
  if (what === 'presets') {
    ex.setsMode = 'chosen';
    ex.prefs = false;
  }
  if (what === 'filters') {
    ex.presetsMode = 'chosen';
    ex.prefs = false;
  }
  other = null;
  picker.value = '';
  paste.value = '';
  setNote('');
  open = true;
  if (scope === 'user' && user) {
    if (user === profileName()) ex.src = 'user';
    else {
      other = { name: user, state: 'wait' };
      ex.src = 'other';
      const mine = other;
      fetchProfile(user).then((r) => {
        if (other !== mine) return;
        other = r.ok
          ? { name: user, state: 'ok', profile: r.profile }
          : { name: user, state: 'err', err: r.locked ? 'Only a trusted user or the Advanced Settings PIN can read another user’s profile.' : sentence(r.err) };
        if (open) renderExport();
      });
    }
  }
  renderExport();
  renderImport();
  dlg.show();
  dialog.focus();
}

function close() {
  if (!open) return;
  open = false;
  dlg.hide();
  if (back?.isConnected) back.focus();
}

const dlg = trapDialog(root, dialog, close);
on('openTransfer', openTransfer);
on('locking', close);
on('profile', () => {
  if (open) renderExport();
});
on('presets', () => {
  if (open) renderExport();
});
on('filters', () => {
  if (open) renderExport();
});

const openBtn = $('transfer-open');
if (openBtn) openBtn.addEventListener('click', () => openTransfer());
