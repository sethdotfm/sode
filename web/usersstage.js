// Manage users as a save source of the one Save (savesources.js). The dialog stages edits; Settings' Save sends them.
// The base and versions come from the usersList answer the dialog last filled from, and move only as fields save.

import { S, emit } from './core.js';
import { addSaveSource } from './savesources.js';
import { listedFields, splitKey, stageFields, stageLabel, stageShow } from './usersstagerules.js';

// hooks: filled() is true when the dialog holds a list. state() is the dialog as stageFields reads it, or {error}.
// refill() fills it again. pinSaved, userAdded and clientAdded update a row after its field saved.
export function makeUsersSource(hooks) {
  let list = { users: [], apiClients: [], guestRole: 'standard', guestRights: {}, maxPinLen: 8 };
  let base = {};
  let vers = {};

  const PLAIN = new Set(['maxPinLen', 'guestRole', 'guestRights']);
  const owns = (key) => PLAIN.has(key) || !!splitKey(key);
  const names = () => Object.fromEntries(list.users.map((u) => [u.id, u.name]));
  const now = () => {
    if (!hooks.filled()) return {};
    const st = hooks.state();
    return st.error ? {} : stageFields(list, st);
  };
  const dropItem = (prefix) => {
    for (const k of Object.keys(base)) if (k.startsWith(prefix)) delete base[k];
  };

  // m: a usersList answer. Called when the dialog fills from it.
  function adopt(m) {
    list = {
      users: (m.users || []).map((u) => ({ ...u })),
      apiClients: (m.apiClients || []).map((c) => ({ ...c })),
      guestRole: m.guestRole === 'trusted' ? 'trusted' : 'standard',
      guestRights: m.guestRights || {},
      maxPinLen: S.cfg?.maxPinLen || 8,
    };
    base = listedFields(list);
    vers = { ...(m.versions || {}), maxPinLen: S.versions?.maxPinLen };
  }

  function saved(key, value, r) {
    const s = splitKey(key);
    if (r.status === 'saved' && s?.attr === 'add') return added(s, value, r.value || {});
    if (r.status === 'saved' && s?.attr === 'remove') {
      dropItem(`${s.kind}/${s.id}/`);
      if (s.kind === 'user') list.users = list.users.filter((u) => u.id !== s.id);
      else list.apiClients = list.apiClients.filter((c) => c.name !== s.id);
      return;
    }
    if (s?.attr === 'pin') {
      hooks.pinSaved(s.id, value);
      return;
    }
    base[key] = value;
    if (r.version) vers[key] = r.version;
  }

  function added(s, value, made) {
    Object.assign(vers, made.versions || {});
    if (s.kind === 'user') {
      const u = {
        id: made.id,
        name: value.name,
        role: value.role,
        persistent: value.persistent,
        noAutoLock: value.noAutoLock,
        rights: value.rights,
        len: value.pin ? value.pin.length : 0,
      };
      list.users.push(u);
      for (const [k, v] of Object.entries(listedFields({ users: [u], apiClients: [] }))) if (k.startsWith('user/')) base[k] = v;
      hooks.userAdded(Number(s.id.slice(4)), u);
      return;
    }
    const c = { name: s.id, scopes: value.scopes, rights: value.rights };
    list.apiClients.push(c);
    base[`client/${s.id}/scopes`] = value.scopes;
    base[`client/${s.id}/rights`] = value.rights;
    base[`client/${s.id}/remove`] = false;
    hooks.clientAdded(s.id);
  }

  const source = {
    owns,
    base: () => base,
    now,
    version: (key) => vers[key],
    label: (r) => stageLabel(r, names()),
    show: (r, v) => stageShow(r, v, list),
    problem: () => (hooks.filled() && hooks.state().error) || '',
    saved,
    conflict(key, r) {
      vers[key] = r.version;
      if (r.value !== undefined) base[key] = r.value;
    },
    reset: () => hooks.refill(),
  };
  addSaveSource(source);

  return { adopt, source, changed: () => emit('saveSourceChanged') };
}
