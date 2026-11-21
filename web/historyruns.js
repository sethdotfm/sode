// Runs of History rows that show as one: stacked changes to one destination, and folded Protect actions
// (docs/agents/history.md). Pure, no imports, so Node tests it.

// rows: newest first, each with shows (false: the filters hide it). A run is rows of one lane with the same key. Each
// is at most gapMs before the one above it in that lane. Rows of other lanes between do not end it. A row of the lane
// that does not join ends the run, and so does one that does not show or has key ''. span(r) is [newest, oldest] in
// ms. join(run) makes one row of a run of two or more, put where its newest row is. gapMs 0 is off.
export function mergeRuns(rows, { lane = () => 0, key, gapMs, span, join }) {
  if (!(gapMs > 0)) return rows;
  const out = [];
  const open = new Map(); // lane → { key, rows, oldest }
  for (const r of rows) {
    const l = lane(r);
    const k = r.shows ? key(r) : '';
    const [newest, oldest] = span(r);
    const run = open.get(l);
    if (k && run && run.key === k && run.oldest - newest <= gapMs) {
      run.rows.push(r);
      run.oldest = Math.min(run.oldest, oldest);
      continue;
    }
    open.delete(l);
    const slot = [r];
    out.push(slot);
    if (k) open.set(l, { key: k, rows: slot, oldest });
  }
  return out.map((s) => (s.length > 1 ? join(s) : s[0]));
}
