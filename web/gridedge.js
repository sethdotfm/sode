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
// when the squares run out are dropped and listed in left, for cascadeWrap.
export function wrapPaste(g, plan, order) {
  const walk = squareOrder({ r1: 1, c1: 1, r2: g.rows, c2: g.cols }, order);
  let i = walk.findIndex(([r, c]) => r === plan.at.r && c === plan.at.c);
  const placed = [];
  let moved = 0;
  let shrunk = 0;
  const left = [];
  for (const cell of [...plan.placed].sort(reading)) {
    while (i < walk.length && placed.some((p) => overlap(p, { r: walk[i][0], c: walk[i][1], rs: 1, cs: 1 }))) i++;
    if (i >= walk.length) {
      left.push(cell);
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
  return { placed, moved, shrunk, dropped: left.length, left };
}

// Fill cascade (GLOSSARY.md): cells left past the last square of Page from go on to the later enabled Pages that hold
// no cells, each from its first square in fill order. Used and disabled Pages are skipped so their cells stay; it
// never goes back to Page 1. onto is { index, cells } per Page used; left counts the cells cut off.
export function cascadeWrap(pages, from, left, order, { rows, cols }) {
  const [r, c] = squareOrder({ r1: 1, c1: 1, r2: rows, c2: cols }, order)[0];
  const onto = [];
  let shrunk = 0;
  for (let i = from + 1; i < pages.length && left.length; i++) {
    if (pages[i].enabled === false || pages[i].cells?.length) continue;
    const w = wrapPaste({ rows, cols }, { at: { r, c }, placed: left }, order);
    onto.push({ index: i, cells: w.placed });
    shrunk += w.shrunk;
    left = w.left;
  }
  return { onto, shrunk, left: left.length };
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

const pageList = (onto) => joinAnd(`Page ${onto[0].index + 1}`, onto.slice(1).map((o) => `Page ${o.index + 1}`)).replace(/,? and/, onto.length > 2 ? ', and' : ' and');

function wrapText(x, orderName, past) {
  const rest = [];
  if (x.shrunk) rest.push(`${past ? 'shrank' : 'shrinks'} ${plural(x.shrunk, 'merged cell')}`);
  const carried = (x.onto || []).reduce((n, o) => n + o.cells.length, 0);
  if (carried) rest.push(`${past ? 'carried' : 'carries'} ${plural(carried, 'cell')} on to ${pageList(x.onto)}`);
  if (x.dropped && !x.onto) rest.push(`${past ? 'dropped' : 'drops'} ${x.dropped} that ${past ? 'found' : x.dropped === 1 ? 'finds' : 'find'} no square`);
  if (x.dropped && x.onto && !past) rest.push(`cuts off ${x.dropped} that ${x.dropped === 1 ? 'does' : 'do'} not fit before the last Page`);
  const text = `${joinAnd(`Wrap ${past ? 'moved' : 'moves'} ${plural(x.moved, 'cell')} along ${orderName}`, rest)}.`;
  return past && x.dropped && x.onto ? `${text} ${plural(x.dropped, 'cell')} ${x.dropped === 1 ? 'was' : 'were'} left over and cut off.` : text;
}

// Both choices worked out before the dialog opens, so it can name the counts. order is Find's fill order. cascade is
// { pages, from }, the Page list and the index of the Page pasted on, for Wrap to carry on to later Pages.
export function edgeChoices(g, plan, order, cascade = null) {
  const { id, name } = orderOf(order);
  const clip = { kind: 'clip', ...clipPaste(g, plan) };
  let wrap = { kind: 'wrap', orderName: name, ...wrapPaste(g, plan, id) };
  if (cascade && cascade.from < cascade.pages.length - 1) {
    const c = cascadeWrap(cascade.pages, cascade.from, wrap.left, id, g);
    wrap = { ...wrap, onto: c.onto, shrunk: wrap.shrunk + c.shrunk, dropped: c.left };
  }
  return { orderName: name, clip: { ...clip, text: clipText(clip) }, wrap: { ...wrap, text: wrapText(wrap, name) } };
}

// The status line once a choice is applied: what a paste that fits would say, then what the choice did.
export function edgeNote(plan, choice) {
  const did = choice.kind === 'wrap' ? wrapText(choice, choice.orderName, true) : clipText(choice, true);
  const placed = [...choice.placed, ...(choice.onto || []).flatMap((o) => o.cells)];
  return `${pasteNote({ ...plan, fits: true, placed })} ${did}`;
}
