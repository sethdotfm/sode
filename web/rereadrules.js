// What a re-read of the router names changed. Pure, so Node tests load it. rereadnames.js shows the result.

const byNum = (list) => new Map(list.map((it) => [it.n, it.name]));

// before and after: { srcs, dsts } as [{ n, name }]. A changed entry is a new number, a removed number or a new name.
export function namesChange(before, after) {
  const count = (a, b) => {
    const old = byNum(a);
    const now = byNum(b);
    let n = 0;
    for (const [k, v] of now) if (old.get(k) !== v) n++;
    for (const k of old.keys()) if (!now.has(k)) n++;
    return n;
  };
  return { srcs: count(before.srcs, after.srcs), dsts: count(before.dsts, after.dsts) };
}

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

// The banner text for a re-read that the router answered.
export function changeText({ srcs, dsts }, total) {
  const all = `${plural(total.srcs, 'source', 'sources')} and ${plural(total.dsts, 'destination', 'destinations')}`;
  if (!srcs && !dsts) return `No names changed. The router has ${all}.`;
  const parts = [srcs && plural(srcs, 'source', 'sources'), dsts && plural(dsts, 'destination', 'destinations')].filter(Boolean);
  return `${parts.join(' and ')} changed. The router has ${all}.`;
}
