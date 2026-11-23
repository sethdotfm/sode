// The Router link status for each router (connview.js, mobile.js, statusinfo.js). Pure, no DOM. The rules for more
// than one router are in docs/agents/web-layout.md (Server and Router status).

// Up to this many routers get one icon each; more share one icon with a count.
export const ROUTER_ICONS_MAX = 3;

// connState's router state (core.js) is the first entry's.
const STATE = { up: 'ok', connecting: 'warn', down: 'error' };
const WORD = { up: 'connected', connecting: 'connecting', down: 'down' };
const RANK = { error: 3, warn: 2, idle: 1, ok: 0 };

// One entry per router, in config order. The first router comes from the scalar fields (routerUid, prefix, router,
// routerAddr, routerLastErr). Each router after it in s.routers comes from of(uid), { link, addr, err }: routerData
// in core.js. link is the server's own word (up, connecting, down); state is a setDot state.
export function routerLinks(s, of = null) {
  const first = routerLink(s, s.routerUid, s.prefix, s.cfg?.routerName, s.routerAddr, s.router, s.routerLastErr);
  if (!of || !(s.routers?.length > 1)) return [first];
  const more = s.routers.slice(1).map((r) => {
    const d = of(r.uid) || {};
    return routerLink(s, r.uid, r.prefix, r.title, d.addr || r.address, d.link, d.err);
  });
  return [first, ...more];
}

export function routerLink(s, uid, prefix, title, addr, link, err) {
  const known = s.wsState === 'up' && !!link;
  // A link that connects again after a failure is lost: its icon is red, as a down one is (docs/color-roles.md).
  if (known && link === 'connecting' && err)
    return { uid: uid || '', prefix: prefix || '', title: title || '', addr: addr || '', link, state: 'error', word: 'down', err };
  return {
    uid: uid || '',
    prefix: prefix || '',
    title: title || '',
    addr: addr || '',
    link: known ? link : '',
    state: known ? STATE[link] || 'idle' : 'idle',
    word: known ? WORD[link] || link : 'unknown',
    err: link !== 'up' ? err || '' : '',
  };
}

// Compact view hides the status only while this is true.
export const allUp = (list) => list.length > 0 && list.every((r) => r.state === 'ok');

const worst = (list) => list.reduce((w, r) => (RANK[r.state] > RANK[w] ? r.state : w), 'ok');

// One mark per Router icon in the status. label: the word beside the icon; aria: the icon's spoken name;
// count: text inside the icon ('' for none). The prefix stays hidden while there is one router.
export function routerMarks(list, max = ROUTER_ICONS_MAX) {
  if (list.length <= max) {
    return list.map((r) => {
      const label = list.length === 1 ? 'Router' : `Router ${r.prefix}`;
      return { state: r.state, label, aria: `${label} ${r.word}`, count: '' };
    });
  }
  const up = list.filter((r) => r.state === 'ok').length;
  return [{ state: worst(list), label: 'Routers', aria: `${up} of ${list.length} routers connected`, count: `${up}/${list.length}` }];
}

// The status details' label for one router's link: the prefix and title follow only when there is more than one.
export const linkLabel = (r, n) => (n === 1 ? 'Router link' : ['Router link', r.prefix, r.title].filter(Boolean).join(' '));

// lost: the link of r is down, or connecting again after a failure. A router that connects for the first time is not
// lost. With no server link nothing is known, so nothing is lost.
export const lost = (r) => r.link === 'down' || (r.link === 'connecting' && !!r.err);

// The state of Sode as the page sees it (GLOSSARY.md: Healthy, Degraded, Down). down: every router is lost, so the
// Lost link banner shows. degraded: one or more are lost and one or more are not. The page then names them and their
// destinations, with no banner. healthy: every link is up. '' while a router connects for the first time.
export function wholeState(list) {
  const n = list.filter(lost).length;
  if (n && n === list.length) return 'down';
  if (n) return 'degraded';
  return allUp(list) ? 'healthy' : '';
}

// Up to this many destination names show in the Degraded note. More show as a range.
export const DEGRADED_NAMES_MAX = 8;

// A router's full name: Router, its prefix and its title. A title that only repeats the prefix ("Router B") is not
// said twice.
export const routerName = (r) => ['Router', r.prefix, r.title !== `Router ${r.prefix}` && r.title].filter(Boolean).join(' ');

// The Degraded note's line for one lost router. dsts: its destinations as the page shows them ({ label }), in number
// order.
export function degradedLine(r, dsts) {
  const name = routerName(r);
  if (!dsts.length) return `${name} is down. You cannot patch its destinations now.`;
  if (dsts.length === 1) return `${name} is down. You cannot patch ${dsts[0].label} now.`;
  const which = dsts.length <= DEGRADED_NAMES_MAX ? dsts.map((d) => d.label).join(', ') : `${dsts[0].label} to ${dsts[dsts.length - 1].label}`;
  return `${name} is down. You cannot patch its ${dsts.length} destinations now: ${which}.`;
}
