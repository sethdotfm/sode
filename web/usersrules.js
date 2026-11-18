// Manage users' pure rules. No imports, so Node tests them.

// Wire values of config.Role*; the server checks every action against them.
export const ROLES = Object.freeze([
  { value: 'standard', label: 'Standard user' },
  { value: 'trusted', label: 'Trusted user' },
  { value: 'administrator', label: 'Administrator' },
]);
// A Guest is never an Administrator.
export const GUEST_ROLES = Object.freeze(ROLES.slice(0, 2));

// rows: [{id, name, pin, role, persistent}]. id is the saved user's ("" for a new row), so a
// rename keeps everything of theirs. An unchanged Guest role is left out (the server keeps it).
export function usersReq(rows, maxPinLen, guestRole, savedGuestRole) {
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
    ...(guestRole && guestRole !== savedGuestRole ? { guestRole } : {}),
  };
}
