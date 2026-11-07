// Manage users' pure rules. No imports, so Node tests them.

export const MODES = Object.freeze([
  { value: 'pin', label: 'PIN only' },
  { value: 'username', label: 'Username and PIN' },
]);

export const usernameColumn = (mode) => mode === 'username';

// rows: [{name, was, pin, trusted, persistent, username}]. A hidden column sends no username
// (the server keeps the saved ones) and an unchanged mode is left out (the server keeps it).
export function usersReq(rows, maxPinLen, mode, savedMode) {
  const withNames = usernameColumn(mode);
  const users = rows.map((r) => ({
    name: r.name.trim(),
    was: r.was,
    pin: r.pin,
    trusted: r.trusted,
    persistent: r.persistent,
    ...(withNames ? { username: r.username.trim() } : {}),
  }));
  return { users, maxPinLen, ...(mode !== savedMode ? { signInMode: mode } : {}) };
}
