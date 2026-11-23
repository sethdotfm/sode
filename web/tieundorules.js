// Undo of a tie take (docs/tielines-design.md, Undo). Pure, no imports, so Node tests it. Undo puts back the
// destination asked for and never switches a tie line back by itself. Before the confirm opens, the page asks the
// server (the message tieUndo, internal/app/tieundo.go) for a verdict on each destination entry:
//   - same: a plain Undo.
//   - putBack: the chain differs only because this take switched a tie line that still carries what the take set. The
//     confirm offers "Put TIE 3 back to CAM 9" with its Affected destinations, as a box that is off until ticked.
//   - changed: Undo sends nothing, says what changed and offers the picker for the source from before.

// tieUndoIds: the History ids of the items of an undo plan that undo the destination of a tie take that succeeded. A
// put-back item is the Undo of a hop entry, not of a destination.
export function tieUndoIds(plan) {
  if (plan.verb !== 'undo' || plan.tieChecked) return [];
  const ids = [];
  for (const it of plan.items) {
    const e = it.undo;
    if (e?.origin && e.ok && !it.putBack && !ids.includes(e.id)) ids.push(e.id);
  }
  return ids;
}

const plural = (n, one, many) => (n === 1 ? one : many);

// putBackText: "Also put TIE 3 back to CAM 9. Affected destinations: B MON 20, B MON 21". label(end) names a
// destination ({router, n, name}).
export function putBackText(puts, label) {
  const what = puts.map((p) => `${p.tieName || p.tie} back${p.to ? ` to ${p.to.name || `source ${p.to.n}`}` : ''}`).join(', ');
  const hits = [...new Map(puts.flatMap((p) => p.affected || []).map((e) => [`${e.router} ${e.n}`, e])).values()];
  const shown = hits.slice(0, 4).map(label);
  const more = hits.length > shown.length ? `, and ${hits.length - shown.length} more` : '';
  return `Also put ${what}. Affected destinations: ${hits.length ? `${shown.join(', ')}${more}` : 'none'}`;
}

// tieUndoPlan joins the verdicts (checks) into the plan. Gives { changed } with the first changed check, or { plan }.
// putItem(entryId) gives the put-back item of a hop entry, or null when its entry is not loaded. The page cannot
// offer it then, so the Undo counts as changed. label names an Affected destination.
export function tieUndoPlan(plan, checks, putItem, label) {
  const changed = checks.find((c) => c.verdict === 'changed');
  if (changed) return { changed };
  const puts = checks.flatMap((c) => (c.verdict === 'putBack' ? c.putBack || [] : []));
  if (!puts.length) return { plan };
  const items = puts.map((p) => putItem(p.entry));
  if (items.some((i) => !i))
    return { changed: { ...checks.find((c) => c.verdict === 'putBack'), verdict: 'changed', text: 'The entry of a tie line to put back is not loaded' } };
  const text = putBackText(puts, label);
  const old = plan.extra;
  // A destination followed by External follow entries keeps that offer in the same box.
  const extra = old
    ? {
        items: [...old.items, ...items],
        on: false,
        label: `${text}. Also undo ${old.items.length} External follow ${plural(old.items.length, 'entry', 'entries')}`,
      }
    : { items, on: false, label: text };
  return { plan: { ...plan, extra } };
}

// pickerPlan opens the picker after an Undo that sent nothing. Its route goes from the source from before (check.want)
// to the destination of entry e. ends: { src(want), dst(e) } give the page's items.
export function pickerPlan(e, check, ends) {
  return {
    verb: 'undo',
    items: [
      {
        dst: ends.dst(e),
        groups: [{ src: ends.src(check.want), levels: [...(check.levels || e.levels)] }],
        skipped: [],
        untouched: [],
        undo: null,
        retry: null,
      },
    ],
  };
}
