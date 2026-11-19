// Users: the directory (users, longest PIN, Roles, API clients, Rights) has its own dialog. Its edits stay staged when
// it closes and save with Settings' one Save (usersstage.js).
// Each saved user has a Profile... button (bus 'openProfile'), also in a Settings list for a Trusted user. The server holds every rule.

import { $, el, S, on, onMsg, emit, wsSend, sentence, LINK_DOWN } from './core.js';
import { diffRows } from './savereviewrules.js';
import { makeUsersSource } from './usersstage.js';
import { trusted } from './trusted.js';
import { askRecheck } from './recheck.js';
import { canSeeProfiles, profileButtonOff } from './profilesrules.js';
import { accessReq, rightsTab } from './rightsrules.js';
import { makeTabs } from './tabs.js';
import { btn, check, field as formField } from './formbits.js';
import { modal } from './modal.js';
import { makeRightsTab } from './usersrights.js';
import { makeClientsTab } from './usersclients.js';
import { ROLES, ownerChoices, pinPlaceholder } from './usersrules.js';

const manage = $('users-manage');
const people = $('users-people');

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
const err = el('div', { class: 'pb-note pb-note--error', role: 'alert', hidden: true });
const note = el('span', { class: 'sode__hint', role: 'status', 'aria-live': 'polite' });

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

let isOpen = false;
let ridSeq = 0;
const usersPanel = el(
  'div',
  null,
  table,
  el('div', 'sode__users-acts', addBtn),
  owners,
  el('div', 'sode__users-opts', formField('usr-maxpin', 'Longest PIN (digits)', el('span', 'pb-number-wrap', maxInput))),
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
const dlg = modal({
  titleId: 'users-title',
  title: 'Manage users',
  cls: 'sode__filters sode__users-dlg',
  body: [tabs.node, usersPanel, clientsTab.node, rightsPanel.node, err],
  foot: [note, btn('pb-btn--outline', 'Done', () => closeUsers())],
  onEsc: () => closeUsers(),
});

let back = null;
let outSeq = 0;
let listSeq = 0;
let listed = []; // the directory the server sent; empty until it does.
let listedMe = ''; // the id of the user asking, the default new owner
const ownerPick = new Map(); // removed user id to the chosen new owner id
let extras = { apiClients: [], guestRights: {} }; // clients and Guest, sent with it.
let loaded = false;
let lastList = null; // the usersList answer the dialog filled from
const signing = new Map();

const setNote = (text, error = false) => {
  note.textContent = text;
  note.classList.toggle('sode__error-text', error);
};
function touched() {
  stage.changed();
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
      return formField(s.id, `New owner of ${a.name}'s ${what}`, el('span', 'pb-select-wrap', s));
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
  // A saved user needs two clicks; a row never saved has nothing to lose.
  const disarm = () => {
    rm.dataset.armed = '';
    rm.textContent = 'Remove';
    rm.classList.replace('pb-btn--danger', 'pb-btn--ghost');
  };
  const rm = btn('pb-btn--ghost pb-btn--sm', 'Remove', () => {
    if (!row.dataset.was) {
      row.remove();
      touched();
      return;
    }
    if (!rm.dataset.armed) {
      rm.dataset.armed = '1';
      rm.textContent = 'Confirm remove';
      rm.classList.replace('pb-btn--ghost', 'pb-btn--danger');
      setNote(`Remove "${row.dataset.was}": Save deletes their PIN and Profile.`);
      return;
    }
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
        if (!wsSend({ t: 'userSignOutAll', name: u.name, id })) return fail(LINK_DOWN);
        out.disabled = true;
        signing.set(id, { out, name: u.name });
      })
    : el('span'); // a new row has none; the span keeps the columns in line
  const prof = btn('pb-btn--outline pb-btn--sm', 'Edit Profile', () => emit('openProfile', { name: row.dataset.was }));
  prof.setAttribute('aria-haspopup', 'dialog');
  const paintProf = () => {
    prof.disabled = profileButtonOff(row.dataset.was, name.value);
    if (prof.disabled) prof.dataset.pbTip = 'Save the user first';
    else delete prof.dataset.pbTip;
  };
  const rights = btn('pb-btn--outline pb-btn--sm', 'Edit Rights', () => {
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
  err.hidden = true;
  if (lastList) stage.adopt(lastList);
  stage.changed();
}

function paintPeople() {
  const names = S.userNames || [];
  const show = canSeeProfiles(names, trusted());
  people.hidden = !show;
  if (!show) return people.replaceChildren();
  people.replaceChildren(
    ...names.map((n) => {
      const b = btn('pb-btn--outline pb-btn--sm', 'Edit Profile', () => emit('openProfile', { name: n }));
      b.setAttribute('aria-haspopup', 'dialog');
      b.setAttribute('aria-label', `Edit Profile of ${n}`);
      return el('div', 'sode__names', el('span', 'sode__label', n), b);
    }),
  );
}

const load = () => {
  wsSend({ t: 'usersList', id: ++listSeq });
};
onMsg('usersListResult', (m) => {
  if (m.id !== listSeq) return;
  if (!m.ok) {
    if (isOpen) fail(m.err); // a Role refusal: no PIN opens it
    return;
  }
  // Staged edits keep the list they were made against. So another person's save shows as a conflict, and
  // their other changes stay out of this dialog.
  const keep = staged();
  if (!keep) {
    listed = m.users || [];
    listedMe = m.me || '';
    extras = { apiClients: m.apiClients || [], guestRights: m.guestRights || {} };
  }
  loaded = true;
  emit('usersListed', m);
  paintPeople();
  if (!keep) {
    lastList = m;
    fill();
  }
});
// policy.go answers usersList to an Administrator only; anyone else would only add a refused row to the Communication Log.
let admin = false;
on('access', (a) => {
  paintPeople();
  const was = admin;
  admin = a.role === 'administrator';
  if (admin && (!was || !loaded)) load();
});
on('conn', paintPeople);
on('hello', () => {
  paintPeople();
  if (admin) load();
});

function openUsers() {
  back = document.activeElement;
  isOpen = true;
  setNote('');
  if (!staged()) load();
  tabs.select('users');
  dlg.show();
  dlg.dialog.focus();
}

// Closing keeps the edits: they are unsaved changes of Settings until its Save or exit.
function closeUsers() {
  isOpen = false;
  clientsTab.shownNow(false);
  dlg.hide();
  if (back?.isConnected) back.focus();
}
manage.addEventListener('click', openUsers);

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

// The dialog as stageFields reads it, or {error} when a Rights entry does not parse.
function state() {
  const users = readRows();
  const acc = accessReq({
    entries: rightsPanel.read(),
    users: users.map((u) => ({ rid: u.rid, name: u.name.trim() })),
    clients: clientsTab.read(),
    off: S.cfg?.numberOffset ?? 0,
    levels: S.levels.length,
  });
  if (acc.error) return { error: `Manage users: ${acc.error}` };
  const listedOf = (id) => listed.find((u) => u.id === id);
  return {
    users: users.map((u, i) => ({
      ...u,
      rights: acc.users[i].rights || listedOf(u.id)?.rights || {},
      noAutoLock: 'noAutoLock' in acc.users[i] ? acc.users[i].noAutoLock : !!listedOf(u.id)?.noAutoLock,
    })),
    clients: clientsTab.read().map((c, i) => ({ ...c, rights: acc.apiClients[i].rights })),
    guestRights: 'guestRights' in acc ? acc.guestRights : extras.guestRights,
    maxPinLen: Math.trunc(Number(maxInput.value)) || 0,
    owners: panelOwners(),
  };
}

const rowOf = (pick) => [...rows.children].find(pick);
const stage = makeUsersSource({
  filled: () => !!lastList,
  state,
  refill: () => {
    lastList = null;
    if (loaded) load();
    stage.changed();
  },
  pinSaved(id, value) {
    const row = rowOf((r) => r.dataset.uid === id);
    if (!row) return;
    row.querySelector('input[data-field="pin"]').value = '';
    row.dataset.hadPin = value ? '1' : '';
    row.querySelector('input[data-flag="noPin"]').checked = !value;
    paintPin(row, readRow(row));
  },
  userAdded(rid, u) {
    const row = rowOf((r) => Number(r.dataset.rid) === rid);
    if (!row) return;
    row.dataset.uid = u.id;
    row.dataset.was = u.name;
    row.dataset.hadPin = u.len ? '1' : '';
    row.querySelector('input[data-field="pin"]').value = '';
    rightsPanel.adoptUser(rid, u);
    paintPin(row, readRow(row));
  },
  clientAdded: (name) => clientsTab.saved(name),
});
// Apply an Access preset (accesspreset.js) stages a listed user's flags as a hand edit would.
export function stageUserFlags(id, { persistent, noAutoLock }) {
  const row = rowOf((r) => r.dataset.uid === id);
  if (!row) return;
  row.querySelector('input[data-flag="persistent"]').checked = persistent;
  rightsPanel.setNoAutoLock(Number(row.dataset.rid), noAutoLock);
  touched();
}

// Edits wait in the dialog until Settings saves them or opens again.
const staged = () => {
  const src = stage.source;
  return !!lastList && diffRows(src.base(), src.now()).length > 0;
};

maxInput.addEventListener('input', () => {
  touched();
  for (const p of rows.querySelectorAll('input[type=password]')) p.maxLength = Number(maxInput.value) || 12;
});

paintPeople();
