// The rules of a take with tie routes (docs/tielines-design.md, Takes with plain routes and tie routes). Pure, so
// node tests run them. tieflow.js drives the pickers and the confirm with them.
//
// routes: each route of the take, in plan order: { src, dst, levels, tie, item }. src and dst are router references
// ({ router, n }); tie: the ends are on two routers; item: the plan item of the route.
//
// picks: one entry for each tie route, by its place in routes. After the person chose a path, it is { take, names }:
// take is the route that POST /takes gets, and names are the tie line names. After Clear this route, it is
// { cleared: true }. Else it is empty.

const bare = (r) => ({ src: r.src, dst: r.dst, levels: r.levels });

// tieIndexes gives the place of each tie route.
export const tieIndexes = (routes) => routes.flatMap((r, i) => (r.tie ? [i] : []));

// nextTie gives the first tie route at or after from that has no pick, or -1.
export function nextTie(routes, picks, from) {
  for (let i = from; i < routes.length; i++) if (routes[i].tie && !picks[i]) return i;
  return -1;
}

// finalAt: the picker of tie route k is the last step, so its Take sends the take. This is true when the take has
// only tie routes and k is the last of them. Then the picker replaces the churning-letter confirm (The Tie line
// picker, rule 9). When the take has a plain route, the confirm comes after the pickers.
export const finalAt = (routes, k) => routes.every((r) => r.tie) && tieIndexes(routes).at(-1) === k;

// planRequest gives the routes of the plan of tie route k. These are every plain route, the tie routes with a pick,
// and route k with no ties. The server counts the other routes. Thus it does not offer a tie line that another route
// uses for a different source. A tie line with the same source is shared. The tie routes after k have no pick yet
// and stay out.
export function planRequest(routes, picks, k) {
  const out = [];
  routes.forEach((r, i) => {
    if (!r.tie || i === k) out.push(bare(r));
    else if (picks[i]?.take) out.push(picks[i].take);
  });
  return out;
}

// takeRequest gives the routes of the take: every plain route and each tie route with its pick, in plan order. A
// cleared route stays out. at gives the place in routes of each route of the request.
export function takeRequest(routes, picks) {
  const out = { routes: [], at: [] };
  routes.forEach((r, i) => {
    if (r.tie && !picks[i]?.take) return;
    out.routes.push(r.tie ? picks[i].take : bare(r));
    out.at.push(i);
  });
  return out;
}

// keptItems gives the plan items that stay after Clear this route, for the confirm. A tie item gets the names of its
// tie lines, so the confirm marks it "Via TIE n". A tie item has one route.
export function keptItems(items, routes, picks) {
  const out = [];
  for (const it of items) {
    const i = routes.findIndex((r) => r.tie && r.item === it);
    if (i < 0) out.push(it);
    else if (picks[i]?.take) out.push({ ...it, via: picks[i].names.join(', ') });
  }
  return out;
}

// changedAt gives the place in routes of the first route that a 409 tie_line_changed body names, or -1. Each route of
// the body has route, its place in the request; at maps it back.
export function changedAt(body, at) {
  const ks = (body?.routes || []).map((r) => at[r.route]).filter((k) => k != null);
  return ks.length ? Math.min(...ks) : -1;
}

// resetFrom drops each pick from route k on, but keeps Clear this route. The person chooses those routes again,
// since a later route can share a tie line with route k.
export const resetFrom = (picks, k) => picks.map((p, i) => (i < k || p?.cleared ? p : undefined));

// failedText says why a route of a 200 answer failed: the hop that failed, else the error.
export function failedText(r) {
  const bad = (r.hops || []).find((h) => h.result === 'failed');
  return bad ? `the hop to ${bad.tieName} failed` : r.error || 'failed';
}

// reasonText names why a route has no usable path (The Tie line picker, rule 10). prefix gives the prefix of a router
// UID.
export function reasonText(x, prefix) {
  switch (x.why) {
    case 'no_tie_line':
      return `No tie line from router ${prefix(x.from)} to router ${prefix(x.to)}`;
    case 'router_down':
      return `Router ${prefix(x.router)} is down`;
    case 'protected':
      return `${x.tieName || x.tie} is protected by ${x.by}`;
    case 'taken':
      return `${x.tieName || x.tie} carries another source in this take`;
  }
  return x.why;
}
