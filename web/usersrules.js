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

// Going back to PIN only: users whose saved PIN another user has (row.shared, from the server)
// and who have no new PIN typed yet. The server refuses the save while any remain.
export const pinClashes = (rows, mode, savedMode) =>
  mode === 'pin' && savedMode === 'username' ? rows.filter((r) => r.shared && !r.pin).map((r) => r.name.trim()) : [];

export function clashNotice(names) {
  if (!names.length) return '';
  if (names.length === 1) return `${names[0]} shares a PIN with another user. Give ${names[0]} a new PIN to switch to PIN only.`;
  return `${names.slice(0, -1).join(', ')} and ${names.at(-1)} share a PIN with other users. Give each a new PIN to switch to PIN only.`;
}

// Saved users whose name was edited. A rename ends the user's sessions on every browser (ADR 0001).
export const renamed = (rows) => rows.filter((r) => r.was && r.name.trim() !== r.was);

export const RENAME_HINT = 'Renaming a user signs them out everywhere. Their Profile and PIN stay.';

export function renameNotice(rows) {
  const names = renamed(rows).map((r) => `${r.was} to ${r.name.trim() || '(no name)'}`);
  return names.length ? `Renaming ${names.join(', ')} signs that user out everywhere. Their Profile and PIN stay.` : '';
}
