// The one place that knows scopes, their order and labels. Pure, no imports.
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

export const scopeLabel = (id) => [DEFAULT, ...SCOPES].find((s) => s.id === id)?.label ?? '';

export const scopeIds = (withAll = false) => [...(withAll ? [DEFAULT] : []), ...SCOPES].map((s) => s.id);

export const usableScopes = (hasUser) => SCOPES.filter((s) => s.id !== 'user' || hasUser).map((s) => s.id);

export const scopeOptions = (hasUser) => ['local', ...usableScopes(hasUser).filter((id) => id !== 'local')]
  .map((id) => [id, scopeLabel(id)]);
