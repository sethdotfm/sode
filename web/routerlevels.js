// Rights `levels` and the level subset of a panel name their router: { <router UID>: [UI levels] }. A router the
// object does not name is not limited. A plain list is the request form, which names the one router; tests and old
// callers pass it. Keep in step with config.RouterLevels and levelsByRouter (internal/app/panellevels.go).

// levelsOn: the list for router, or null when it limits nothing there.
export function levelsOn(levels, router) {
  if (Array.isArray(levels)) return levels;
  if (!levels || typeof levels !== 'object') return null;
  const l = levels[router];
  return Array.isArray(l) ? l : null;
}

// putLevels: levels with router's list set to list, or dropped when list is null. The lists of other routers stay.
// Without a router UID it gives the plain list, which the server reads as the one router's. With no list too, it
// gives the levels as they are.
export function putLevels(levels, router, list) {
  if (!router) return list ?? (levels && typeof levels === 'object' && !Array.isArray(levels) ? levels : null);
  const out = levels && typeof levels === 'object' && !Array.isArray(levels) ? { ...levels } : {};
  if (list) out[router] = list;
  else delete out[router];
  return Object.keys(out).length ? out : null;
}
