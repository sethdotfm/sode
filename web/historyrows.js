// History rows, for the desktop list and the phone's swipe rows. Pure: wording and plans only,
// each layout builds its own DOM from these.

import { S, srcItem, dstItem } from './core.js';
import { undoPlan, againPlan, groupUndoPlan, lastTake, nextTake, usedNext } from './plans.js';
import { groupPartial, groupIds } from './history.js';
import { UNDO_TIP_ALL } from './widgets.js';
import { protectRow, protectOfferOf } from './protects.js';
import { protectUndoable } from './protectrules.js';

const noun = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;
const againWord = (again) => (again.verb === 'retry' ? 'Retry' : 'Redo');

// What a row offers: undo {plan, label, text, tip} and again {plan, label}. tip is undefined unless the
// row has its own wording (the buttons then use their default). row is {e} or {f}.
export function rowActions(row) {
  if (row.f) {
    const f = row.f;
    const again = againPlan({ f });
    return {
      undo: {
        plan: groupUndoPlan(f),
        label: `Undo all ${f.entries.length} routes of ${groupIds(f)}`,
        text: groupPartial(f) ? `Undo all ${f.entries.length}` : 'Undo all',
        tip: UNDO_TIP_ALL,
      },
      again: { plan: again, label: `${againWord(again)} ${noun(again.items.length, 'route')} of ${groupIds(f)}` },
    };
  }
  const e = row.e;
  if (e.protect) return protectActions(e);
  const src = srcItem(e.src, e.srcName),
    dst = dstItem(e.dst, e.dstName);
  const again = againPlan({ e });
  const undo = e.chain
    ? {
        plan: undoPlan(e),
        label: `Undo ${e.chain.length} changes on ${dst.label}`,
        text: `Undo ${e.chain.length}`,
        tip: 'Restores the source from before these changes',
      }
    : { plan: undoPlan(e), label: `Undo ${src.label} to ${dst.label}`, text: 'Undo', tip: undefined };
  return { undo, again: { plan: again, label: `${againWord(again)} ${src.label} to ${dst.label}` } };
}

// Undo works only on the newest Protect entry of its destination, and only when the route view would offer the
// opposite now (protectUndoable). The layouts draw the rows again on a Protect change, so the button follows it.
function protectActions(e) {
  const r = protectRow(e);
  const can = protectUndoable(e, S.hist, protectOfferOf(e.dst));
  return {
    undo: { plan: can ? r.undo : { verb: 'undo', items: [] }, label: r.undoLabel, text: 'Undo', tip: r.undoLabel },
    again: { plan: { verb: 'redo', items: [] }, label: '' },
  };
}

// Aria label of a Group's disclosure: "Show the 3 takes", "Hide the 2 followers".
export const disclosureAria = (open, n, what) => `${open ? 'Hide' : 'Show'} the ${n} ${what}`;

// The External follow header of a Group: the disclosure's count text, aria label, and the id of its kids.
export function followersHead(f, open, idPrefix) {
  return {
    count: `${f.followers.length}`,
    aria: disclosureAria(open, f.followers.length, 'External follow entries'),
    kidsId: `${idPrefix}-followers-${f.group}`,
  };
}

// The Last / Next fill of one box (k: src, dst or lvl). null: nothing to fill for k; {none: true}:
// no earlier take at all; else {t}, with a Next fill remembered so a repeat goes on. state: the boxes.
export function lastFill(k, next, state) {
  const t = next ? nextTake(state) : lastTake();
  if (!t) return { none: true };
  if (k === 'src' && !t.src) return null;
  if (k === 'dst' && !t.dsts.length) return null;
  if (next && k !== 'lvl') usedNext(k, t);
  return { t };
}
