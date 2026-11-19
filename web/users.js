// Users: the directory (users, longest PIN, Roles) has its own dialog with its own save (`users` message, Administrator
// only). It never saves with the main Save.
// Each saved user has a Profile... button (bus 'openProfile'), also in a Settings list for a Trusted user. The server holds every rule.

import { $, el, S, on, onMsg, emit, wsSend, sentence } from './core.js';
import { trusted, unlocked } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { askRecheck } from './recheck.js';
import { trapDialog } from './widgets.js';
import { canSeeProfiles, profileButtonOff } from './profilesrules.js';
import { accessReq, mergeAccess, rightsTab } from './rightsrules.js';
import { makeTabs } from './tabs.js';
import { btn, check, field as formField } from './formbits.js';
import { makeRightsTab } from './usersrights.js';
import { makeClientsTab } from './usersclients.js';
import { ROLES, ownerChoices, pinPlaceholder, usersReq } from './usersrules.js';

// ---- the Settings section (index.html #users-group)
const manage = $('users-manage');
const people = $('users-people');

// ---- the dialog
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

const roleSelect = (roles, value, label) => {
  const s = el('select', 'pb-select');
  for (const r of roles) s.append(Object.assign(el('option', null, r.label), { value: r.value }));
  s.value = value;
  s.setAttribute('aria-label', label);
  return s;
};
const table = el('div', 'sode__users-table', el('div', 'sode__users-head', el('span', null, 'Name'), el('span', null, 'PIN')), rows);
// A removed user's panels need a new owner in the same save (the server refuses it otherwise).
const owners = el('div', 'sode__users-opts');
owners.hidden = true;

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
let isOpen = false;
let ridSeq = 0;
const usersPanel = el(
  'div',
  null,
  el(
    'p',
    'sode__hint',
    'With users and a lock on (Settings > Access), a page asks for a PIN before it changes anything. Each user signs in with their name and PIN, PINs are digits, and two users can have the same PIN. A User without a PIN signs in by name only and opens no page lock. Such a user acts no wider than a Guest. A Trusted user or an Administrator always needs a PIN. A Standard user patches within their Rights and keeps their own Profile and panels. A Trusted user also changes shared Settings, filter sets, presets, every Profile and panel, the Icon library and Default artwork. An Administrator changes everything, and Sode always keeps one. A rename keeps the user signed in, with their keys, panels and Profile.',
  ),
  table,
  el('div', 'sode__users-acts', addBtn),
  owners,
  el('div', 'sode__users-opts', formField('usr-maxpin', 'Longest PIN (digits)', el('span', 'pb-number-wrap', maxInput), '4-12.')),
);
// The rights editor and the API clients tab read the live rows, and hand back their drafts on save.
const userList = () =>
  [...rows.children].map((r) => ({
    rid: Number(r.dataset.rid),
    id: r.dataset.uid,
    name: r.querySelector('input[data-field="name"]').value,
    was: r.dataset.was,
  }));
const rightsPanel = makeRightsTab({ users: userList, clients: () => clientsTab.read(), touched: () => touched() });
const clientsTab = makeClientsTab({ users: userList, touched: () => touched(), adopt: (rid, c) => c.name && rightsPanel.adoptClient(rid, c) });
const tabs = makeTabs(
  [
    { id: 'users', text: 'Users', panel: usersPanel },
    { id: 'clients', text: 'API clients', panel: clientsTab.node },
    { id: 'rights', text: 'Rights', panel: rightsPanel.node },
  ],
  (id) => {
    clientsTab.shownNow(id === 'clients' && isOpen);
    if (id === 'rights') rightsPanel.refresh();
  },
);
dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Manage users'), { id: 'users-title' })),
  el('div', 'pb-modal__body', tabs.node, usersPanel, clientsTab.node, rightsPanel.node, err),
  el('div', 'pb-modal__footer', note, cancelBtn, saveBtn),
);
root.append(dialog);
document.body.append(root);

// Enter saves from the Users panel only; the other tabs have their own forms.
usersPanel.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && e.target.tagName === 'INPUT') {
    e.preventDefault();
    saveBtn.click();
  }
});

let edited = false; // changed in the dialog since it filled.
let discard = false; // warned one time; the next close discards.
let back = null;
let seq = 0;
let outSeq = 0;
let listSeq = 0;
let listed = []; // the directory the server sent; empty until it does.
let listedMe = ''; // the id of the user asking, the default new owner
const ownerPick = new Map(); // removed user id to the chosen new owner id
let extras = { apiClients: [], guestRights: {} }; // clients and Guest, sent with it.
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
  for (const row of rows.children)
    paintPin(
      row,
      r.find((x) => x.rid === Number(row.dataset.rid)),
    );
  paintOwners(r);
}

function paintPin(row, r) {
  if (r) row.querySelector('input[data-field="pin"]').placeholder = pinPlaceholder(r);
}

function paintOwners(r = readRows()) {
  const asks = ownerChoices(listed, r, listedMe);
  owners.hidden = !asks.length;
  owners.replaceChildren(
    ...asks.map((a) => {
      const s = el('select', 'pb-select');
      s.id = `usr-owner-${a.id}`;
      s.dataset.owner = a.id;
      for (const k of a.kept) s.append(Object.assign(el('option', null, k.name), { value: k.id }));
      s.value = a.kept.some((k) => k.id === ownerPick.get(a.id)) ? ownerPick.get(a.id) : a.pick;
      s.addEventListener('change', () => ownerPick.set(a.id, s.value));
      const what = a.panels === 1 ? '1 panel' : `${a.panels} panels`;
      return formField(
        s.id,
        `New owner of ${a.name}'s ${what}`,
        el('span', 'pb-select-wrap', s),
        'You remove this user. Their panels go to this user when you save.',
      );
    }),
  );
}
const panelOwners = () => Object.fromEntries([...owners.querySelectorAll('select[data-owner]')].map((s) => [s.dataset.owner, s.value]));
function fail(text) {
  err.textContent = sentence(text);
  err.hidden = false;
}

function addRow(u = { name: '', len: 0 }) {
  const rid = ++ridSeq;
  const name = el('input', 'pb-input');
  name.value = u.name;
  name.spellcheck = false;
  name.autocomplete = 'off';
  name.placeholder = 'Name';
  name.setAttribute('aria-label', 'Name');
  name.dataset.field = 'name';
  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.inputMode = 'numeric';
  pin.autocomplete = 'new-password';
  pin.setAttribute('aria-label', 'PIN');
  pin.dataset.field = 'pin';
  const saved = !!u.id;
  pin.maxLength = Number(maxInput.value) || S.cfg?.maxPinLen || 8;
  const role = roleSelect(ROLES, u.role || 'standard', 'Role');
  role.dataset.field = 'role';
  // A new row starts at the New user defaults (Settings > Access > Users).
  const never = check('Never sign out', { checked: saved ? !!u.persistent : !!S.cfg?.newUserDefaults?.persistent });
  never.input.dataset.flag = 'persistent';
  // No PIN: a saved user's PIN goes on Save, a new row saves with none. A typed PIN wins.
  const noPin = check('No PIN', { checked: saved && !u.len });
  noPin.input.dataset.flag = 'noPin';
  // A saved user needs two clicks and the Advanced Settings PIN; a row never saved has nothing to lose.
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
      setNote(`Remove "${row.dataset.was}"? After you save, this cannot be undone: their PIN and profile are deleted.`);
      return;
    }
    if (S.pinSet && !(await askPin('Removing a user needs the Advanced Settings PIN.'))) return disarm();
    setNote('');
    row.remove();
    touched();
  });
  rm.addEventListener('blur', disarm);
  const out = saved
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
  const rights = btn('pb-btn--outline pb-btn--sm', 'Edit user rights', () => {
    const t = rightsTab(rid);
    tabs.select(t.tab);
    rightsPanel.show(t.pick);
  });
  const row = el(
    'div',
    'sode__users-row',
    el('div', 'sode__users-in', name),
    el('div', 'sode__users-in', pin),
    el('span', 'pb-select-wrap', role),
    noPin.label,
    never.label,
    prof,
    rights,
    out,
    rm,
  );
  row.dataset.rid = String(rid);
  row.dataset.was = saved ? u.name : '';
  row.dataset.uid = saved ? u.id : '';
  row.dataset.hadPin = u.len ? '1' : '';
  row.dataset.pinNeeded = u.pinNeeded ? '1' : '';
  paintProf();
  name.addEventListener('input', paintProf);
  for (const f of [name, pin]) f.addEventListener('input', touched);
  for (const f of [role, never.input, noPin.input]) f.addEventListener('change', touched);
  rows.append(row);
  if (saved) rightsPanel.adoptUser(rid, u);
  paintPin(row, readRow(row));
  return name;
}

onMsg('userSignOutAllResult', (m) => {
  const w = signing.get(m.id);
  signing.delete(m.id);
  if (w) w.out.disabled = false;
  if (m.ok) return setNote(`Signed ${w?.name || 'the user'} out everywhere.`);
  if (m.recheck) {
    askRecheck('Signing a user out everywhere').then((ok) => {
      if (ok) w?.out.click();
      else fail(m.err);
    });
    return;
  }
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
  rows.replaceChildren();
  clientsTab.clear();
  clientsTab.forget();
  rightsPanel.reset();
  for (const u of listed) addRow(u);
  ownerPick.clear();
  paintOwners();
  for (const c of extras.apiClients) clientsTab.addRow(c);
  rightsPanel.adoptGuest(extras.guestRights);
  rightsPanel.refresh();
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

// The directory is not in the hello (a locked page gets that too): the holder of the Advanced Settings PIN asks for it.
const load = () => {
  wsSend({ t: 'usersList', id: ++listSeq });
};
onMsg('usersListResult', (m) => {
  if (m.id !== listSeq) return;
  if (!m.ok) {
    // a stale unlock: ask, then read the directory again
    if (m.locked && isOpen) reask('Managing users needs the Advanced Settings PIN.').then((ok) => (ok ? load() : closeUsers()));
    else if (isOpen) fail(m.err); // a Role refusal: no PIN opens it
    return;
  }
  listed = m.users || [];
  listedMe = m.me || '';
  extras = { apiClients: m.apiClients || [], guestRights: m.guestRights || {} };
  loaded = true;
  emit('usersListed', m);
  paintSummary();
  if (isOpen && !edited) fill();
});
on('unlockResult', (m) => {
  if (m.ok) load();
});
on('locked', () => {
  listed = [];
  extras = { apiClients: [], guestRights: {} };
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
  // It is possible that a trusted user has not entered the PIN: a popup asks instead of an editor with no users.
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
  tabs.select('users');
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
  clientsTab.shownNow(false);
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

function readRow(row) {
  const val = (k) => row.querySelector(`input[data-field="${k}"]`).value;
  const flag = (k) => row.querySelector(`input[data-flag="${k}"]`).checked;
  return {
    rid: Number(row.dataset.rid),
    id: row.dataset.uid,
    name: val('name'),
    was: row.dataset.was,
    pin: val('pin'),
    role: row.querySelector('select[data-field="role"]').value,
    persistent: flag('persistent'),
    noPin: flag('noPin'),
    hadPin: !!row.dataset.hadPin,
    pinNeeded: !!row.dataset.pinNeeded,
  };
}

function readRows() {
  return [...rows.children].map(readRow);
}

function doSave() {
  err.hidden = true;
  setNote('');
  const users = readRows();
  const acc = accessReq({
    entries: rightsPanel.read(),
    users: users.map((u) => ({ rid: u.rid, name: u.name.trim() })),
    clients: clientsTab.read(),
    off: S.cfg?.numberOffset ?? 0,
    levels: S.levels.length,
  });
  if (acc.error) return fail(acc.error);
  const req = mergeAccess(usersReq(users, Math.trunc(Number(maxInput.value)) || 0, panelOwners()), acc, loaded);
  const id = ++seq;
  if (!wsSend({ t: 'users', id, usersReq: req })) return fail('Not connected to Sode.');
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
  if (m.recheck && (await askRecheck('Saving these user changes'))) return doSave();
  if (m.locked && (await reask('Managing users needs the Advanced Settings PIN again.'))) return doSave(); // the PIN was entered: save again
  fail(m.err);
});

maxInput.addEventListener('input', () => {
  touched();
  for (const p of rows.querySelectorAll('input[type=password]')) p.maxLength = Number(maxInput.value) || 12;
});

paintSummary();
