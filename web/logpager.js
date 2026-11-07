// How /log reads the sent log back, newest first. Filters hide entries, so it reads on until enough are shown (want)
// or the log or page cap runs out; else a page of only hidden keepalives looked stuck. Pure, no imports.
// Methods return the request { before, n } to send, or null.

export const PAGE = 100;
// Per request while reading on to find more to show, or back to a time
// range (the server's most).
export const FILL = 500;
export const AUTO_PAGES = 20;

export function newPager() {
  let list = []; // loaded entries, newest first
  const seen = new Set(); // their seqs
  let more = false; // the server has older ones
  let total = 0; // entries in the log, as last reported
  let pending = null; // the `before` of the request awaiting its reply
  let started = false; // asked for the newest page on this connection
  let want = 0; // shown rows to read on until
  let pages = 0; // requests since the last open or Load older

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
    get list() { return list; },
    get more() { return more; },
    get total() { return total; },
    get started() { return started; },
    get loading() { return pending !== null; },
    get oldest() { return oldest(); },

    open() {
      list = [];
      seen.clear();
      more = false;
      started = true;
      want = PAGE;
      pages = 0;
      return ask(-1, PAGE);
    },

    older(shownNow) {
      if (!started || pending !== null || !list.length) return null;
      want = shownNow + PAGE;
      pages = 0;
      return ask(oldest().seq, PAGE);
    },

    // null when not the awaited reply (replaced connection, duplicate); else { fresh, newest }.
    receive(m) {
      if (!started || pending === null || m.before !== pending) return null;
      const newest = m.before === -1;
      pending = null;
      const fresh = add(m.entries);
      // Newest: live entries may have come in first. Older: all before the
      // oldest, which the list ends with.
      list = newest ? [...list, ...fresh].sort(byNewest) : [...list, ...fresh];
      more = m.more;
      if (m.total != null) total = m.total;
      return { fresh, newest };
    },

    live(m) {
      const fresh = add(m.entries);
      list.unshift(...fresh);
      if (m.total != null) total = m.total;
      return fresh;
    },

    next(shownNow) {
      return idle() && shownNow < want && pages < AUTO_PAGES ? ask(oldest().seq, FILL) : null;
    },

    back() {
      return idle() ? ask(oldest().seq, FILL) : null;
    },

    unsent() { pending = null; },

    close() {
      started = false;
      pending = null;
    },
  };
}
