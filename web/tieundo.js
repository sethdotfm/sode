// Undo of a tie take on the main page (docs/tielines-design.md, Undo; rules in tieundorules.js). Every Undo of a
// destination entry of a tie take goes through guardTieUndo before its confirm opens: the row's Undo, Undo all and
// Ctrl+Z. The server judges the Via chain (tieundo.go); the page never computes it.

import { S, el, request, srcItem, dstItem } from './core.js';
import { showResult } from './banner.js';
import { undoPlan } from './plans.js';
import { tieUndoIds, tieUndoPlan, pickerPlan } from './tieundorules.js';

// The put-back item of a hop entry: its Undo, tagged putBack (as historyrows.js putBacks).
function putItem(id) {
  const hop = S.hist.find((x) => x.id === id);
  if (!hop) return null;
  const [it] = undoPlan(hop).items;
  return it ? { ...it, putBack: true } : null;
}

const label = (e) => dstItem(e.n, e.name || '', e.router).label;

// guardTieUndo: false when plan has no Undo of a tie take, so the caller opens its confirm as usual. Else it asks the
// server and later calls go with the plan to confirm, or says why Undo sends nothing.
export function guardTieUndo(plan, go) {
  const ids = tieUndoIds(plan);
  if (!ids.length) return false;
  request('tieUndoResult', { t: 'tieUndo', entries: ids }).then((m) => {
    if (!m.checks) return showResult('error', 'Not sent', m.err || 'Sode could not check the Undo.');
    const out = tieUndoPlan(plan, m.checks, putItem, label);
    if (out.changed) return notSent(plan, out.changed, go);
    go({ ...out.plan, tieChecked: true });
  });
  return true;
}

// notSent: the chain changed for another reason. Undo sends nothing, and the banner offers the picker for the source
// from before.
function notSent(plan, check, go) {
  const e = plan.items.map((i) => i.undo).find((x) => x?.id === check.entry);
  const text = `${check.text}. Sode sent nothing.`;
  if (!e || !check.want) return showResult('warn', 'Undo not sent', text);
  const src = srcItem(check.want.n, check.want.name || '', check.want.router);
  showResult('warn', 'Undo not sent', () => {
    const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', `Choose tie lines for ${src.label}`);
    b.type = 'button';
    // Each layout draws its own copy of the banner, so no id.
    b.dataset.tieUndoPick = '';
    b.addEventListener('click', () =>
      go(
        pickerPlan(e, check, {
          src: () => src,
          dst: (x) => dstItem(x.dst, x.dstName, x.router),
        }),
      ),
    );
    return el('div', null, el('div', null, text), b);
  });
}
