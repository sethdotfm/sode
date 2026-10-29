// How the /log page reads the sent log back (issue 141). Pure, no imports,
// so Node tests it. The page asks for pages of entries newest first; the
// filters hide some, so it keeps reading until enough are *shown* (want)
// or the log, or the page cap, runs out. Without that, an open or a Load
// older whose pages hold only hidden entries (keepalives under the
// default "Connects" filter) showed nothing, and looked stuck.

// Entries per request on open and per Load older.
export const PAGE = 100;
// Per request while reading on to find more to show, or back to a time
// range (the server's most).
export const FILL = 500;
// Requests for one open or Load older while the filters hide everything.
export const AUTO_PAGES = 20;

export function newPager() {
  return {
    list: [], // loaded entries, newest first
    seen: new Set(), // their seqs
    more: false, // the server has older ones
    total: 0,
    pending: null, // the `before` of the request awaiting its reply
    started: false, // asked for the newest page on this connection
    want: 0, // shown rows to read on until
    pages: 0, // requests since the last open or Load older
  };
}

export const loading = (p) => p.pending !== null;
export const oldest = (p) => p.list[p.list.length - 1];

// The entries not loaded yet, stamped with their time as ms (t).
export function add(p, entries) {
  const fresh = entries.filter((e) => !p.seen.has(e.seq));
  for (const e of fresh) {
    p.seen.add(e.seq);
    e.t = Date.parse(e.time);
  }
  return fresh;
}

function ask(p, before, n) {
  p.pending = before;
  p.pages++;
  return { before, n };
}

// A fresh connection: forget everything and ask for the newest page.
// Returns the request to send.
export function open(p) {
  p.list = [];
  p.seen.clear();
  p.more = false;
  p.started = true;
  p.want = PAGE;
  p.pages = 0;
  p.pending = null;
  return ask(p, -1, PAGE);
}

// Load older: PAGE more shown rows than now. null if busy or nothing loaded.
export function older(p, shownNow) {
  if (!p.started || loading(p) || !p.list.length) return null;
  p.want = shownNow + PAGE;
  p.pages = 0;
  return ask(p, oldest(p).seq, PAGE);
}

// A reply. Returns null when it isn't the one awaited (a connection since
// replaced, or a duplicate) and leaves the request pending. Else
// { fresh, newest }: the entries it added.
export function receive(p, m) {
  if (!p.started || p.pending === null || m.before !== p.pending) return null;
  const newest = m.before === -1;
  p.pending = null;
  const fresh = add(p, m.entries);
  // Newest: live entries may have come in first. Older: all before the
  // oldest, which the list ends with.
  p.list = newest ? [...p.list, ...fresh].sort((a, b) => b.seq - a.seq) : [...p.list, ...fresh.sort((a, b) => b.seq - a.seq)];
  p.more = m.more;
  if (m.total != null) p.total = m.total;
  return { fresh, newest };
}

// Live entries (sentAdd), newest first; those already loaded are dropped.
export function live(p, entries) {
  const fresh = add(p, entries).sort((a, b) => b.seq - a.seq);
  p.list.unshift(...fresh);
  return fresh;
}

// What to ask next after a reply: more of the log while the filters leave
// fewer than `want` shown (within the page cap), or while a time range
// isn't covered (rangeNeeds). null: nothing.
export function next(p, shownNow, rangeNeeds) {
  if (!p.started || loading(p) || !p.more || !p.list.length) return null;
  if (rangeNeeds || (shownNow < p.want && p.pages < AUTO_PAGES)) return ask(p, oldest(p).seq, FILL);
  return null;
}

// A time range changed: read on only if it needs more.
export function fill(p, rangeNeeds) {
  return rangeNeeds ? next(p, Infinity, true) : null;
}

// The connection dropped.
export function close(p) {
  p.started = false;
  p.pending = null;
}
