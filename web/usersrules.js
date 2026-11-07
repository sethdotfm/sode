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

// Salted hashes hide whether saved users share a PIN, so going back to PIN only needs a new PIN
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
  if (missing.length) out.push(`Give ${nameList(missing)} a new PIN to switch to PIN only. Sode can't tell whether saved PINs match.`);
  if (same.length) out.push(`${nameList(same)} have the same new PIN. Each needs a different one.`);
  return out.join(' ');
}

// Saved users whose name was edited. A rename ends the user's sessions on every browser (ADR 0001).
export const renamed = (rows) => rows.filter((r) => r.was && r.name.trim() !== r.was);

export const RENAME_HINT = 'Renaming a user signs them out everywhere. Their Profile and PIN stay.';

export function renameNotice(rows) {
  const names = renamed(rows).map((r) => `${r.was} to ${r.name.trim() || '(no name)'}`);
  return names.length ? `Renaming ${names.join(', ')} signs that user out everywhere. Their Profile and PIN stay.` : '';
}

// Usernames for rows that have none: the Name without spaces. Keep in step with config.Normalize's
// username rules (24 runes, printable, no spaces, unique ignoring case); the server still checks.
const MAX_USERNAME = 24;
export function prefillUsernames(rows) {
  const taken = new Set(rows.map((r) => r.username.trim().toLowerCase()).filter(Boolean));
  return rows.map((r) => {
    const own = r.username.trim();
    if (own) return own;
    const base = [...r.name.replace(/[\p{White_Space}\p{C}]/gu, '')].slice(0, MAX_USERNAME);
    if (!base.length) return '';
    let pick = base.join('');
    for (let n = 2; taken.has(pick.toLowerCase()); n++) {
      const tail = String(n);
      pick = base.slice(0, MAX_USERNAME - tail.length).join('') + tail;
    }
    taken.add(pick.toLowerCase());
    return pick;
  });
}
