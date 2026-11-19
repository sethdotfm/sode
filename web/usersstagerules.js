// Manage users as fields of the one Save (docs/agents/settings-saves.md). There is one key per attribute of a user
// or API client, so two people who change different attributes of one user do not conflict. Pure, Node-tested. Keep
// the keys in step with internal/app/onesavefields.go.

import { ROLES } from './usersrules.js';

const USER_ATTRS = ['name', 'role', 'rights', 'persistent', 'noAutoLock'];

// list: the usersList answer plus maxPinLen. The base the dialog's changes are measured against.
export function listedFields(list) {
  const out = { maxPinLen: list.maxPinLen, guestRights: list.guestRights || {} };
  for (const u of list.users) {
    const k = `user/${u.id}/`;
    out[k + 'name'] = u.name;
    out[k + 'role'] = u.role || 'standard';
    out[k + 'rights'] = u.rights || {};
    out[k + 'persistent'] = !!u.persistent;
    out[k + 'noAutoLock'] = !!u.noAutoLock;
    out[k + 'remove'] = false;
  }
  for (const c of list.apiClients || []) clientBase(out, c.name, c);
  return out;
}

function clientBase(out, name, c) {
  out[`client/${name}/scopes`] = c.scopes || [];
  out[`client/${name}/rights`] = c.rights || {};
  out[`client/${name}/remove`] = false;
}

const same = (a, b) => a.trim().toLowerCase() === b.trim().toLowerCase();

// st: the dialog now. users: rows {id, rid, name, pin, noPin, hadPin, role, persistent, noAutoLock, rights}.
// clients: rows {was, name, scopes, rights}. owners: removed user id to the id of the new owner of their panels.
// A listed API client that belongs to a panel or a user is never removed here: the server keeps it.
export function stageFields(list, st) {
  const out = { maxPinLen: st.maxPinLen, guestRights: st.guestRights || {} };
  const rows = new Map(st.users.filter((r) => r.id).map((r) => [r.id, r]));
  for (const u of list.users) {
    const r = rows.get(u.id);
    const k = `user/${u.id}/`;
    if (!r) {
      out[k + 'remove'] = { owner: st.owners?.[u.id] || '' };
      continue;
    }
    const v = { ...r, name: r.name.trim(), role: r.role || 'standard', rights: r.rights || {}, persistent: !!r.persistent, noAutoLock: !!r.noAutoLock };
    for (const a of USER_ATTRS) out[k + a] = v[a];
    out[k + 'remove'] = false;
    if (r.pin) out[k + 'pin'] = r.pin;
    else if (r.noPin && r.hadPin) out[k + 'pin'] = '';
  }
  for (const r of st.users.filter((x) => !x.id)) {
    out[`user/new:${r.rid}/add`] = {
      name: r.name.trim(),
      pin: r.noPin ? '' : r.pin,
      role: r.role || 'standard',
      persistent: !!r.persistent,
      noAutoLock: !!r.noAutoLock,
      rights: r.rights || {},
    };
  }
  const kept = (c) => c.panel || c.ownerName;
  for (const c of list.apiClients || []) {
    const r = st.clients.find((x) => x.was && same(x.was, c.name) && same(x.name, c.name));
    if (r) clientBase(out, c.name, r);
    else if (kept(c)) clientBase(out, c.name, c);
    else out[`client/${c.name}/remove`] = true;
  }
  for (const r of st.clients) {
    if (r.was && same(r.was, r.name)) continue;
    out[`client/${r.name.trim()}/add`] = { scopes: r.scopes || [], rights: r.rights || {} };
  }
  return out;
}

const ATTR_LABELS = {
  name: 'Name',
  role: 'Role',
  pin: 'PIN',
  rights: 'Rights',
  persistent: 'Never sign out',
  noAutoLock: 'No auto-lock',
  remove: 'Remove',
  scopes: 'Scopes',
};
const OTHER_LABELS = { maxPinLen: 'Longest PIN (digits)', guestRights: 'Guest Rights' };

// {kind, id, attr} of an item key, or null.
export function splitKey(key) {
  const m = /^(user|client)\/(.+)\/(\w+)$/.exec(key);
  return m ? { kind: m[1], id: m[2], attr: m[3] } : null;
}

// names: user id to name. now: the row's new value, for an add.
export function stageLabel(r, names, now = r.now) {
  const s = splitKey(r.key);
  if (!s) return OTHER_LABELS[r.key] || r.key;
  if (s.attr === 'add') return `${s.kind === 'user' ? 'New user' : 'New API client'} ${s.kind === 'user' ? now?.name || '' : s.id}`.trim();
  const who = s.kind === 'user' ? names[s.id] || s.id : `API client ${s.id}`;
  return `${who}: ${ATTR_LABELS[s.attr] || s.attr}`;
}

const roleText = (v) => ROLES.find((r) => r.value === (v || 'standard'))?.label || v;
const LISTS = [
  ['sources', 'sources'],
  ['dsts', 'destinations'],
  ['levels', 'levels'],
  ['presets', 'presets'],
];

export function rightsText(r) {
  const parts = LISTS.filter(([k]) => Array.isArray(r?.[k])).map(([k, word]) => (r[k].length ? `${r[k].length} ${word}` : `no ${word}`));
  return parts.length ? parts.join(', ') : 'All';
}

const plural = (n, one, many) => (n === 1 ? one : many);

// list: the usersList answer, for the API clients that go with a removed user. Falls back to showValue's rules.
export function stageShow(r, v, list = { users: [] }) {
  const s = splitKey(r.key);
  if (r.key === 'guestRights') return rightsText(v);
  if (!s) return v == null ? 'Blank' : String(v);
  if (v === undefined) return s.attr === 'pin' ? 'Unchanged' : 'Blank';
  switch (s.attr) {
    case 'pin':
      return v ? 'New PIN' : 'No PIN';
    case 'rights':
      return rightsText(v);
    case 'role':
      return roleText(v);
    case 'scopes':
      return v.length ? v.join(', ') : 'None';
    case 'remove': {
      if (!v) return 'Kept';
      const gone = (s.kind === 'user' && list.users.find((u) => u.id === s.id)?.clients) || [];
      return gone.length ? `Removed, with ${plural(gone.length, 'API client', 'API clients')} ${gone.join(', ')}` : 'Removed';
    }
    case 'add':
      return s.kind === 'user' ? `${roleText(v.role)}, ${v.pin ? 'with a PIN' : 'no PIN'}` : v.scopes.join(', ') || 'No scopes';
  }
  if (v === true) return 'On';
  if (v === false) return 'Off';
  return v === '' ? 'Blank' : String(v);
}
