// The Undone mark of a History row: a record that a later undo put the take back. Pure, no imports, so Node tests it.
// It is worked out in the browser from the `undo` and `retry` ids of the loaded entries (docs/agents/history.md). It does
// not show what the destination has now. The Group header and the stacked row use the same words, so keep them here.

export const UNDONE = 'Undone';
export const PARTLY = 'Partly undone';
const DEPTH = 50; // a retry chain this long is not real; it guards a loop

// Levels of an entry that the router confirmed. A failed take lists the levels it did not confirm.
const confirmed = (x) => (x.ok ? x.levels : x.levels.filter((u) => !(x.failed || x.levels).includes(u)));

// The levels a take's mark is judged on: the ones it confirmed. An undo of a failed take leaves out the levels the router
// answered for (untouchedLevels in plans.js), so the full levels would never reach Undone. A take that confirmed none
// keeps all its levels, since its undo sends the ones with no answer.
export const takeLevels = (x) => {
  const got = confirmed(x);
  return got.length ? got : x.levels;
};

// The mark of one History entry (markFor), on its takeLevels.
export const takeMark = (x, index) => markFor([x.id], takeLevels(x), index);

// The undo row that a take belongs to. It is the entry itself when it is an undo. Else it is the failed undo that a Retry
// repeats, also through a Retry of a Retry. null when the chain does not reach a loaded undo.
export function undoRoot(x, byId) {
  let cur = x;
  for (let i = 0; i < DEPTH && cur; i++) {
    if (cur.undo != null) return cur;
    cur = cur.retry != null ? byId.get(cur.retry) : null;
  }
  return null;
}

// list: all loaded entries, not only the shown ones, so a filter that hides an undo row still marks the take.
// Answers Map(take id → Map(undo row id → confirmed levels)). A Retry counts toward the undo row it repeats.
export function undoIndex(list) {
  const byId = new Map(list.map((e) => [e.id, e]));
  const index = new Map();
  for (const x of list) {
    const root = undoRoot(x, byId);
    if (!root) continue;
    const got = confirmed(x);
    if (!got.length) continue;
    if (!index.has(root.undo)) index.set(root.undo, new Map());
    const by = index.get(root.undo);
    by.set(root.id, [...new Set([...(by.get(root.id) || []), ...got])]);
  }
  return index;
}

// The mark of one row: ids are the entries it stands for (a stacked row has its chain), levels its levels. Answers
// null (no mark) or { state: 'undone' | 'partly', text, levels, by } with levels the ones that went back, by the undo
// row ids.
export function markFor(ids, levels, index) {
  const back = new Set();
  const by = new Set();
  for (const id of ids) {
    for (const [undoId, got] of index.get(id) || []) {
      const hit = got.filter((u) => levels.includes(u));
      if (!hit.length) continue;
      hit.forEach((u) => back.add(u));
      by.add(undoId);
    }
  }
  if (!back.size) return null;
  const all = levels.every((u) => back.has(u));
  return {
    state: all ? 'undone' : 'partly',
    text: all ? UNDONE : PARTLY,
    levels: levels.filter((u) => back.has(u)),
    by: [...by].sort((a, b) => a - b),
  };
}

// A Group's header: Undone when every route is, "3 of 5 undone" when only some are, Partly undone when none is whole
// but one is partly. entries are the Group's routes; External follows are not in it.
export function groupMark(entries) {
  const n = entries.length;
  const whole = entries.filter((e) => e.undone?.state === 'undone').length;
  if (n && whole === n) return { state: 'undone', text: UNDONE };
  if (whole) return { state: 'some', text: `${whole} of ${n} undone` };
  if (entries.some((e) => e.undone)) return { state: 'partly', text: PARTLY };
  return null;
}
