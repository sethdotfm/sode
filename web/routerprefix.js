// A typed source or destination can name its router by prefix (A.23). No imports, so Node tests load it.
// Only a prefix and digits count, so a name such as "A.B Mixer" stays a name.
const PREFIXED = /^\s*([a-z])\.(\d+)\s*$/i;

const split = (q) => {
  const m = PREFIXED.exec(q);
  return m ? { prefix: m[1].toUpperCase(), text: m[2] } : { prefix: '', text: q };
};

// prefixOf splits q into the router prefix it starts with ('' for none) and the text after it.
export const prefixOf = (q) => split(q);

// searchText is what to search router `own`'s list with, or null when the field cannot match it. others are the
// texts of the route's other fields: the first prefix of a known router there pins the whole route to that router.
// routers lists the prefixes this page knows.
export function searchText(q, own, others, routers) {
  const { prefix, text } = split(q);
  if (prefix && !routers.includes(prefix)) return null;
  const route = others.map((o) => split(o).prefix).find((p) => routers.includes(p));
  if (route && prefix && route !== prefix) return null;
  const pinned = prefix || route;
  return pinned && pinned !== own ? null : text;
}
