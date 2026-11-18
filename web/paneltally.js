// Panel pages keep the tally from GET /api/v1/tally up to date with the WebSocket's tally frames. A full read is
// needed only after the socket connects again, or while it is down. A frame's rows are [dst, UI level, src], the same
// numbers as the API's { dst, level, src }.

// A new answer with each row of u in it; t is the shared object from withEtag (etagfetch.js), so it stays as it was.
export function withRoutes(t, u) {
  const routes = t.routes.slice();
  const at = new Map(routes.map((r, i) => [`${r.dst} ${r.level}`, i]));
  for (const [dst, level, src] of u) {
    const k = `${dst} ${level}`;
    const row = { dst, level, src };
    if (at.has(k)) routes[at.get(k)] = row;
    else at.set(k, routes.push(row) - 1);
  }
  return { ...t, routes };
}

// A Hidden caller's tally leaves out a route whose source it cannot see (docs/api.md, Visibility); the socket does not.
// For such a caller a frame only asks for a full read.
export const framesApply = (rights) => rights?.visibility !== 'hidden';

// Whether a and b hold the same routes, in any order: the slow check of a tally kept from frames.
export function sameRoutes(a, b) {
  if (a.routes.length !== b.routes.length) return false;
  const at = new Map(a.routes.map((r) => [`${r.dst} ${r.level}`, r.src]));
  return b.routes.every((r) => at.get(`${r.dst} ${r.level}`) === r.src);
}
