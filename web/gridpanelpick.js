// Picks on a Grid panel's Source bar and Destination bar, and the Diagonal they make. Pure.
// Picks are router numbers; anchor is the last plain or Cmd click on each bar, where a Shift-click range starts.
// Every order here is display order (gridAxes), never router order.

export const createPicks = () => ({ srcs: [], dsts: [], anchor: { srcs: null, dsts: null } });

export const pickCount = (p) => p.srcs.length + p.dsts.length;

export const dropPicks = (p) => ({ ...p, srcs: [], dsts: [], anchor: { srcs: null, dsts: null } });

// kind is 'srcs' or 'dsts'. mods: { range: Shift, toggle: Cmd or Ctrl }. shown is that bar's shown names, since a
// range covers only what the person sees.
export function pickName(p, kind, n, { range = false, toggle = false } = {}, shown = []) {
  const had = p[kind];
  const from = shown.indexOf(p.anchor[kind]);
  const to = shown.indexOf(n);
  let next;
  let anchor = n;
  if (range && from >= 0 && to >= 0) {
    const span = shown.slice(Math.min(from, to), Math.max(from, to) + 1);
    next = toggle ? [...had, ...span.filter((x) => !had.includes(x))] : span;
    anchor = p.anchor[kind];
  } else if (toggle) next = had.includes(n) ? had.filter((x) => x !== n) : [...had, n];
  else next = had.length === 1 && had[0] === n ? [] : [n];
  return { ...p, [kind]: next, anchor: { ...p.anchor, [kind]: anchor } };
}

// list in display order, starting at the first item at or after at's place in order, wrapping.
export function fromHere(list, order, at) {
  const pos = new Map(order.map((x, i) => [x, i]));
  const sorted = list.filter((x) => pos.has(x)).sort((a, b) => pos.get(a) - pos.get(b));
  const here = pos.get(at) ?? 0;
  const i = sorted.findIndex((x) => pos.get(x) >= here);
  return i <= 0 ? sorted : [...sorted.slice(i), ...sorted.slice(0, i)];
}

// The routes a press on xp stages, as [{ src, dst }] in the order they are numbered. One per target destination,
// so no destination comes twice. axes: { srcs, dsts } in display order, shownDsts the destinations not filtered out
// (default dsts). Filtered-out picks still count.
export function diagonal(p, xp, { srcs, dsts, shownDsts = dsts }) {
  const pSrcs = fromHere(p.srcs, srcs, xp.src);
  const pDsts = fromHere(p.dsts, dsts, xp.dst);
  let targets;
  if (pDsts.length > 1) targets = pDsts;
  else {
    const i = shownDsts.indexOf(xp.dst);
    // Down from the crosspoint and no further than the last shown destination.
    targets = i < 0 ? [xp.dst] : shownDsts.slice(i, i + Math.max(1, pSrcs.length));
  }
  if (pSrcs.length < 2) return targets.map((dst) => ({ src: xp.src, dst }));
  return targets.map((dst, i) => ({ src: pSrcs[i % pSrcs.length], dst }));
}

// A Shift move on a bar. With no anchor shown, the range starts at from, the name the cursor left.
export function extendPick(p, kind, from, to, shown) {
  const q = shown.includes(p.anchor[kind]) ? p : { ...p, anchor: { ...p.anchor, [kind]: from } };
  return pickName(q, kind, to, { range: true }, shown);
}

// A mouse drag along a bar from name from to name to. base is the picks at the press, so a drag that comes back
// unpicks what it passed. add (Cmd or Ctrl) keeps base's picks on that bar; else the span replaces them.
export function dragPick(base, kind, from, to, add, shown) {
  return pickName({ ...base, anchor: { ...base.anchor, [kind]: from } }, kind, to, { range: true, toggle: add }, shown);
}

// A move from a bar (was) into the grid (next). With two or more names picked on that bar and the cursor on one of
// them, it goes to the first shown pick. Then a press numbers the range from its start, not its end.
export function enterGrid(was, next, p, axes) {
  const kind = was.j < 0 && was.i >= 0 && next.j >= 0 ? 'dsts' : was.i < 0 && was.j >= 0 && next.i >= 0 ? 'srcs' : null;
  if (!kind || p[kind].length < 2) return next;
  const k = kind === 'dsts' ? 'i' : 'j';
  if (!p[kind].includes(axes[kind][was[k]])) return next;
  const first = axes[kind].findIndex((n) => p[kind].includes(n));
  return { ...next, [k]: first };
}
