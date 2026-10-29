// How the /log page reads the sent log back (issue 141). Pure, no imports,
// so Node tests it. The page asks for pages of entries newest first; the
// filters hide some, so it keeps reading until enough are *shown* (want)
// or the log, or the page cap, runs out. Without that, an open or a Load
// older whose pages hold only hidden entries (keepalives under the
// default "Connects" filter) showed nothing, and looked stuck.
//
// Each method that asks for something returns the request to send,
// { before, n }, or null.

// Entries per request on open and per Load older.
export const PAGE = 100;
// Per request while reading on to find more to show, or back to a time
// range (the server's most).
export const FILL = 500;
// Requests for one open or Load older while the filters hide everything.
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

  // The entries not loaded yet, stamped with their time as ms (t), newest first.
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

    // A fresh connection: forget everything and ask for the newest page.
    open() {
      list = [];
      seen.clear();
      more = false;
      started = true;
      want = PAGE;
      pages = 0;
      return ask(-1, PAGE);
    },

    // Load older: PAGE more shown rows than now. null if busy or nothing loaded.
    older(shownNow) {
      if (!started || pending !== null || !list.length) return null;
      want = shownNow + PAGE;
      pages = 0;
      return ask(oldest().seq, PAGE);
    },

    // A reply. null when it isn't the one awaited (a connection since
    // replaced, or a duplicate), leaving the request pending. Else
    // { fresh, newest }: the entries it added.
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

    // Live entries (sentAdd), newest first; those already loaded are dropped.
    live(m) {
      const fresh = add(m.entries);
      list.unshift(...fresh);
      if (m.total != null) total = m.total;
      return fresh;
    },

    // After a reply: more of the log while the filters leave fewer than
    // `want` of the loaded entries shown, within the page cap.
    next(shownNow) {
      return idle() && shownNow < want && pages < AUTO_PAGES ? ask(oldest().seq, FILL) : null;
    },

    // Read on to cover a time range, whatever is shown and however many pages.
    back() {
      return idle() ? ask(oldest().seq, FILL) : null;
    },

    // The request returned couldn't be sent: there is nothing to wait for.
    unsent() { pending = null; },

    // The connection dropped.
    close() {
      started = false;
      pending = null;
    },
  };
}
