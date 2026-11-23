// Keeps GET /api/v1/tally current with the WebSocket's frames; a full read is needed only after a reconnect. A frame's rows are [dst, UI level, src].

// A new answer with each row of u in it; t is the shared object from withEtag (etagfetch.js), so it stays as it was.
export function withRoutes(t, u) {
  const routes = t.routes.slice();
  const at = new Map(routes.map((r, i) => [`${r.dst} ${r.level}`, i]));
  for (const [dst, level, src] of u) {
    const k = `${dst} ${level}`;
    // A row with the same source keeps its Via. A new source waits for the read that a `viaRows` message asks for.
    const was = at.has(k) ? routes[at.get(k)] : null;
    const row = was?.via && was.src === src ? was : { dst, level, src };
    if (at.has(k)) routes[at.get(k)] = row;
    else at.set(k, routes.push(row) - 1);
  }
  return { ...t, routes };
}

// A Hidden caller's tally leaves out a route whose source it cannot see (docs/api.md, Visibility); the socket does not.
// For such a caller a frame only asks for a full read.
export const framesApply = (rights) => rights?.visibility !== 'hidden';

// The slow check of a tally kept from frames.
export function sameRoutes(a, b) {
  if (a.routes.length !== b.routes.length) return false;
  const at = new Map(a.routes.map((r) => [`${r.dst} ${r.level}`, r.src]));
  return b.routes.every((r) => at.get(`${r.dst} ${r.level}`) === r.src);
}

// While the WebSocket is up its frames carry the tally, which is read only this often, as a check.
export const CHECK_MS = 60000;

// The feed, replaced whole on each event (panelrun.js holds it). live: a hello came since the socket last closed, so
// frames hold every change since. trusted: no check found the kept tally wrong since that hello. shown: the last read
// with every frame since put in. stale: frames from before a hello can have gaps, so the next poll reads. late: frames
// that came while a read was out, put into its answer (null with no read out). rows: frames waiting for their draw.
export const newFeed = (live, rights) => ({ live, trusted: true, rights, shown: null, stale: false, hellos: 0, late: null, rows: [], checkedAt: 0 });

export const wantsRead = (f, now) => !f.live || !f.trusted || !f.shown || f.stale || now - f.checkedAt >= CHECK_MS || !framesApply(f.rights);

export const readStarts = (f, read) => ({ ...f, late: read ? [] : null });

// hellos: f.hellos when the read went out; a hello since then keeps the feed stale.
export function readCame(f, tally, hellos, now) {
  const read = f.late?.length ? withRoutes(tally, f.late) : tally;
  const kept = f.shown && f.rows.length ? withRoutes(f.shown, f.rows) : f.shown;
  const wrong = f.live && kept && !f.stale && f.trusted && !sameRoutes(kept, read);
  return { ...f, shown: read, trusted: f.trusted && !wrong, checkedAt: now, stale: hellos === f.hellos ? false : f.stale };
}

export const readEnds = (f) => ({ ...f, late: null });

// act: 'poll' asks for a read, 'queue' draws the rows soon, '' does nothing.
export function framesCame(f, u) {
  const g = f.late ? { ...f, late: [...f.late, ...u] } : f;
  if (!framesApply(f.rights)) return { feed: g, act: 'poll' };
  if (!f.live || !f.trusted) return { feed: g, act: '' };
  if (!f.shown) return { feed: g, act: f.late ? '' : 'poll' };
  return { feed: { ...g, rows: [...f.rows, ...u] }, act: 'queue' };
}

// The queued rows go into shown; changed is false when there was nothing to draw.
export function drawRows(f) {
  if (!f.shown || !f.rows.length) return { feed: { ...f, rows: [] }, changed: false };
  return { feed: { ...f, shown: withRoutes(f.shown, f.rows), rows: [] }, changed: true };
}

// The Via of some row changed (the page's `viaRows` message). Frames carry no Via, so the next poll reads.
export const viaCame = (f) => ({ ...f, stale: true });

export const helloCame = (f) => ({ ...f, live: true, trusted: true, stale: true, hellos: f.hellos + 1 });

export const socketClosed = (f) => ({ ...f, live: false });

// A tally read under Hidden leaves routes out, so frames on it after the Rights widen would never bring them back.
export const rightsCame = (f, r) => ({ ...f, rights: r, stale: f.stale || framesApply(r) !== framesApply(f.rights) });
