// Users (#151, #165): Settings shows a button; the
// directory (users and the longest PIN) is edited in its own dialog, which
// saves on its own (the `users` message, guarded by the settings PIN like the
// rest), never with the main Save. The locks and the idle time are Settings >
// Access fields. Each saved user has a Profile… button (bus 'openProfile'),
// in the editor and, for a trusted user without the settings PIN (who cannot
// open the editor), in a list in the Settings section. The server holds every
// rule and says what is wrong.
// Imported by settings.js before it builds its table of contents.

import { $, el, S, on, onMsg, emit, wsSend, sentence } from './core.js';
import { trusted, unlocked } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { trapDialog } from './widgets.js';
import { canSeeProfiles, profileButtonOff } from './profilesrules.js';

const form = $('settings-view');

// ---- the Settings section: the button

const manage = el('button', 'pb-btn pb-btn--outline', 'Manage users');
manage.type = 'button';
manage.id = 'users-manage';
manage.dataset.find = 'users-manage';
manage.dataset.label = 'Manage users';
manage.setAttribute('aria-haspopup', 'dialog');

// A trusted user without the settings PIN: each user's name and a Profile…
// button (the profile dialog reads and saves through the server, which decides).
const people = el('div', 'sode__field');
people.hidden = true;
people.setAttribute('role', 'group');
people.setAttribute('aria-label', 'Profiles');

const group = el('fieldset', 'sode__group',
  el('legend', null, 'Users'),
  el('div', 'sode__field', manage),
  people);
group.dataset.apart = ''; // its inputs save apart from the Settings form

// Next to Access, where the PIN is; at the end if that isn't found.
const access = [...form.querySelectorAll(':scope > fieldset')].find((f) => f.querySelector('legend')?.textContent === 'Access');
if (access) access.after(group);
else form.querySelector('#settings-err')?.parentElement?.before(group) ?? form.append(group);

// ---- the dialog

const check = (label) => {
  const input = el('input', 'pb-check__input');
  input.type = 'checkbox';
  return { input, label: el('label', 'pb-check', input, el('span', 'pb-check__label', label)) };
};
const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};
const field = (id, label, input, hint) => el('div', 'sode__field',
  Object.assign(el('label', null, label), { htmlFor: id }),
  el('span', 'pb-number-wrap', input),
  el('span', 'sode__hint', hint));
const number = (id) => {
  const i = el('input', 'pb-input');
  i.type = 'number';
  i.id = id;
  return i;
};

const rows = el('div', 'sode__users-list');
const maxInput = number('usr-maxpin');
maxInput.min = '4';
maxInput.max = '12';
const err = el('div', 'pb-note pb-note--error');
err.setAttribute('role', 'alert');
err.hidden = true;
const note = el('span', 'sode__hint');
note.setAttribute('role', 'status');
note.setAttribute('aria-live', 'polite');

const addBtn = btn('pb-btn--outline pb-btn--sm', 'Add user', () => { addRow().focus(); touched(); });
const saveBtn = btn('pb-btn--ok', 'Save', () => doSave());
const cancelBtn = btn('pb-btn--outline', 'Cancel', () => closeUsers());

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__filters sode__users-dlg');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'users-title');
dialog.tabIndex = -1;
dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Users'), { id: 'users-title' })),
  el('div', 'pb-modal__body',
    el('p', 'sode__hint', 'With users and a lock on (Settings > Access), a page asks for a PIN before it changes anything. PINs are digits and must differ. A trusted user can save Settings, filter sets and presets without the settings PIN.'),
    el('div', 'sode__users-table', el('div', 'sode__users-head', el('span', null, 'Name'), el('span', null, 'PIN')), rows),
    addBtn,
    el('div', 'sode__users-opts',
      field('usr-maxpin', 'Longest PIN (digits)', maxInput, '4-12.')),
    err),
  el('div', 'pb-modal__footer', note, cancelBtn, saveBtn));
root.append(dialog);
document.body.append(root);

// Enter in a field saves the users, not the Settings form.
dialog.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && e.target.tagName === 'INPUT') {
    e.preventDefault();
    saveBtn.click();
  }
});

let isOpen = false;
let edited = false; // changed in the dialog since it filled
let discard = false; // warned once; the next close discards
let back = null;
let seq = 0;
let outSeq = 0;
let listSeq = 0;
let listed = []; // the directory the server sent; empty until it does
let loaded = false;
const signing = new Map();

const setNote = (text, error = false) => {
  note.textContent = text;
  note.classList.toggle('sode__error-text', error);
};
function touched() {
  edited = true;
  discard = false;
  setNote('');
}
function fail(text) {
  err.textContent = sentence(text);
  err.hidden = false;
}

function addRow(u = { name: '', len: 0 }) {
  const name = el('input', 'pb-input');
  name.value = u.name;
  name.spellcheck = false;
  name.autocomplete = 'off';
  name.placeholder = 'Name';
  name.setAttribute('aria-label', 'User name');
  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.inputMode = 'numeric';
  pin.autocomplete = 'new-password';
  pin.setAttribute('aria-label', 'PIN');
  pin.placeholder = u.len ? 'unchanged' : 'new PIN';
  pin.maxLength = Number(maxInput.value) || S.cfg?.maxPinLen || 8;
  const trust = check('Trusted');
  trust.input.checked = !!u.trusted;
  trust.input.dataset.flag = 'trusted';
  const never = check('Never log out');
  never.input.checked = !!u.persistent;
  never.input.dataset.flag = 'persistent';
  const rm = btn('pb-btn--ghost pb-btn--sm', 'Remove', () => { row.remove(); touched(); });
  const out = u.len ? btn('pb-btn--outline pb-btn--sm', 'Sign out everywhere', () => {
    err.hidden = true;
    setNote('');
    const id = ++outSeq;
    if (!wsSend({ t: 'userSignOutAll', name: u.name, id })) return fail('Not connected to Sode.');
    out.disabled = true;
    signing.set(id, { out, name: u.name });
  }) : el('span'); // a new row has none; the span keeps the columns in line
  // Only a saved user has a profile: not a new row, not one being renamed.
  const prof = btn('pb-btn--outline pb-btn--sm', 'Edit Profile', () => emit('openProfile', { name: row.dataset.was }));
  prof.setAttribute('aria-haspopup', 'dialog');
  const paintProf = () => {
    prof.disabled = profileButtonOff(row.dataset.was, name.value);
    prof.title = prof.disabled ? 'Save the user first' : '';
  };
  const row = el('div', 'sode__users-row',
    el('div', 'sode__users-in', name),
    el('div', 'sode__users-in', pin),
    trust.label, never.label, prof, out, rm);
  row.dataset.was = u.len ? u.name : '';
  paintProf();
  name.addEventListener('input', paintProf);
  for (const f of [name, pin]) f.addEventListener('input', touched);
  for (const f of [trust.input, never.input]) f.addEventListener('change', touched);
  rows.append(row);
  return name;
}

onMsg('userSignOutAllResult', (m) => {
  const w = signing.get(m.id);
  signing.delete(m.id);
  if (w) w.out.disabled = false;
  if (m.ok) return setNote(`Signed ${w?.name || 'the user'} out everywhere.`);
  if (m.locked) { // ask, then send it again; the error shows only if the PIN is not given
    reask('Managing users needs the Advanced Settings PIN again.').then((ok) => { if (ok) w?.out.click(); else fail(m.err); });
    return;
  }
  fail(m.err);
});

function fill() {
  maxInput.value = String(S.cfg?.maxPinLen || 8);
  rows.replaceChildren();
  for (const u of listed) addRow(u);
  edited = false;
  discard = false;
  err.hidden = true;
  saveBtn.disabled = false;
}

// The Settings list of profiles, for a trusted user who can't open the editor.
function paintPeople() {
  const names = S.userNames || [];
  const show = trusted() && !unlocked() && canSeeProfiles(names, true, false);
  people.hidden = !show;
  if (!show) return people.replaceChildren();
  people.replaceChildren(...names.map((n) => {
    const b = btn('pb-btn--outline pb-btn--sm', 'Edit Profile', () => emit('openProfile', { name: n }));
    b.setAttribute('aria-haspopup', 'dialog');
    b.setAttribute('aria-label', `Profile of ${n}`);
    return el('div', 'sode__names', el('span', 'sode__label', n), b);
  }));
}

const paintSummary = paintPeople;

// The directory is not in the hello (a locked page gets that too): the
// settings PIN holder asks for it. Without the PIN the list stays empty.
const load = () => { wsSend({ t: 'usersList', id: ++listSeq }); };
onMsg('usersListResult', (m) => {
  if (m.id !== listSeq) return;
  if (!m.ok) { // a stale unlock: ask, then read the directory again
    if (m.locked && isOpen) reask('Managing users needs the Advanced Settings PIN.').then((ok) => (ok ? load() : closeUsers()));
    return;
  }
  listed = m.users || [];
  loaded = true;
  paintSummary();
  if (isOpen && !edited) fill();
});
on('unlockResult', (m) => { if (m.ok) load(); }); // the PIN just entered
on('locked', () => { listed = []; loaded = false; paintSummary(); });
on('access', paintSummary);
for (const ev of ['unlockResult', 'conn']) on(ev, paintPeople);
on('hello', () => { paintSummary(); load(); if (isOpen && !edited) fill(); });

// ---- open / close / save

async function openUsers() {
  // The list and the saving need the settings PIN, which a trusted user may not
  // have entered: a popup asks for it, instead of an editor with no users in it.
  const opener = document.activeElement;
  if (S.pinSet && !unlocked() && !(await askPin('Managing users needs the Advanced Settings PIN.'))) { opener?.focus?.(); return; }
  back = opener;
  isOpen = true;
  setNote('');
  fill();
  load();
  dlg.show();
  dialog.focus();
}

function closeUsers() {
  if (isOpen && edited && !discard) {
    discard = true;
    return setNote('Unsaved changes. Close again to discard them, or Save.', true);
  }
  isOpen = false;
  edited = false;
  dlg.hide();
  if (back?.isConnected) back.focus();
}
manage.addEventListener('click', openUsers);
// Esc goes to the top dialog of the stack (dialogkeys.js), so a Profile
// opened from here closes first.
const dlg = trapDialog(root, dialog, closeUsers);
// The page locking takes the dialog with it, edits and all.
on('locking', () => { if (isOpen) { edited = false; closeUsers(); } });

function doSave() {
  err.hidden = true;
  setNote('');
  const users = [...rows.children].map((row) => {
    const [name, pin] = row.querySelectorAll('input');
    const flag = (k) => row.querySelector(`input[data-flag="${k}"]`).checked;
    return { name: name.value.trim(), was: row.dataset.was, pin: pin.value, trusted: flag('trusted'), persistent: flag('persistent') };
  });
  const id = ++seq;
  if (!wsSend({ t: 'users', id, usersReq: { users, maxPinLen: Math.trunc(Number(maxInput.value)) || 0 } })) return fail('Not connected to Sode.');
  saveBtn.disabled = true;
}

onMsg('usersResult', async (m) => {
  saveBtn.disabled = false;
  if (m.id !== seq) return;
  if (m.ok) {
    edited = false;
    load(); // the hello before this was skipped as an edit
    if (isOpen) closeUsers();
    return;
  }
  if (m.locked && await reask('Managing users needs the Advanced Settings PIN again.')) return doSave(); // the PIN was entered: save again
  fail(m.err);
});

maxInput.addEventListener('input', () => {
  touched();
  for (const p of rows.querySelectorAll('input[type=password]')) p.maxLength = Number(maxInput.value) || 12;
});

paintSummary();
