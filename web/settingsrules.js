// Pure rules for the Settings form (Node-tested).

// entries come from the form's own inputs, not the built config. A blank Auto-lock time means 0 (never).
export function intProblem(entries) {
  for (const e of entries) {
    if (e.type !== 'int') continue;
    const v = String(e.value ?? '').trim();
    if (v === '' && e.key === 'autoLockMinutes') continue;
    if (v === '' || !Number.isInteger(Number(v))) return e;
  }
  return null;
}

// Go sends a zero time ("0001-…") for "never".
const realTime = (iso) => !!iso && new Date(iso).getFullYear() > 2000;

// The Router names time: a finished read wins over the saved names table (savedAt).
export function namesTimeText({ at, savedAt }, fmt) {
  if (realTime(at)) return `Re-read ${fmt(at)}`;
  if (realTime(savedAt)) return `Names from ${fmt(savedAt)}`;
  return 'Not read yet';
}

// Level colors are one palette by position for every router (owner decision 7). The tip of a slot says which level of
// each router takes it, the router prefix first: "Position 1. A: 1, B: 0, C: V." routers: the hello's routers list. With
// one router the slot needs no tip.
export function positionTip(pos, routers) {
  if (routers.length < 2) return '';
  const uses = routers.map((r) => [r.prefix, (r.levels || []).find((l) => l.ui === pos)?.label]).filter(([, label]) => label !== undefined);
  if (!uses.length) return `Position ${pos}. No router has this many levels.`;
  return `Position ${pos}. ${uses.map(([p, label]) => `${p}: ${label}`).join(', ')}.`;
}
