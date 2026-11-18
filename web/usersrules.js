// Manage users' pure rules. No imports, so Node tests them.

export const MODES = Object.freeze([
  { value: 'pin', label: 'PIN only' },
  { value: 'username', label: 'Name and PIN' },
]);

// Wire values of config.Role*; the server checks every action against them.
export const ROLES = Object.freeze([
  { value: 'standard', label: 'Standard user' },
  { value: 'trusted', label: 'Trusted user' },
  { value: 'administrator', label: 'Administrator' },
]);
// A Guest is never an Administrator.
export const GUEST_ROLES = Object.freeze(ROLES.slice(0, 2));

// rows: [{id, name, pin, role, persistent}]. id is the saved user's ("" for a new row), so a
// rename keeps everything of theirs. An unchanged mode or Guest role is left out (the server
// keeps it). panelOwners: removed user id to the id of the user who takes their panels.
export function usersReq(rows, maxPinLen, mode, savedMode, guestRole, savedGuestRole, panelOwners = {}) {
  const users = rows.map((r) => ({
    id: r.id,
    name: r.name.trim(),
    pin: r.pin,
    role: r.role || 'standard',
    persistent: r.persistent,
  }));
  return {
    users,
    maxPinLen,
    ...(mode !== savedMode ? { signInMode: mode } : {}),
    ...(guestRole && guestRole !== savedGuestRole ? { guestRole } : {}),
    ...(Object.keys(panelOwners).length ? { panelOwners } : {}),
  };
}

// The saved users that rows remove and that own panels, each with the saved users rows keep
// (by their names in rows) as new owners. pick is me when kept, else the first kept user.
export function ownerChoices(listed, rows, me) {
  const names = new Map(rows.filter((r) => r.id).map((r) => [r.id, r.name.trim()]));
  const kept = listed.filter((u) => names.has(u.id)).map((u) => ({ id: u.id, name: names.get(u.id) || u.name }));
  const pick = kept.some((k) => k.id === me) ? me : kept[0]?.id || '';
  return listed.filter((u) => u.panels > 0 && !names.has(u.id)).map((u) => ({ id: u.id, name: u.name, panels: u.panels, kept, pick }));
}

// Salted hashes hide whether saved users share a PIN. So a switch back to PIN only needs a new PIN
// typed for every user, all different. The server refuses the save otherwise.
export const needsNewPins = (mode, savedMode) => mode === 'pin' && savedMode === 'username';

// missing: users with no new PIN. same: users whose typed PIN another row also typed.
export function pinClashes(rows, mode, savedMode) {
  const none = { missing: [], same: [] };
  if (!needsNewPins(mode, savedMode)) return none;
  const count = new Map();
  for (const r of rows) if (r.pin) count.set(r.pin, (count.get(r.pin) || 0) + 1);
  return {
    missing: rows.filter((r) => !r.pin).map((r) => r.name.trim()),
    same: rows.filter((r) => r.pin && count.get(r.pin) > 1).map((r) => r.name.trim()),
  };
}

const nameList = (names) => (names.length > 1 ? `${names.slice(0, -1).join(', ')} and ${names.at(-1)}` : names[0]);

export function clashNotice({ missing, same }) {
  const out = [];
  if (missing.length) out.push(`Give ${nameList(missing)} a new PIN to switch to PIN only. Sode cannot tell whether saved PINs match.`);
  if (same.length) out.push(`${nameList(same)} have the same new PIN. Each needs a different one.`);
  return out.join(' ');
}
