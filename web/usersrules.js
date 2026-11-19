// Manage users' pure rules. No imports, so Node tests them.

// Wire values of config.Role*; the server checks every action against them.
export const ROLES = Object.freeze([
  { value: 'standard', label: 'Standard user' },
  { value: 'trusted', label: 'Trusted user' },
  { value: 'administrator', label: 'Administrator' },
]);
// A Guest is never an Administrator.
export const GUEST_ROLES = Object.freeze(ROLES.slice(0, 2));

// The PIN box's placeholder for a row {id, hadPin, noPin, pinNeeded}: what an empty box saves.
export function pinPlaceholder(r) {
  if (!r.id) return r.noPin ? 'no PIN' : 'new PIN';
  if (r.hadPin) return r.noPin ? 'no PIN after Save' : 'unchanged';
  return r.pinNeeded ? 'PIN needed' : 'no PIN';
}

// The saved users that rows remove and that own panels, each with the saved users rows keep
// (by their names in rows) as new owners. pick is me when kept, else the first kept user.
export function ownerChoices(listed, rows, me) {
  const names = new Map(rows.filter((r) => r.id).map((r) => [r.id, r.name.trim()]));
  const kept = listed.filter((u) => names.has(u.id)).map((u) => ({ id: u.id, name: names.get(u.id) || u.name }));
  const pick = kept.some((k) => k.id === me) ? me : kept[0]?.id || '';
  return listed.filter((u) => u.panels > 0 && !names.has(u.id)).map((u) => ({ id: u.id, name: u.name, panels: u.panels, kept, pick }));
}
