// Item numbers: one number for a source or destination on any router, for the code that keys items by number. These
// are the Grid panel, a panel page's tally, Protects and names, and the panel editor's Grid lists. Pure.
// The number is place * SPAN + n. place is the router's place in GET /routers (the first router is 0), and n is its
// router number. So on the first router an item number is the router number, and a page with one router works on
// router numbers as before. A source of -1 (nothing routed) stays -1.
import { typedPrefix, prefixOf } from './routerprefix.js';

export const SPAN = 0x10000; // one more than dstNumberMax in internal/app/panelfixed.go
export const itemId = (place, n) => (n < 0 ? n : place * SPAN + (n % SPAN));
export const placeOf = (id) => (id < 0 ? 0 : Math.floor(id / SPAN));
export const routerN = (id) => (id < 0 ? id : id % SPAN);

// places: the routers in the order of GET /routers ({ uid, prefix, title }). A UID that the list does not have gets
// the next place, so its items stay apart and show as gone. Blank is the first router. With an empty list, the first
// UID it sees is the first router: a page that read no list has one router.
export function makePlaces(routers = []) {
  const known = routers.map((r) => ({ uid: r.uid, prefix: r.prefix || '', title: r.title || '' }));
  const all = known.slice();
  const place = (uid) => {
    if (!uid) return 0;
    let i = all.findIndex((r) => r.uid === uid);
    if (i < 0) i = all.push({ uid, prefix: '', title: '' }) - 1;
    return i;
  };
  const uid = (p) => all[p]?.uid || '';
  return {
    // many: Sode has two or more routers, so every item shows its prefix and every take names its router.
    many: known.length > 1,
    routers: known,
    place,
    uid,
    prefix: (p) => all[p]?.prefix || '',
    id: (u, n) => itemId(place(u), n),
    // The router reference of an item number; with no UID known, {n}, which the server puts on the first router.
    ref(id) {
      const u = uid(placeOf(id));
      return u ? { router: u, n: routerN(id) } : { n: routerN(id) };
    },
    // One end of a route in a take: a bare number while Sode has one router.
    end(id) {
      return known.length > 1 ? this.ref(id) : routerN(id);
    },
  };
}

// Rights whose source and destination lists are on item numbers, so rightsAllow and allowed (number only) work on
// items of every router. A plain number stays: it is on the first router. Levels and presets are as they were.
export function rightsIn(rights, places) {
  if (!rights) return rights;
  const out = { ...rights };
  const on = (x) => (x && typeof x === 'object' ? { n: places.id(x.router || '', x.n) } : x);
  for (const k of ['sources', 'dsts']) if (Array.isArray(rights[k])) out[k] = rights[k].map(on);
  return out;
}

// The same route ends on two routers: such a route needs a tie line, which Sode does not have yet.
export const acrossRouters = (src, dst) => src >= 0 && placeOf(src) !== placeOf(dst);

// A take body's routes on item numbers, as the server reads them.
export const wireRoutes = (routes, places) => routes.map((r) => ({ ...r, src: places.end(r.src), dst: places.end(r.dst) }));

// GET /panels/<id>/undo's routes on item numbers.
export const undoIn = (undo, places) =>
  undo?.routes ? { ...undo, routes: undo.routes.map((r) => ({ ...r, src: places.id(r.router, r.src), dst: places.id(r.router, r.dst) })) } : undo;

// The answers of each router as one answer on item numbers. first is the first router's answer, whose levels and
// settings the page uses. more is [{ place, reply }] for the other routers. With no more, first is given back as it is.
// The joined tally keeps the levels of each router in byPlace (tallyAt), so a route on a router takes that router's
// levels.
export function mergeTally(first, more = []) {
  if (!more.length) return first;
  const routes = [...(first?.routes || [])];
  const byPlace = [levelsPart(first)];
  for (const { place, reply } of more) {
    byPlace[place] = levelsPart(reply);
    for (const r of reply?.routes || []) routes.push({ dst: itemId(place, r.dst), level: r.level, src: itemId(place, r.src) });
  }
  return { ...first, routes, byPlace };
}

const levelsPart = (t) => ({ router: t?.router || '', levels: t?.levels || [], levelNumbering: t?.levelNumbering, defaultLevels: t?.defaultLevels || '' });

// The levels part of a tally answer for the router at place: { router, levels, levelNumbering, defaultLevels }. A
// tally of one router is its own part at place 0. null for a router the answer does not hold yet.
export const tallyAt = (tally, place) => tally?.byPlace?.[place] || (place === 0 && tally ? tally : null);

// GET /presets with each item's source and destinations on item numbers, so a Preset cell can hold routes on any
// router. A stored item holds router references (presetref.js).
export function presetsIn(reply, places) {
  if (!reply?.presets) return reply;
  const on = (x) => (x && typeof x === 'object' && Number.isInteger(x.n) ? { ...x, n: places.id(x.router || '', x.n) } : x);
  const item = (it) => ({ ...it, src: on(it.src), dsts: Array.isArray(it.dsts) ? it.dsts.map(on) : it.dsts });
  return { ...reply, presets: reply.presets.map((p) => (Array.isArray(p.items) ? { ...p, items: p.items.map(item) } : p)) };
}

export function mergeProtected(first, more = []) {
  if (!more.length) return first;
  const rows = [...(first?.protected || [])];
  for (const { place, reply } of more) for (const r of reply?.protected || []) rows.push({ ...r, dst: itemId(place, r.dst) });
  return { ...first, protected: rows };
}

// GET /names of each router. places is makePlaces's. With two or more routers the answer carries prefixes, each
// place's { prefix, numberOffset }, so a number shows as A.12 (namesFrom, numText).
export function mergeNames(first, more = [], places = null) {
  if (!places?.many) return first;
  const prefixes = [{ prefix: places.prefix(0), numberOffset: first?.numberOffset || 0 }];
  const sources = [...(first?.sources || [])];
  const destinations = [...(first?.destinations || [])];
  for (const { place, reply } of more) {
    prefixes[place] = { prefix: places.prefix(place), numberOffset: reply?.numberOffset || 0 };
    for (const x of reply?.sources || []) sources.push({ ...x, n: itemId(place, x.n) });
    for (const x of reply?.destinations || []) destinations.push({ ...x, n: itemId(place, x.n) });
  }
  for (let p = 0; p < prefixes.length; p++) prefixes[p] ||= { prefix: places.prefix(p), numberOffset: 0 };
  return { ...first, sources, destinations, prefixes };
}

// The number of an item as a person reads it: the router number plus the router's numberOffset. When the names answer
// has prefixes, the router prefix comes first (A.12). reply is a GET /names answer, merged or not.
export function numText(reply) {
  const off = reply?.numberOffset || 0;
  const pl = reply?.prefixes;
  if (!pl) return (n) => String(n + off);
  return (n) => {
    const p = pl[placeOf(n)] || {};
    const num = String(routerN(n) + (p.numberOffset || 0));
    return p.prefix ? `${p.prefix}.${num}` : num;
  };
}

// What a typed search matches: { place, text }, with place null when the text names no router. A prefix that no
// router has gives place -1, which matches nothing.
export function typedPlace(reply, text) {
  const pl = reply?.prefixes;
  if (!pl) return { place: null, text };
  const list = pl.map((p) => p.prefix);
  const prefix = typedPrefix(text, list);
  if (!prefix) return { place: null, text };
  return { place: list.indexOf(prefix), text: prefixOf(text).text };
}
