// Staged Settings edits across Step in and Step out (docs/agents/settings-saves.md): who staged each, and which ones the
// saver's Role cannot see. Pure, no imports (Node-tested). Rows are diffRows rows (savereviewrules.js).

const same = (a, b) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null);
export const rowId = (r) => `${r.key}/${r.sub}`;

// marks: rowId -> {by, now}. who made the rows as they are now; a row they did not change keeps its author.
export function noteAuthors(marks, rows, who) {
  const out = new Map();
  for (const r of rows) {
    const m = marks.get(rowId(r));
    out.set(rowId(r), m && same(m.now, r.now) ? m : { by: who, now: r.now });
  }
  return out;
}

// seen(row): the saver's Role can see the row's section.
export function splitSeen(rows, seen) {
  const shown = [];
  const unseen = [];
  for (const r of rows) (seen(r) ? shown : unseen).push(r);
  return { shown, unseen };
}

export const sentKeys = (rows, seen) => [...new Set(splitSeen(rows, seen).shown.map((r) => r.key))];

export function unseenNotes(unseen, by) {
  const n = new Map();
  for (const r of unseen) n.set(by(r), (n.get(by(r)) || 0) + 1);
  return [...n].map(([who, k]) => `${k} ${k === 1 ? 'change' : 'changes'} by ${who} that your Role cannot see`);
}

export const namesAuthors = (rows, by, me) => rows.some((r) => by(r) !== me);
