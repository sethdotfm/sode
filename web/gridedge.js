// A paste past the edge of the grid: what Clip and Wrap would place, and the words for the dialog and the status
// line. Pure (test/gridedge.test.mjs); the dialog is editorpasteedge.js. plan is gridclip.js pastePlan's, with
// absolute placed cells. Wrap walks squareOrder, the one fill order Find uses, so a new order shows up in both.
import { squareOrder, FILL_ORDERS } from './gridmodel.js';
import { pasteNote } from './gridclip.js';

const plural = (n, one, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;
const overlap = (a, b) => !(a.r + a.rs - 1 < b.r || b.r + b.rs - 1 < a.r || a.c + a.cs - 1 < b.c || b.c + b.cs - 1 < a.c);
const reading = (a, b) => a.r - b.r || a.c - b.c;

// Cells wholly outside go; a merged cell partly outside keeps the squares inside. A cell's top left is never
// above or left of the paste square, so only the bottom and right can run out.
export function clipPaste(g, plan) {
  let dropped = 0;
  let shrunk = 0;
  const placed = [];
  for (const c of plan.placed) {
    if (c.r > g.rows || c.c > g.cols) {
      dropped++;
      continue;
    }
    const rs = Math.min(c.rs, g.rows - c.r + 1);
    const cs = Math.min(c.cs, g.cols - c.c + 1);
    if (rs !== c.rs || cs !== c.cs) shrunk++;
    placed.push({ ...c, rs, cs });
  }
  return { placed, dropped, shrunk };
}

// The block's cells in reading order on successive squares of the whole grid in fill order, from the paste square
// to the order's last square, never back to the first. A merged cell goes whole when it stays inside and covers no
// square already placed, else it shrinks to one square; squares a placed cell covers are stepped over. Cells left
// when the squares run out are dropped (the cascade to the next Page will take them on).
export function wrapPaste(g, plan, order) {
  const walk = squareOrder({ r1: 1, c1: 1, r2: g.rows, c2: g.cols }, order);
  let i = walk.findIndex(([r, c]) => r === plan.at.r && c === plan.at.c);
  const placed = [];
  let moved = 0;
  let shrunk = 0;
  let dropped = 0;
  for (const cell of [...plan.placed].sort(reading)) {
    while (i < walk.length && placed.some((p) => overlap(p, { r: walk[i][0], c: walk[i][1], rs: 1, cs: 1 }))) i++;
    if (i >= walk.length) {
      dropped++;
      continue;
    }
    const [r, c] = walk[i++];
    let spot = { ...cell, r, c };
    if (r + cell.rs - 1 > g.rows || c + cell.cs - 1 > g.cols || placed.some((p) => overlap(p, spot))) {
      if (cell.rs * cell.cs > 1) shrunk++;
      spot = { ...spot, rs: 1, cs: 1 };
    }
    if (r !== cell.r || c !== cell.c) moved++;
    placed.push(spot);
  }
  return { placed, moved, shrunk, dropped };
}

const orderOf = (order) => FILL_ORDERS.find((o) => o.id === order) || FILL_ORDERS[0];

const joinAnd = (head, rest) => (rest.length ? `${head}${rest.length > 1 ? `, ${rest.slice(0, -1).join(', ')},` : ','} and ${rest.at(-1)}` : head);

function clipText(x, past) {
  const parts = [];
  if (x.dropped)
    parts.push(
      past ? `dropped ${plural(x.dropped, 'cell')}` : `drops ${plural(x.dropped, 'cell')} that ${x.dropped === 1 ? 'falls' : 'fall'} outside the grid`,
    );
  if (x.shrunk) parts.push(`${past ? 'shrank' : 'shrinks'} ${plural(x.shrunk, 'merged cell')}`);
  return `Clip ${parts.join(' and ')}.`;
}

function wrapText(x, orderName, past) {
  const rest = [];
  if (x.shrunk) rest.push(`${past ? 'shrank' : 'shrinks'} ${plural(x.shrunk, 'merged cell')}`);
  if (x.dropped) rest.push(`${past ? 'dropped' : 'drops'} ${x.dropped} that ${past ? 'found' : x.dropped === 1 ? 'finds' : 'find'} no square`);
  return `${joinAnd(`Wrap ${past ? 'moved' : 'moves'} ${plural(x.moved, 'cell')} along ${orderName}`, rest)}.`;
}

// Both choices worked out before the dialog opens, so it can name the counts. order is Find's fill order.
export function edgeChoices(g, plan, order) {
  const { id, name } = orderOf(order);
  const clip = { kind: 'clip', ...clipPaste(g, plan) };
  const wrap = { kind: 'wrap', orderName: name, ...wrapPaste(g, plan, id) };
  return { orderName: name, clip: { ...clip, text: clipText(clip) }, wrap: { ...wrap, text: wrapText(wrap, name) } };
}

// The status line once a choice is applied: what a paste that fits would say, then what the choice did.
export function edgeNote(plan, choice) {
  const did = choice.kind === 'wrap' ? wrapText(choice, choice.orderName, true) : clipText(choice, true);
  return `${pasteNote({ ...plan, fits: true, placed: choice.placed })} ${did}`;
}
