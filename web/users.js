// Users: the directory (users, longest PIN) is edited in its own dialog that saves on its own (`users` message, settings-PIN guarded), never with the main Save.
// Each saved user has a Profile... button (bus 'openProfile'), also in a Settings list for a trusted user without the PIN. The server holds every rule.
// Imported by settings.js before it builds its table of contents.

import { $, el, S, on, onMsg, emit, wsSend, sentence } from './core.js';
import { trusted, unlocked } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { trapDialog } from './widgets.js';
import { canSeeProfiles, profileButtonOff } from './profilesrules.js';
import { MODES, RENAME_HINT, clashNotice, pinClashes, prefillUsernames, renameNotice, usernameColumn, usersReq } from './usersrules.js';

const form = $('settings-view');

// ---- the Settings section: the button
const manage = el('button', 'pb-btn pb-btn--outline', 'Manage users');
manage.type = 'button';
manage.id = 'users-manage';
manage.dataset.find = 'users-manage';
manage.dataset.label = 'Manage users';
manage.setAttribute('aria-haspopup', 'dialog');

const people = el('div', 'sode__field');
people.hidden = true;
people.setAttribute('role', 'group');
people.setAttribute('aria-label', 'Profiles');

const group = el('fieldset', 'sode__group', el('legend', null, 'Users'), el('div', 'sode__field', manage), people);
group.dataset.apart = '';

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
const field = (id, label, input, hint) =>
  el('div', 'sode__field', Object.assign(el('label', null, label), { htmlFor: id }), el('span', 'pb-number-wrap', input), el('span', 'sode__hint', hint));
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

const modeSelect = el('select', 'pb-select');
modeSelect.id = 'usr-mode';
for (const m of MODES) modeSelect.append(Object.assign(el('option', null, m.label), { value: m.value }));
const modeField = el(
  'div',
  'sode__field',
  Object.assign(el('label', null, 'Sign-in mode'), { htmlFor: 'usr-mode' }),
  el('span', 'pb-select-wrap', modeSelect),
  el(
    'span',
    'sode__hint',
    'Username and PIN: each user has a username, and two users may share a PIN. Switching back to PIN only needs a different PIN for each user who shares one.',
  ),
);
const fillBtn = btn('pb-btn--outline pb-btn--sm', 'Fill empty usernames', () => {
  const r = readRows();
  const names = prefillUsernames(r);
  for (const [i, row] of [...rows.children].entries()) row.querySelector('input[data-field="username"]').value = names[i];
  touched();
});
fillBtn.dataset.pbTip = 'Sets each empty username to the Name without spaces. You can edit them before saving.';
const usernameHead = el('span', null, 'Username');
const table = el('div', 'sode__users-table', el('div', 'sode__users-head', el('span', null, 'Name'), usernameHead, el('span', null, 'PIN')), rows);

const addBtn = btn('pb-btn--outline pb-btn--sm', 'Add user', () => {
  addRow().focus();
  touched();
});
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
  el(
    'div',
    'pb-modal__body',
    el(
      'p',
      'sode__hint',
      'With users and a lock on (Settings > Access), a page asks for a PIN before it changes anything. PINs are digits, and in PIN only mode they must differ. A trusted user can save Settings, filter sets and presets without the Advanced Settings PIN. ' +
        RENAME_HINT,
    ),
    table,
    el('div', 'sode__users-acts', addBtn, fillBtn),
    el('div', 'sode__users-opts', modeField, field('usr-maxpin', 'Longest PIN (digits)', maxInput, '4-12.')),
    err,
  ),
  el('div', 'pb-modal__footer', note, cancelBtn, saveBtn),
);
root.append(dialog);
document.body.append(root);

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
let listedMode = 'pin'; // the saved sign-in mode, what the server keeps when none is sent
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
  const r = readRows();
  const clashes = pinClashes(r, mode(), listedMode);
  for (const row of rows.children) {
    const clash = clashes.includes(row.querySelector('input[data-field="name"]').value.trim());
    row.toggleAttribute('data-clash', clash);
    row.querySelector('input[data-field="pin"]').classList.toggle('pb-input--invalid', clash);
  }
  setNote(clashNotice(clashes) || renameNotice(r));
}
function fail(text) {
  err.textContent = sentence(text);
  err.hidden = false;
}

const mode = () => (modeSelect.value === 'username' ? 'username' : 'pin');
function paintMode() {
  const named = usernameColumn(mode());
  table.classList.toggle('sode__users-table--named', named);
  usernameHead.hidden = !named;
  fillBtn.hidden = !named;
  for (const c of rows.querySelectorAll('.sode__users-name')) c.hidden = !named;
}

function addRow(u = { name: '', len: 0 }) {
  const name = el('input', 'pb-input');
  name.value = u.name;
  name.spellcheck = false;
  name.autocomplete = 'off';
  name.placeholder = 'Name';
  name.setAttribute('aria-label', 'Name');
  name.dataset.field = 'name';
  const uname = el('input', 'pb-input');
  uname.value = u.username || '';
  uname.spellcheck = false;
  uname.autocomplete = 'off';
  uname.autocapitalize = 'none';
  uname.maxLength = 24;
  uname.placeholder = 'Username';
  uname.setAttribute('aria-label', 'Username');
  uname.dataset.field = 'username';
  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.inputMode = 'numeric';
  pin.autocomplete = 'new-password';
  pin.setAttribute('aria-label', 'PIN');
  pin.dataset.field = 'pin';
  pin.placeholder = u.len ? 'unchanged' : 'new PIN';
  pin.maxLength = Number(maxInput.value) || S.cfg?.maxPinLen || 8;
  const trust = check('Trusted');
  trust.input.checked = !!u.trusted;
  trust.input.dataset.flag = 'trusted';
  const never = check('Never sign out');
  never.input.checked = !!u.persistent;
  never.input.dataset.flag = 'persistent';
  // A saved user takes two clicks and the Advanced Settings PIN; a row never saved has nothing to lose.
  const disarm = () => {
    rm.dataset.armed = '';
    rm.textContent = 'Remove';
    rm.classList.replace('pb-btn--danger', 'pb-btn--ghost');
  };
  const rm = btn('pb-btn--ghost pb-btn--sm', 'Remove', async () => {
    if (!row.dataset.was) {
      row.remove();
      touched();
      return;
    }
    if (!rm.dataset.armed) {
      rm.dataset.armed = '1';
      rm.textContent = 'Confirm remove';
      rm.classList.replace('pb-btn--ghost', 'pb-btn--danger');
      setNote(`Remove "${row.dataset.was}"? Once saved, this can't be undone: their PIN and profile are deleted.`);
      return;
    }
    if (S.pinSet && !(await askPin('Removing a user needs the Advanced Settings PIN.'))) return disarm();
    setNote('');
    row.remove();
    touched();
  });
  rm.addEventListener('blur', disarm);
  const out = u.len
    ? btn('pb-btn--outline pb-btn--sm', 'Sign out everywhere', () => {
        err.hidden = true;
        setNote('');
        const id = ++outSeq;
        if (!wsSend({ t: 'userSignOutAll', name: u.name, id })) return fail('Not connected to Sode.');
        out.disabled = true;
        signing.set(id, { out, name: u.name });
      })
    : el('span'); // a new row has none; the span keeps the columns in line
  const prof = btn('pb-btn--outline pb-btn--sm', 'Edit Profile', () => emit('openProfile', { name: row.dataset.was }));
  prof.setAttribute('aria-haspopup', 'dialog');
  const paintProf = () => {
    prof.disabled = profileButtonOff(row.dataset.was, name.value);
    prof.title = prof.disabled ? 'Save the user first' : '';
  };
  const row = el(
    'div',
    'sode__users-row',
    el('div', 'sode__users-in', name),
    Object.assign(el('div', 'sode__users-in sode__users-name', uname), { hidden: !usernameColumn(mode()) }),
    el('div', 'sode__users-in', pin),
    trust.label,
    never.label,
    prof,
    out,
    rm,
  );
  row.dataset.was = u.len ? u.name : '';
  if (u.shared) row.dataset.shared = '1';
  paintProf();
  name.addEventListener('input', paintProf);
  for (const f of [name, uname, pin]) f.addEventListener('input', touched);
  for (const f of [trust.input, never.input]) f.addEventListener('change', touched);
  rows.append(row);
  return name;
}

onMsg('userSignOutAllResult', (m) => {
  const w = signing.get(m.id);
  signing.delete(m.id);
  if (w) w.out.disabled = false;
  if (m.ok) return setNote(`Signed ${w?.name || 'the user'} out everywhere.`);
  if (m.locked) {
    // ask, then send it again; the error shows only if the PIN is not given
    reask('Managing users needs the Advanced Settings PIN again.').then((ok) => {
      if (ok) w?.out.click();
      else fail(m.err);
    });
    return;
  }
  fail(m.err);
});

function fill() {
  maxInput.value = String(S.cfg?.maxPinLen || 8);
  modeSelect.value = listedMode;
  rows.replaceChildren();
  paintMode();
  for (const u of listed) addRow(u);
  edited = false;
  discard = false;
  err.hidden = true;
  saveBtn.disabled = false;
}

function paintPeople() {
  const names = S.userNames || [];
  const show = trusted() && !unlocked() && canSeeProfiles(names, true, false);
  people.hidden = !show;
  if (!show) return people.replaceChildren();
  people.replaceChildren(
    ...names.map((n) => {
      const b = btn('pb-btn--outline pb-btn--sm', 'Edit Profile', () => emit('openProfile', { name: n }));
      b.setAttribute('aria-haspopup', 'dialog');
      b.setAttribute('aria-label', `Profile of ${n}`);
      return el('div', 'sode__names', el('span', 'sode__label', n), b);
    }),
  );
}

const paintSummary = paintPeople;

// The directory is not in the hello (a locked page gets that too): the settings PIN holder asks for it.
const load = () => {
  wsSend({ t: 'usersList', id: ++listSeq });
};
onMsg('usersListResult', (m) => {
  if (m.id !== listSeq) return;
  if (!m.ok) {
    // a stale unlock: ask, then read the directory again
    if (m.locked && isOpen) reask('Managing users needs the Advanced Settings PIN.').then((ok) => (ok ? load() : closeUsers()));
    return;
  }
  listed = m.users || [];
  listedMode = m.signInMode === 'username' ? 'username' : 'pin';
  loaded = true;
  paintSummary();
  if (isOpen && !edited) fill();
});
on('unlockResult', (m) => {
  if (m.ok) load();
});
on('locked', () => {
  listed = [];
  listedMode = 'pin';
  loaded = false;
  paintSummary();
});
on('access', paintSummary);
for (const ev of ['unlockResult', 'conn']) on(ev, paintPeople);
on('hello', () => {
  paintSummary();
  load();
  if (isOpen && !edited) fill();
});

// ---- open / close / save
async function openUsers() {
  // A trusted user may not have the PIN entered: a popup asks instead of an editor with no users.
  const opener = document.activeElement;
  if (S.pinSet && !unlocked() && !(await askPin('Managing users needs the Advanced Settings PIN.'))) {
    opener?.focus?.();
    return;
  }
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
const dlg = trapDialog(root, dialog, closeUsers);
on('locking', () => {
  if (isOpen) {
    edited = false;
    closeUsers();
  }
});

function readRows() {
  return [...rows.children].map((row) => {
    const val = (k) => row.querySelector(`input[data-field="${k}"]`).value;
    const flag = (k) => row.querySelector(`input[data-flag="${k}"]`).checked;
    return {
      name: val('name'),
      was: row.dataset.was,
      shared: !!row.dataset.shared,
      pin: val('pin'),
      trusted: flag('trusted'),
      persistent: flag('persistent'),
      username: val('username'),
    };
  });
}

function doSave() {
  err.hidden = true;
  setNote('');
  const users = readRows();
  const clashes = pinClashes(users, mode(), listedMode);
  if (clashes.length) {
    setNote(clashNotice(clashes), true);
    return rows.querySelector('.sode__users-row[data-clash] input[data-field="pin"]')?.focus();
  }
  const id = ++seq;
  if (!wsSend({ t: 'users', id, usersReq: usersReq(users, Math.trunc(Number(maxInput.value)) || 0, mode(), listedMode) }))
    return fail('Not connected to Sode.');
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
  if (m.locked && (await reask('Managing users needs the Advanced Settings PIN again.'))) return doSave(); // the PIN was entered: save again
  fail(m.err);
});

modeSelect.addEventListener('change', () => {
  touched();
  paintMode();
});
maxInput.addEventListener('input', () => {
  touched();
  for (const p of rows.querySelectorAll('input[type=password]')) p.maxLength = Number(maxInput.value) || 12;
});

paintSummary();
