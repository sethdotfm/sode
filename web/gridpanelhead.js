// The words in a Grid panel's header. Pure.

// The grid when it has nothing to show on an axis: a new Grid panel starts with no items (docs/panels-design.md).
export function emptyText({ srcs, dsts, canEdit }) {
  if (srcs && dsts) return null;
  const what = !srcs && !dsts ? 'sources or destinations' : !srcs ? 'sources' : 'destinations';
  return {
    title: `This panel shows no ${what}`,
    msg: canEdit ? 'Add them in the editor.' : 'The owner of the panel adds them in the editor.',
    edit: !!canEdit,
  };
}

// The banner's rule (gridpanelheadview.js say). An error (a failure, or a state that waits, held) outranks a result
// that went well. A lower one never takes the place of a higher one until its X or its state clears it. A new error
// of the same group joins the shown one ("2 takes failed"); another error takes its place.
// shown is { key, kind, held, title, line, group, names, gone } or null; gone marks one closed or timed out, whose key
// must not come back. m is { text, kind, key, held }. Returns { shown, changed, back }: back is true when m was held back.
const rank = (kind) => (kind === 'ok' ? 0 : 1);
const failedOn = /^(Take|Undo) failed on (.+)$/;
export function bannerStep(shown, m) {
  const same = { shown, changed: false, back: false };
  const live = shown && !shown.gone ? shown : null;
  if (!m.text) return live?.held ? { shown: null, changed: true, back: false } : same;
  if (shown?.key === m.key) return same;
  if (live && rank(m.kind) < rank(live.kind)) return { ...same, back: true };
  const f = failedOn.exec(m.text);
  const group = m.kind === 'ok' || m.held ? null : f ? f[1] : m.text;
  const names = f ? [f[2]] : [];
  if (live && group && live.group === group) {
    const all = [...live.names, ...names];
    const n = (live.n || 1) + 1;
    const title = f ? (f[1] === 'Take' ? `${n} takes failed` : `Undo failed ${n} times`) : `${m.text} (${n} times)`;
    return { shown: { ...live, key: m.key, title, line: all.length ? `On ${all.join(', ')}` : '', names: all, n }, changed: true, back: false };
  }
  return { shown: { key: m.key, kind: m.kind, held: !!m.held, title: m.text, line: '', group, names, n: 1 }, changed: true, back: false };
}
