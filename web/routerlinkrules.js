// The Router link status for each router (connview.js, mobile.js, statusinfo.js, degradedview.js). Pure, no DOM. The
// rules for more than one router are in docs/agents/web-layout.md (Server and Router status).

// connState's router state (core.js) is the first entry's.
const STATE = { up: 'ok', connecting: 'warn', down: 'error' };
const WORD = { up: 'connected', connecting: 'connecting', down: 'down' };

// One entry per router of s.routers, in config order; before the hello, one entry for the first router. of(uid) gives
// the router's { uid, prefix, title, link, addr, err, never }: routerData in core.js, where blank is the first router.
// A router after the first with no link address yet shows its address from the settings. link is the server's own
// word (up, connecting, down); state is a setDot state.
export function routerLinks(s, of) {
  const first = of('');
  const list = [routerLink(s, first.uid, first.prefix, first.title, first.addr, first.link, first.err, first.never)];
  for (const r of (s.routers || []).slice(1)) {
    const d = of(r.uid);
    list.push(routerLink(s, r.uid, r.prefix, r.title, d.addr || r.address, d.link, d.err, d.never));
  }
  return list;
}

// never: the server says that the link was not up one time since the start or the last change of the address.
export function routerLink(s, uid, prefix, title, addr, link, err, never = false) {
  const known = s.wsState === 'up' && !!link;
  const base = { uid: uid || '', prefix: prefix || '', title: title || '', addr: addr || '' };
  // A link that connects again after a failure is lost: its icon is red, as a down one is (docs/color-roles.md).
  const isLost = known && (link === 'down' || (link === 'connecting' && !!err));
  if (isLost && never) return { ...base, link, state: 'error', word: 'never connected', err: err || '', never: true };
  if (isLost) return { ...base, link, state: 'error', word: 'down', err: err || '', never: false };
  return {
    ...base,
    link: known ? link : '',
    state: known ? STATE[link] || 'idle' : 'idle',
    word: known ? WORD[link] || link : 'unknown',
    err: link !== 'up' ? err || '' : '',
    never: false,
  };
}

// A lost link in one set of words on every page. The Lost link banner's title uses them (lostTitle). So do the panel
// page's Offline cover (panelrules.js offlineOf) and the editor's line (editorrules.js signInNote).
export const LINK_WORDS = { server: 'The Server link is down.', router: 'The Router link is down.', every: 'Every Router link is down.' };

// The Lost link banner's title from a LINK_WORDS sentence: "Lost link: the Server link is down".
export const lostTitle = (s) => `Lost link: ${s[0].toLowerCase()}${s.slice(1).replace(/\.$/, '')}`;

// Compact view hides the status only while this is true.
export const allUp = (list) => list.length > 0 && list.every((r) => r.state === 'ok');

// lost: the link of r is down, or connecting again after a failure. A router that connects for the first time is not
// lost. With no server link nothing is known, so nothing is lost.
export const lost = (r) => r.link === 'down' || (r.link === 'connecting' && !!r.err);

// The one Routers icon of the status bar, at every router count. state: no color while every router is up (dark
// cockpit). Red while every router is lost. Amber while one or more are lost and one or more are not. A router that
// connects for the first time is amber too, as the Connecting of one router is. idle: no Server link, so nothing is
// known. word: the text beside the icon. It counts the routers that are down, never the ones that are up, so a number
// in color is always the number of faults. aria: the spoken name of the icon. down: how many routers are lost.
export function barMark(list) {
  const n = list.length;
  const down = list.filter(lost).length;
  let state = 'warn';
  if (list.some((r) => r.state === 'idle')) state = 'idle';
  else if (down && down === n) state = 'error';
  else if (allUp(list)) state = 'ok';
  if (n === 1) return { state, word: 'Router', aria: `Router ${list[0].word}`, down };
  if (down) return { state, word: `${down} of ${n} down`, aria: `Routers: ${down} of ${n} down`, down };
  const aria = { ok: `Routers: all ${n} connected`, warn: 'Routers connecting', idle: 'Routers unknown' }[state];
  return { state, word: 'Routers', aria, down };
}

// The state of Sode as the page sees it (GLOSSARY.md: Healthy, Degraded, Down). down: every router is lost, so the
// Lost link banner shows. degraded: one or more are lost and one or more are not. The page then names them and their
// destinations, with no banner. healthy: every link is up. '' while a router connects for the first time.
export function wholeState(list) {
  const n = list.filter(lost).length;
  if (n && n === list.length) return 'down';
  if (n) return 'degraded';
  return allUp(list) ? 'healthy' : '';
}

// The router list of the status bar (statusinfo.js) and its rows.

// The router list shows a find box from this many routers.
export const FIND_FROM = 8;

// The name of a router in the router list and the hover box: routerName. With one router the prefix stays hidden.
export function listName(r, n) {
  if (n === 1) return r.title || 'Router';
  return routerName(r);
}

// The state of one row in words. state is the color of the word: none for Connected (dark cockpit). note: what to do.
export function rowState(r) {
  if (r.never) return { word: 'Never connected', state: 'error', note: 'Check the address.' };
  if (r.state === 'error') return { word: 'Down', state: 'error', note: '' };
  if (r.state === 'warn') return { word: 'Connecting', state: 'warn', note: '' };
  if (r.state === 'ok') return { word: 'Connected', state: '', note: '' };
  return { word: 'Not known', state: 'idle', note: '' };
}

const ORDER = { error: 0, warn: 1, idle: 2, ok: 3 };

// The rows of the router list: lost routers first, then the ones that connect, then the ones that are up. Config
// order stays inside each group. q: the text of the find box, which matches the prefix, the title or the address.
export function listRows(list, q = '') {
  const want = q.trim().toLowerCase();
  const n = list.length;
  return list
    .map((r, i) => ({ r, i, name: listName(r, n), ...rowState(r) }))
    .filter((x) => !want || [x.name, x.r.prefix, x.r.title, x.r.addr].some((t) => t.toLowerCase().includes(want)))
    .sort((a, b) => ORDER[a.r.state] - ORDER[b.r.state] || a.i - b.i);
}

// Up to this many destination names show in the Degraded note. More show as a range.
export const DEGRADED_NAMES_MAX = 8;

// Up to this many lost routers get one line each in the Degraded note. More get one sentence that names them.
export const DEGRADED_LINES_MAX = 2;

// A router's one name on every page (the Lost link banner, the hover box, the router list, /log): "Router B", then the
// Router title. A title that starts with "Router B" is the name as it is, so the default title "Router B" and "Router B
// (studio 2)" are not said twice.
export const routerName = (r) => {
  const head = ['Router', r.prefix].filter(Boolean).join(' ');
  const t = (r.title || '').trim();
  if (!t || t === head || t.startsWith(`${head} `)) return t || head;
  return `${head} ${t}`;
};

// The Degraded note's line for one lost router. dsts: its destinations as the page shows them ({ label }), in number
// order. A router that never connected gets the hint to check its address.
export function degradedLine(r, dsts) {
  const name = routerName(r);
  const head = r.never ? `${name} never connected. Check the address.` : `${name} is down.`;
  if (!dsts.length) return `${head} You cannot patch its destinations now.`;
  if (dsts.length === 1) return `${head} You cannot patch ${dsts[0].label} now.`;
  const which = dsts.length <= DEGRADED_NAMES_MAX ? dsts.map((d) => d.label).join(', ') : `${dsts[0].label} to ${dsts[dsts.length - 1].label}`;
  return `${head} You cannot patch its ${dsts.length} destinations now: ${which}.`;
}

// The lines of the Degraded note. With one or two lost routers, one line each (degradedLine). With more, one sentence
// that names their prefixes, and more is true: the note then has a Show routers button. dstsOf(r) gives the
// destinations of r.
export function degradedNote(lostList, dstsOf) {
  if (lostList.length <= DEGRADED_LINES_MAX) return { lines: lostList.map((r) => degradedLine(r, dstsOf(r))), more: false };
  const prefixes = lostList.map((r) => r.prefix).join(', ');
  return { lines: [`${lostList.length} routers are down: ${prefixes}. You cannot patch their destinations now.`], more: true };
}
