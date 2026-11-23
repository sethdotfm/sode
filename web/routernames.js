// GET /names of every router, as one answer on item numbers (routerids.js), for the panel editor. With two or more
// routers a read of names needs the router parameter (docs/api.md, Routers), so each router is read on its own.
import { makePlaces, mergeNames } from './routerids.js';

// get(path) answers the parsed JSON, or null when the read was refused. was is the places of an earlier read. It stays
// while the router list is the same, so item numbers do not change under the page. Answers { places, first, names },
// first being the first router's own answer, or null when a read was refused.
export async function readRouterNames(get, was = null) {
  const routers = (await get('api/v1/routers'))?.routers || [];
  const same = was && was.routers.length === routers.length && was.routers.every((r, i) => r.uid === routers[i].uid && r.prefix === routers[i].prefix);
  const places = same ? was : makePlaces(routers);
  if (!places.many) {
    const first = await get('api/v1/names');
    return first && { places, first, names: first };
  }
  const q = (uid) => `api/v1/names?router=${encodeURIComponent(uid)}`;
  const all = await Promise.all(routers.map((r) => get(q(r.uid))));
  if (all.some((x) => !x)) return null;
  const more = all.slice(1).map((reply, i) => ({ place: i + 1, reply }));
  return { places, first: all[0], names: mergeNames(all[0], more, places) };
}
