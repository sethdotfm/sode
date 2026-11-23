// A typed source or destination can name its router by prefix: one or two letters, an optional dot, then digits (A.23,
// A23, BA.23, BA23). No imports, so Node tests load it. Only a prefix and digits count, so a name such as "A.B Mixer"
// stays a name.
const PREFIXED = /^\s*([a-z]{1,2})(\.?)(\d+)\s*$/i;

const split = (q) => {
  const m = PREFIXED.exec(q);
  return m ? { prefix: m[1].toUpperCase(), text: m[3], dot: !!m[2] } : { prefix: '', text: q, dot: false };
};

// prefixOf splits q into the router prefix it starts with ('' for none), the text after it, and dot (the prefix has
// its dot). It does not check that a router has the prefix: see typedPrefix.
export const prefixOf = (q) => split(q);

// typedPrefix is the router prefix that q names, or '' for none. routers lists the prefixes this page knows. With one
// router the prefix is hidden, so only the dotted form names it, and "A11" stays a name. Without the dot, letters that
// no router has are a name ("TX1"); with the dot, they are a prefix that matches nothing.
export function typedPrefix(q, routers) {
  const { prefix, dot } = split(q);
  if (!prefix) return '';
  if (dot) return prefix;
  return routers.length > 1 && routers.includes(prefix) ? prefix : '';
}

// searchText is what to search router `own`'s list with, or null when the field cannot match it. others are the
// texts of the route's other fields: the first prefix of a known router there pins the whole route to that router.
// routers lists the prefixes this page knows.
export function searchText(q, own, others, routers) {
  const prefix = typedPrefix(q, routers);
  const text = prefix ? split(q).text : q;
  if (prefix && !routers.includes(prefix)) return null;
  const route = others.map((o) => typedPrefix(o, routers)).find((p) => routers.includes(p));
  if (route && prefix && route !== prefix) return null;
  const pinned = prefix || route;
  return pinned && pinned !== own ? null : text;
}
