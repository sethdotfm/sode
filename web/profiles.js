// Profiles (Beta 1.6, #161): Settings > Users has a Profiles… button, next to
// Manage users…, for trusted users and the settings PIN. The dialog lists the
// users; picking one shows that user's saved This device settings, filter sets
// and presets, each with a Remove. Export… and Import… hand over to the
// transfer dialog (bus 'openTransfer'). Reads and saves go through profile.js
// (the one owner of the profile messages); the server decides who may.
// Imported by settings.js right after users.js, whose button it sits beside.

import { $, el, S, on, emit, sentence } from './core.js';
import { demandPin, trusted, unlocked } from './pin.js';
import { trapDialog } from './widgets.js';
import { fetchProfile, saveProfile } from './profile.js';
import { prefRows, withoutPref, setSummary, presetSummary, without, canSeeProfiles } from './profilesrules.js';

const manage = $('settings-view').querySelector('#users-manage');

// ---- the Settings button (built here: users.js builds its own section)

const open = el('button', 'pb-btn pb-btn--outline', 'Profiles…');
open.type = 'button';
open.id = 'profiles-open';
open.dataset.find = 'profiles-open';
open.dataset.label = 'Profiles';
open.setAttribute('aria-haspopup', 'dialog');
open.hidden = true;
manage?.after(open);

const names = () => S.userNames || [];
const offered = () => canSeeProfiles(names(), trusted(), unlocked());
const paintButton = () => { open.hidden = !offered(); };

// ---- the dialog

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

const err = el('div', 'pb-note pb-note--error');
err.setAttribute('role', 'alert');
err.hidden = true;
const note = el('span', 'sode__hint');
note.setAttribute('role', 'status');
note.setAttribute('aria-live', 'polite');
const list = el('div', 'sode__profiles-users');
list.setAttribute('role', 'group');
list.setAttribute('aria-label', 'Users');
const body = el('div', 'sode__profiles-body');
const exportBtn = btn('pb-btn--outline', 'Export…', () => transfer());
const importBtn = btn('pb-btn--outline', 'Import…', () => transfer());
const closeBtn = btn('pb-btn--outline', 'Close', () => closeDialog());
exportBtn.disabled = true;
importBtn.disabled = true;

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__filters sode__users-dlg');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'profiles-title');
dialog.tabIndex = -1;
dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Profiles'), { id: 'profiles-title' })),
  el('div', 'pb-modal__body',
    el('p', 'sode__hint', 'Admins and trusted users can see and edit every profile.'),
    list, body, err),
  el('div', 'pb-modal__footer', note, exportBtn, importBtn, closeBtn));
// A later "Restrictions" section (issue #184: sources and destinations a
// trusted user hides from a user, enforced by the server) goes in `body`
// after the presets section. Nothing is shown for it yet.
root.append(dialog);
document.body.append(root);

let isOpen = false;
let back = null;
let who = ''; // the user shown
let prof = null; // their profile, as last read
let token = 0; // answers to an older pick or read are ignored
let removing = null; // { kind, i } waiting for its confirm

const setNote = (t) => { note.textContent = t; };
function fail(text) {
  err.textContent = sentence(text);
  err.hidden = false;
}
// The server said the PIN is wanted: show the normal prompt in Settings.
const refused = () => { demandPin(); emit('locked'); };

function transfer() {
  if (!who) return;
  closeDialog();
  emit('openTransfer', { scope: 'user', user: who });
}

function section(title, rows, empty) {
  return el('section', 'sode__profiles-sec',
    el('h3', 'sode__profiles-h', title),
    rows.length ? el('ul', 'sode__profiles-list', ...rows) : el('p', 'sode__hint', empty));
}

// One line: text, a summary, and Remove (which turns into its confirm).
function line(kind, i, title, summary) {
  const text = el('span', 'sode__profiles-item', el('strong', null, title), el('span', 'sode__hint', summary));
  if (removing && removing.kind === kind && removing.i === i) {
    return el('li', 'sode__profiles-row', text,
      el('span', 'pb-note pb-note--warn', `Remove "${title}" from ${who}'s profile?`),
      btn('pb-btn--danger pb-btn--sm', 'Remove', () => doRemove(kind, i)),
      btn('pb-btn--outline pb-btn--sm', 'Keep', () => { removing = null; paint(); }));
  }
  const rm = btn('pb-btn--ghost pb-btn--sm', 'Remove', () => { removing = { kind, i }; paint(); });
  rm.setAttribute('aria-label', `Remove ${title}`);
  return el('li', 'sode__profiles-row', text, rm);
}

function paint() {
  list.replaceChildren(...names().map((n) => {
    const b = btn(n === who ? 'pb-btn--sm' : 'pb-btn--outline pb-btn--sm', n, () => pick(n));
    b.setAttribute('aria-pressed', String(n === who));
    return b;
  }));
  exportBtn.disabled = importBtn.disabled = !prof;
  if (!prof) {
    body.replaceChildren(el('p', 'sode__hint', who ? `Reading ${who}'s profile…` : 'Choose a user to see their profile.'));
    return;
  }
  const prefs = prefRows(prof.prefs);
  body.replaceChildren(
    section('This device settings', prefs.map((p, i) => line('prefs', i, p.label, p.value)), 'None saved. This user follows the global defaults.'),
    section('Filter sets', prof.filterSets.map((s, i) => line('filterSets', i, String(s.name), setSummary(s))), 'None saved.'),
    section('Presets', prof.presets.map((p, i) => line('presets', i, String(p.name), presetSummary(p))), 'None saved.'));
}

async function pick(name) {
  const mine = ++token;
  who = name;
  prof = null;
  removing = null;
  err.hidden = true;
  setNote('');
  paint();
  const r = await fetchProfile(name);
  if (mine !== token || !isOpen) return;
  if (!r.ok) {
    if (r.locked) refused();
    fail(r.err || 'Could not read that profile.');
    return paint();
  }
  prof = r.profile;
  paint();
}

async function doRemove(kind, i) {
  if (!prof) return;
  const mine = ++token;
  removing = null;
  err.hidden = true;
  setNote('');
  // Prefs are shown only when valid, so the index is into the shown rows.
  const parts = kind === 'prefs'
    ? { prefs: withoutPref(prof.prefs, prefRows(prof.prefs)[i].key) }
    : { [kind]: without(prof[kind], i) };
  const r = await saveProfile(parts, who);
  if (mine !== token || !isOpen) return;
  if (!r.ok) {
    if (r.locked) refused();
    fail(r.err || 'Could not save that profile.');
    return paint();
  }
  prof = { ...prof, ...parts };
  setNote('Removed.');
  paint();
}

function openDialog() {
  back = document.activeElement;
  isOpen = true;
  who = '';
  prof = null;
  removing = null;
  err.hidden = true;
  setNote('');
  paint();
  root.hidden = false;
  dialog.focus();
}

function closeDialog() {
  isOpen = false;
  token++;
  root.hidden = true;
  if (back?.isConnected) back.focus();
}

open.addEventListener('click', openDialog);
trapDialog(root, dialog, () => { if (removing) { removing = null; paint(); } else closeDialog(); });
// Esc closes the dialog (a pending Remove first), never Settings behind it.
document.addEventListener('keydown', (e) => {
  if (!isOpen || e.key !== 'Escape') return;
  e.preventDefault();
  e.stopImmediatePropagation();
  if (removing) { removing = null; paint(); } else closeDialog();
}, true);
// The page locking takes the dialog with it.
on('locking', () => { if (isOpen) closeDialog(); });

for (const ev of ['access', 'unlockResult', 'locked', 'hello', 'conn']) {
  on(ev, () => {
    paintButton();
    if (isOpen && !offered()) closeDialog();
    else if (isOpen) paint();
  });
}
paintButton();
