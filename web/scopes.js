// Where a filter set or preset lives: the one place that knows the scopes,
// their order and their labels. Pure, no imports.
//   global: everyone's, in the server config (settings PIN to change)
//   user:   the logged-in user's profile
//   local:  this browser's localStorage
// Filter sets also have 'all', the built-in Default.

export const SCOPES = [
  { id: 'global', label: 'Everyone' },
  { id: 'user', label: 'Me' },
  { id: 'local', label: 'This browser' },
];
const DEFAULT = { id: 'all', label: 'Default' };

// The label of a scope id ('' for an unknown one).
export const scopeLabel = (id) => [DEFAULT, ...SCOPES].find((s) => s.id === id)?.label ?? '';

// Scope ids in order; 'all' first when asked (filter sets).
export const scopeIds = (withAll = false) => [...(withAll ? [DEFAULT] : []), ...SCOPES].map((s) => s.id);

// The scopes someone may save to: 'Me' only while a user is logged in.
export const usableScopes = (hasUser) => SCOPES.filter((s) => s.id !== 'user' || hasUser).map((s) => s.id);

// Save as's choices, This browser first: [[id, label], ...].
export const scopeOptions = (hasUser) => ['local', ...usableScopes(hasUser).filter((id) => id !== 'local')]
  .map((id) => [id, scopeLabel(id)]);
