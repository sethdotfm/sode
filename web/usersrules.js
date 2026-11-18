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
// keeps it).
export function usersReq(rows, maxPinLen, mode, savedMode, guestRole, savedGuestRole) {
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
  };
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
