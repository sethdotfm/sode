// The Router link status for each router (connview.js, mobile.js, statusinfo.js). Pure, no DOM. The rules for more
// than one router are in docs/agents/web-layout.md (Server and Router status).

// Up to this many routers get one icon each; more share one icon with a count.
export const ROUTER_ICONS_MAX = 3;

// Keep in step with core.js connState's router state; test/routerlink.test.mjs compares them.
const STATE = { up: 'ok', connecting: 'warn', down: 'error' };
const WORD = { up: 'connected', connecting: 'connecting', down: 'down' };
const RANK = { error: 3, warn: 2, idle: 1, ok: 0 };

// One entry per router, in config order. The hello names one router today (routerUid, prefix and the scalar router
// fields), so the list holds one entry. link is the server's own word (up, connecting, down); state is a setDot state.
export function routerLinks(s) {
  return [routerLink(s, s.routerUid, s.prefix, s.cfg?.routerName, s.routerAddr, s.router, s.routerLastErr)];
}

export function routerLink(s, uid, prefix, title, addr, link, err) {
  const known = s.wsState === 'up' && !!link;
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
