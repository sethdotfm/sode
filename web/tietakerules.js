// The rules of a take with tie routes (docs/tielines-design.md, Takes with plain routes and tie routes). Pure, so
// node tests run them. tieflow.js drives the pickers and the confirm with them.
//
// routes: each route of the take, in plan order: { src, dst, levels, tie, item, stored, last }. src and dst are router
// references ({ router, n }); tie: the ends are on two routers; item: the plan item of the route. stored: the tie
// lines of a preset or of a Redo.
//
// picks: one entry for each tie route, by its place in routes. When a question chose the first hops of the route
// only, it is { ids }: those tie lines, upstream first. After the route has its whole path, it is
// { take, names, prot, ids, seq }. take is the route that POST /takes gets. names are the tie line names, and prot
// the names that it protects. seq is the order in which the routes got their path. After Clear this route, it is
// { cleared: true }. Else it is empty.
//
// Questions: the picker asks one question for each hop that routes share. A hop is shared when the routes have the
// same source, levels and tie lines before it. Their best paths must also go from and to the same routers there.

import { best, stepsOf } from './tiepickrules.js';

const bare = (r) => ({ src: r.src, dst: r.dst, levels: r.levels });

// tieIndexes gives the place of each tie route.
export const tieIndexes = (routes) => routes.flatMap((r, i) => (r.tie ? [i] : []));

// openAt gives the place of each tie route that has no path yet and is not cleared, in plan order.
export const openAt = (routes, picks) => routes.flatMap((r, i) => (r.tie && !picks[i]?.cleared && !picks[i]?.take ? [i] : []));

// doneAt gives the place of each tie route with a path, in the order that they got it.
const doneAt = (routes, picks) =>
  tieIndexes(routes)
    .filter((i) => picks[i]?.take)
    .sort((a, b) => picks[a].seq - picks[b].seq);

// planRequest gives the routes of a plan, and at, the place in routes of each one. First come the plain routes, then
// the tie routes with a path in the order that they got it. Each open tie route with no ties comes last. The server counts
// the routes before each one. Thus it does not offer a tie line that another route uses for a different source.
// A tie line with the same source is shared. The take keeps this order (takeRequest), so the stale check sees
// the plan that the person saw. through names the tie lines that a question chose before.
export function planRequest(routes, picks) {
  const out = { routes: [], at: [] };
  const add = (r, i) => {
    out.routes.push(r);
    out.at.push(i);
  };
  routes.forEach((r, i) => r.tie || add(bare(r), i));
  for (const i of doneAt(routes, picks)) add(picks[i].take, i);
  for (const i of openAt(routes, picks)) add(picks[i]?.ids?.length ? { ...bare(routes[i]), through: picks[i].ids } : bare(routes[i]), i);
  return out;
}

// takeRequest gives the routes of the take: every plain route, then each tie route with its path, in the order of
// planRequest. A cleared route stays out. at gives the place in routes of each route of the request.
export function takeRequest(routes, picks) {
  const out = { routes: [], at: [] };
  routes.forEach((r, i) => {
    if (r.tie) return;
    out.routes.push(bare(r));
    out.at.push(i);
  });
  for (const i of doneAt(routes, picks)) {
    out.routes.push(picks[i].take);
    out.at.push(i);
  }
  return out;
}

// hopOf gives the question key of open tie route i and its hop: { key, from, hop: [router, router] }. plan is the
// route of the 409 body. A route with no path is a question of its own.
function hopOf(routes, plan, picks, i) {
  const r = routes[i];
  const pre = picks[i]?.ids || [];
  const j = plan ? best(plan.paths, pre) : -1;
  if (j < 0) return { key: `alone ${i}`, from: 0, hop: [] };
  const p = plan.paths[j];
  const from = Math.min(pre.length, Math.max(stepsOf(p) - 1, 0));
  const hop = [p.hops[from]?.router, p.hops[from + 1]?.router];
  // A preset or a Redo that stores another tie line for this hop is a question of its own.
  const mark = r.stored ? (r.stored[from] ?? '') : null;
  return { key: JSON.stringify([r.src.router, r.src.n, [...r.levels].sort((a, b) => a - b), pre.slice(0, from), hop, mark, !!r.last]), from, hop };
}

// nextQuestion gives the next question, or null when every tie route has a path or is cleared. plans[i] is the route
// of the 409 body for the route at place i. split lists the keys that the person opened per destination.
// The question is { at, members, from, to, only, key }. at is the route whose plan the picker shows. members are
// the routes that share the hop, at first. from is the step of the hop. When every member ends on one router, the
// question goes on to the destination (to is null). Else it is the one hop (to is from), and only names the router
// that the hop goes to.
export function nextQuestion(routes, plans, picks, split = []) {
  const open = openAt(routes, picks);
  if (!open.length) return null;
  const at = open[0];
  const lead = hopOf(routes, plans[at], picks, at);
  const members = split.includes(lead.key) ? [at] : open.filter((i) => i === at || hopOf(routes, plans[i], picks, i).key === lead.key);
  const one = members.every((i) => routes[i].dst.router === routes[at].dst.router);
  return { at, members, from: lead.from, to: one ? null : lead.from, only: one ? '' : lead.hop[1] || '', key: lead.key };
}

// questionIds gives what a question chose for each member, from path, the chosen path of route at: { k, ids, full }.
// ids are the tie lines up to the last step of the question. full: they reach the router of the destination.
export function questionIds(q, path, routes) {
  const n = stepsOf(path);
  const last = q.to == null ? n - 1 : Math.min(q.to, n - 1);
  const ids = path.hops.slice(0, last + 1).map((h) => h.tie?.id || '');
  const reach = path.hops[last + 1]?.router;
  return q.members.map((k) => ({ k, ids, full: reach === routes[k].dst.router }));
}

// questionHead names the hop of a question and its destinations: "A → B, for B.1 B.2". path is the chosen path,
// names the destinations as shown, prefix(uid) the prefix of a router.
export function questionHead(q, path, names, prefix) {
  const dsts = `for ${names.join(' ')}`;
  if (!path || !stepsOf(path)) return dsts;
  const n = stepsOf(path);
  const last = q.to == null ? n - 1 : Math.min(q.to, n - 1);
  const routers = path.hops.slice(q.from, last + 2).map((h) => prefix(h.router));
  return `${routers.join(' → ')}, ${dsts}`;
}

// finalAt: the question is the last step, so its Take sends the take. This is true when the take has only tie
// routes, the question has every open route, and it goes on to the destination. Then the picker replaces the
// churning-letter confirm (The Tie line picker, rule 9). When the take has a plain route, the confirm comes after the
// pickers.
export const finalAt = (routes, picks, q) => routes.every((r) => r.tie) && q.to == null && openAt(routes, picks).every((i) => q.members.includes(i));

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

// changedAt gives the place in routes of the route that a 409 tie_line_changed body names first, or -1. The body
// lists every tie route, so the first one with a changed tie line or a new Affected destination counts. Each route
// of the body has route, its place in the request; at maps it back.
export function changedAt(body, at) {
  const list = (body?.routes || []).filter((r) => at[r.route] != null);
  const hit = list.filter((r) => r.changed?.length || r.newAffected?.length);
  const ks = (hit.length ? hit : list).map((r) => at[r.route]);
  return ks.length ? Math.min(...ks) : -1;
}

// reopenPicks drops each choice, but keeps Clear this route. The person answers the questions again, since a route
// can share a tie line with the route that changed.
export const reopenPicks = (picks) => picks.map((p) => (p?.cleared ? p : undefined));

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

// takeRan tells if the server ran the take of answer m (tieTakeResult): it has results, which can be failed. A refusal
// before the take, as tie_line_changed, sent nothing.
export const takeRan = (m) => m?.status === 200 && Array.isArray(m.body?.results);
