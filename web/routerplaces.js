// The item numbers of a page with a WebSocket (the main page and the panel editor): places from the hello's routers
// list (routerids.js). One places object stays while the list stays, so item numbers do not change under a page.
import { S } from './core.js';
import { makePlaces, rightsIn } from './routerids.js';

let from = null;
let places = makePlaces();
export function pagePlaces() {
  if (from !== S.routers) {
    from = S.routers;
    places = makePlaces(S.routers);
  }
  return places;
}

// rightsIn, made again only when the Rights or the places change.
let rightsFrom = null;
let rightsFor = null;
let rightsOut = null;
export function itemRights(rights, p = pagePlaces()) {
  if (rights !== rightsFrom || p !== rightsFor) {
    rightsFrom = rights;
    rightsFor = p;
    rightsOut = rightsIn(rights, p);
  }
  return rightsOut;
}
