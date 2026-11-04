// Profile dialog (Beta 1.6, #161): one user's saved This device settings,
// filter sets and presets, each with a Remove, and one Export / import…
// button that hands over to the transfer dialog (bus 'openTransfer'). It is
// opened for one user by the bus event 'openProfile' {name}: the Profile…
// buttons of the users editor and of Settings > Users, and My profile… in the
// account block. There is no user list here. The own profile is read from
// profile.js's cached copy (kept current by its 'profile' event); anyone
// else's goes through fetchProfile (a trusted user or the settings PIN; the
// server decides). Saves go through saveProfile.
// Imported by settings.js.

import { el, S, on, emit, sentence } from './core.js';
import { demandPin, trusted, unlocked } from './pin.js';
import { trapDialog } from './widgets.js';
import { fetchProfile, saveProfile, profile, profileName } from './profile.js';
import { prefRows, withoutPref, setSummary, presetSummary, without, canSeeProfiles } from './profilesrules.js';

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
const body = el('div', 'sode__profiles-body');
const title = Object.assign(el('span', 'pb-modal__title', 'Profile'), { id: 'profiles-title' });
const transferBtn = btn('pb-btn--outline', 'Export / import…', () => transfer());
const closeBtn = btn('pb-btn--outline', 'Close', () => closeDialog());
transferBtn.disabled = true;

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__filters sode__profiles-dlg');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'profiles-title');
dialog.tabIndex = -1;
dialog.append(
  el('div', 'pb-modal__header', title),
  el('div', 'pb-modal__body',
    el('p', 'sode__hint', 'Admins and trusted users can see and edit every profile.'),
    body, err),
  el('div', 'pb-modal__footer', note, transferBtn, closeBtn));
// A later "Restrictions" section (issue #184: sources and destinations a
// trusted user hides from a user, enforced by the server) goes in `body`
// after the presets section. Nothing is shown for it yet.
root.append(dialog);
document.body.append(root);

let isOpen = false;
let back = null;
let who = ''; // the user shown
let prof = null; // their profile, as last read
let token = 0; // answers to an older read are ignored
let removing = null; // { kind, i } waiting for its confirm

const own = () => !!who && who === profileName();
const setNote = (t) => { note.textContent = t; };
function fail(text) {
  err.textContent = sentence(text);
  err.hidden = false;
}
// The server said the PIN is wanted: show the normal prompt in Settings.
const refused = () => { demandPin(); emit('locked'); };

// Export / import opens on top of this dialog, which stays (Esc closes the
// newer one first, then this).
function transfer() {
  if (!who) return;
  emit('openTransfer', { scope: 'user', user: who });
}

function section(heading, rows, empty) {
  return el('section', 'sode__profiles-sec',
    el('h3', 'sode__profiles-h', heading),
    rows.length ? el('ul', 'sode__profiles-list', ...rows) : el('p', 'sode__hint', empty));
}

// One line: text, a summary, and Remove (which turns into its confirm).
function line(kind, i, name, summary) {
  const text = el('span', 'sode__profiles-item', el('strong', null, name), el('span', 'sode__hint', summary));
  if (removing && removing.kind === kind && removing.i === i) {
    return el('li', 'sode__profiles-row', text,
      el('span', 'pb-note pb-note--warn', `Remove "${name}" from ${who}'s profile?`),
      btn('pb-btn--danger pb-btn--sm', 'Remove', () => doRemove(kind, i)),
      btn('pb-btn--outline pb-btn--sm', 'Keep', () => { removing = null; paint(); }));
  }
  const rm = btn('pb-btn--ghost pb-btn--sm', 'Remove', () => { removing = { kind, i }; paint(); });
  rm.setAttribute('aria-label', `Remove ${name}`);
  return el('li', 'sode__profiles-row', text, rm);
}

function paint() {
  title.textContent = who ? `Profile: ${who}` : 'Profile';
  transferBtn.disabled = !prof;
  if (!prof) {
    body.replaceChildren(el('p', 'sode__hint', who ? `Reading ${who}'s profile…` : 'No user chosen.'));
    return;
  }
  const prefs = prefRows(prof.prefs);
  body.replaceChildren(
    section('This device settings', prefs.map((p, i) => line('prefs', i, p.label, p.value)), 'None saved. This user follows the global defaults.'),
    section('Filter sets', prof.filterSets.map((s, i) => line('filterSets', i, String(s.name), setSummary(s))), 'None saved.'),
    section('Presets', prof.presets.map((p, i) => line('presets', i, String(p.name), presetSummary(p))), 'None saved.'));
}

async function load(name) {
  const mine = ++token;
  who = name;
  removing = null;
  err.hidden = true;
  setNote('');
  if (own()) { // the cached copy, no round trip
    prof = profile();
    return paint();
  }
  prof = null;
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
  prof = own() ? profile() : { ...prof, ...parts };
  setNote('Removed.');
  paint();
}

function openDialog(name) {
  if (!name) return;
  if (!isOpen) back = document.activeElement;
  isOpen = true;
  dlg.show();
  load(name);
  dialog.focus();
}

function closeDialog() {
  isOpen = false;
  token++;
  who = '';
  prof = null;
  removing = null;
  dlg.hide();
  if (back?.isConnected) back.focus();
}

on('openProfile', (m) => openDialog(String(m?.name || '')));
// A pending Remove goes first on Esc, then the dialog.
const dlg = trapDialog(root, dialog, () => { if (removing) { removing = null; paint(); } else closeDialog(); });
// The page locking takes the dialog with it.
on('locking', () => { if (isOpen) closeDialog(); });
// My own profile changes under the dialog (a save here, or elsewhere).
on('profile', () => { if (isOpen && own()) { prof = profile(); paint(); } });
// The user is gone, or the page is no longer allowed to see profiles.
// Someone else's profile also needs a trusted user or the settings PIN, still.
for (const ev of ['access', 'conn', 'unlockResult', 'locked', 'hello']) {
  on(ev, () => {
    if (isOpen && !own() && (!S.userNames?.includes(who) || !canSeeProfiles(S.userNames, trusted(), unlocked()))) closeDialog();
  });
}
