// Undo on a Grid panel. Pure. st is gridpanelstage.js's state. next is GET /panels/{id}/undo's `undo`: null when this
// instance has nothing left to undo. The server plans and sends the undo whole (docs/agents/panels.md, Undo cell).
// So the stage holds it as given. It ignores the Levels box, nothing trims it, and no part goes through /takes.
import { afterTake } from './gridpanelstage.js';

const refuse = (st, error) => ({ ...st, error });

// The undo in place of the staged routes. A refusal leaves the stage as it was.
export function stageUndo(st, next, env) {
  if (!next) return refuse(st, { code: 'nothing-to-undo' });
  if (!next.allowed) return refuse(st, { code: 'undo-refused', message: next.reason });
  const dst = next.routes.find((r) => env.protectedDsts.has(r.dst))?.dst;
  if (dst !== undefined) return refuse(st, { code: 'protected', dst });
  const staged = next.routes.flatMap(({ src, dst: d, levels }) => levels.map((level) => ({ dst: d, level, src })));
  return { ...st, staged, undoId: next.id, error: null };
}

// A press stages over a staged undo only after it drops the undo whole.
export const leaveUndo = (st) => (st.undoId === null ? st : { ...st, staged: [], undoId: null });

// A failed undo keeps no plan, since it is possible that its id is stale.
export const afterUndo = (st, failed) => ({ ...afterTake(st, failed), staged: [], undoId: null });

export function undoText(err) {
  switch (err?.code) {
    case 'nothing-to-undo':
      return 'Nothing to undo';
    case 'undo-refused':
      return `Undo is refused: ${err.message}`;
    default:
      return '';
  }
}
