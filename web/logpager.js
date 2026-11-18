// How /log reads the Communication Log back, newest first. Filters hide entries, so it reads on until enough are
// shown (want) or the log or page cap runs out. Else a page of only hidden keepalives looked stuck. Pure, no imports.
// Methods return the request { before, n } to send, or null.

export const PAGE = 100;
// Per request while reading on to find more to show, or back to a time
// range (the server's most).
const FILL = 500;
export const AUTO_PAGES = 20;
// Live rows kept on the page. Older rows that Load older fetched raise the cap by their count, so a
// live row does not drop them. Rows past the cap leave the page, never the log file.
export const LIVE_CAP = 500;

export function newPager() {
  let list = []; // loaded entries, newest first.
  const seen = new Set(); // their seqs.
  let more = false; // the server has older ones.
  let total = 0; // entries in the log, as last reported.
  let pending = null; // the `before` of the request that waits for its reply.
  let started = false; // asked for the newest page on this connection.
  let want = 0; // shown rows to read on until.
  let pages = 0; // requests since the last open or Load older
  let cap = LIVE_CAP;
  let raise = false; // older rows the user asked for (Load older, a time range) raise the cap; the read-on at open does not

  const oldest = () => list[list.length - 1];
  const byNewest = (a, b) => b.seq - a.seq;

  function add(entries) {
    const fresh = entries.filter((e) => !seen.has(e.seq));
    for (const e of fresh) {
      seen.add(e.seq);
      e.t = Date.parse(e.time);
    }
    return fresh.sort(byNewest);
  }

  function ask(before, n) {
    pending = before;
    pages++;
    return { before, n };
  }

  const idle = () => started && pending === null && more && list.length > 0;

  return {
    get list() {
      return list;
    },
    get more() {
      return more;
    },
    get total() {
      return total;
    },
    get started() {
      return started;
    },
    get loading() {
      return pending !== null;
    },
    get oldest() {
      return oldest();
    },

    open() {
      list = [];
      seen.clear();
      more = false;
      started = true;
      want = PAGE;
      pages = 0;
      cap = LIVE_CAP;
      raise = false;
      return ask(-1, PAGE);
    },

    older(shownNow) {
      if (!started || pending !== null || !list.length) return null;
      want = shownNow + PAGE;
      raise = true;
      pages = 0;
      return ask(oldest().seq, PAGE);
    },

    // null when not the awaited reply (replaced connection, duplicate); else { fresh, newest }.
    receive(m) {
      if (!started || pending === null || m.before !== pending) return null;
      const newest = m.before === -1;
      pending = null;
      const fresh = add(m.entries);
      // Newest: it is possible that live entries came in first. Older: all before the
      // oldest, which the list ends with.
      list = newest ? [...list, ...fresh].sort(byNewest) : [...list, ...fresh];
      if (!newest && raise) cap += fresh.length;
      more = m.more;
      if (m.total != null) total = m.total;
      return { fresh, newest };
    },

    // { fresh, dropped }: dropped are the oldest entries that left the list for the cap.
    live(m) {
      const fresh = add(m.entries);
      list.unshift(...fresh);
      if (m.total != null) total = m.total;
      const dropped = list.length > cap ? list.splice(cap) : [];
      for (const e of dropped) seen.delete(e.seq);
      if (dropped.length) more = true;
      return { fresh, dropped };
    },

    next(shownNow) {
      return idle() && shownNow < want && pages < AUTO_PAGES ? ask(oldest().seq, FILL) : null;
    },

    back() {
      raise = true;
      return idle() ? ask(oldest().seq, FILL) : null;
    },

    unsent() {
      pending = null;
    },

    close() {
      started = false;
      pending = null;
    },
  };
}
