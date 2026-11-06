// Profile dialog (Beta 1.6, #161): the editor of one user's profile. This
// device settings are a select each (Not set removes the key); filter sets
// and presets are lists with Edit, Add and Remove. Edit and Add open the
// Filters dialog / preset editor aimed at this profile (bus
// 'openFilterEditor' / 'openPresetEditor' {user, name?, profile}; they save
// through saveProfile, whose 'profileSaved' event refreshes this dialog). One Import & Export…
// button hands over to the transfer dialog (bus 'openTransfer'). It is
// opened for one user by the bus event 'openProfile' {name}: the Profile…
// buttons of the users editor and of Settings > Users, and My profile… in the
// account block. There is no user list here. The own profile is read from
// profile.js's cached copy (kept current by its 'profile' event); anyone
// else's goes through fetchProfile (a trusted user or the settings PIN; the
// server decides). Saves go through saveProfile.
// Imported by settings.js.

import { el, S, on, emit, sentence } from './core.js';
import { trusted, unlocked } from './pin.js';
import { reask } from './pinprompt.js';
import { trapDialog } from './widgets.js';
import { fetchProfile, saveProfile, profile, profileName } from './profile.js';
import { prefFields, prefsWith, withParts, setSummary, presetSummary, without, canSeeProfiles } from './profilesrules.js';

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
const transferBtn = btn('pb-btn--outline', 'Import & Export…', () => transfer());
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
let saving = 0; // this dialog's own saves in flight

const own = () => !!who && who === profileName();
const setNote = (t) => { note.textContent = t; };
function fail(text) {
  err.textContent = sentence(text);
  err.hidden = false;
}
// The server said the PIN is wanted (someone else's profile, trust revoked): ask in the popup.
const askAgain = () => reask('Another user\'s profile needs the Advanced Settings PIN.');

// Export / import opens on top of this dialog, which stays (Esc closes the
// newer one first, then this).
function transfer() {
  if (!who) return;
  emit('openTransfer', { scope: 'user', user: who });
}

// One line: text, a summary, and Remove (which turns into its confirm).
function line(kind, i, name, summary) {
  const editEv = kind === 'filterSets' ? 'openFilterEditor' : 'openPresetEditor';
  const text = el('span', 'sode__profiles-item', el('strong', null, name), el('span', 'sode__hint', summary));
  if (removing && removing.kind === kind && removing.i === i) {
    return el('li', 'sode__profiles-row', text,
      el('span', 'pb-note pb-note--warn', `Remove "${name}" from ${who}'s profile?`),
      btn('pb-btn--danger pb-btn--sm', 'Remove', () => doRemove(kind, i)),
      btn('pb-btn--outline pb-btn--sm', 'Keep', () => { removing = null; paint(); }));
  }
  const rm = btn('pb-btn--ghost pb-btn--sm', 'Remove', () => { removing = { kind, i }; paint(); });
  rm.setAttribute('aria-label', `Remove ${name}`);
  const ed = btn('pb-btn--outline pb-btn--sm', 'Edit', () => emit(editEv, { user: who, name, profile: prof }));
  ed.setAttribute('aria-label', `Edit ${name}`);
  return el('li', 'sode__profiles-row', text, ed, rm);
}

// One This device setting: a label and a select (Not set, then the values).
function prefLine(f) {
  const id = `profiles-pref-${f.key}`;
  const sel = el('select', 'pb-select');
  sel.id = id;
  for (const [v, t] of f.options) {
    const o = el('option', null, t);
    o.value = v;
    sel.append(o);
  }
  sel.value = f.value;
  sel.addEventListener('change', () => doSave({ prefs: prefsWith(prof.prefs, f.key, sel.value) }, 'Saved.'));
  const lab = el('label', 'sode__label', f.label);
  lab.htmlFor = id;
  return el('li', 'sode__profiles-row', el('span', 'sode__profiles-item', lab), el('span', 'pb-select-wrap', sel));
}

// A section with its list and an Add button.
function listSection(heading, rows, empty, addText, ev) {
  const add = btn('pb-btn--outline pb-btn--sm', addText, () => emit(ev, { user: who, profile: prof }));
  return el('section', 'sode__profiles-sec',
    el('h3', 'sode__profiles-h', heading),
    rows.length ? el('ul', 'sode__profiles-list', ...rows) : el('p', 'sode__hint', empty),
    el('div', null, add));
}

function paint() {
  title.textContent = who ? `Profile: ${who}` : 'Profile';
  transferBtn.disabled = !prof;
  if (!prof) {
    body.replaceChildren(el('p', 'sode__hint', who ? `Reading ${who}'s profile…` : 'No user chosen.'));
    return;
  }
  body.replaceChildren(
    el('section', 'sode__profiles-sec sode__profiles-sec--prefs',
      el('h3', 'sode__profiles-h', 'User defaults'),
      el('p', 'sode__hint', 'Not set follows the global default.'),
      el('ul', 'sode__profiles-list', ...prefFields(prof.prefs).map(prefLine))),
    listSection('Filter sets', prof.filterSets.map((s, i) => line('filterSets', i, String(s.name), setSummary(s))), 'None saved.', 'Add filter set', 'openFilterEditor'),
    listSection('Presets', prof.presets.map((p, i) => line('presets', i, String(p.name), presetSummary(p))), 'None saved.', 'Add preset', 'openPresetEditor'));
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
    if (r.locked && await askAgain()) return load(name); // the PIN was entered: read it again
    if (mine !== token || !isOpen) return;
    fail(r.err || 'Could not read that profile.');
    return paint();
  }
  prof = r.profile;
  paint();
}

// Saves some parts ({prefs}, {filterSets}, {presets}) and paints the result.
async function doSave(parts, done) {
  if (!prof) return;
  const mine = ++token;
  removing = null;
  err.hidden = true;
  setNote('');
  // Put the change in now: a second change before this answer is built on
  // it (only the latest answer is shown, so the first must not be lost).
  const name = who;
  prof = withParts(prof, parts);
  saving++;
  const r = await saveProfile(parts, name).finally(() => { saving--; });
  if (mine !== token || !isOpen) return;
  if (!r.ok) {
    if (r.locked && await askAgain()) return doSave(parts, done); // the PIN was entered: save again
    if (mine !== token || !isOpen) return;
    fail(r.err || 'Could not save that profile.');
    if (own()) { prof = profile(); return paint(); }
    paint();
    return load(name).then(() => fail(r.err || 'Could not save that profile.'));
  }
  prof = own() ? profile() : prof;
  setNote(done);
  paint();
}
const doRemove = (kind, i) => doSave({ [kind]: without(prof[kind], i) }, 'Removed.');

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
// An editor saved a filter set or preset into this profile.
// (Own profile: the cached copy; someone else's is read again. This
// dialog's own saves are skipped, it has the value already.)
on('profileSaved', (m) => {
  if (!isOpen || m?.name !== who || saving) return;
  err.hidden = true;
  if (own()) { prof = profile(); setNote('Saved.'); return paint(); }
  load(who).then(() => setNote('Saved.'));
});
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
