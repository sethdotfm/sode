// The artwork on a Grid panel's names. Pure. An item shows its own artwork, else its Default artwork (panellook.js
// withDefault). Hide text works as on a cell: the item's own true or false, else its default's.
import { artOf, artBOf, withDefault, textHidden } from './panellook.js';
import { routerN } from './routerids.js';

// kind is 'source' or 'destination', n an item number (routerids.js), edge the panel's edge and defaults a defaultsBy
// Map. routerOf(n) is the UID of the item's router; blank is the first router.
// { a, b, hide? } is Icon A as artOf gives it (or null) and Icon B (or ''). hide is true when the name's text is
// hidden. null when the name has neither icon.
export function nameArt(edge, kind, n, defaults, routerOf = () => '') {
  const own = edge?.items?.[n] || {};
  const router = routerOf(n);
  const drawn = withDefault({ kind, n: routerN(n), ...(router ? { router } : {}), art: own.art, artB: own.artB, hideText: own.hideText }, defaults);
  const a = artOf(drawn);
  const b = artBOf(drawn);
  if (!a && !b) return null;
  return textHidden(drawn) ? { a, b, hide: true } : { a, b };
}

// Equal for equal artwork, so a paint changes the node only when it must.
export const nameArtKey = (art) => (art ? `${art.a ? (art.a.icon ? `i:${art.a.icon}` : art.a.id) : ''}|${art.b}|${art.hide ? 'h' : ''}` : '');
