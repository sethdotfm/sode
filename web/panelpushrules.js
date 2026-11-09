// What the Push modal on the panels page offers and sends. Pure. The server decides who may push to
// what (a panel's canEdit), and refuses the whole Push if any target is not allowed.

const same = (a, b) => String(a).toLowerCase() === String(b).toLowerCase();

// A group is the panels whose names differ only by a trailing number: EVS 1 to EVS 4 are "EVS".
export const groupKey = (name) =>
  String(name)
    .replace(/[\s_-]*\d+\s*$/, '')
    .trim() || String(name).trim();

// Panels the caller may push to: changeable, and not the one being pushed.
export const pushCandidates = (panels, srcId) => panels.filter((p) => p.canEdit && p.id !== srcId);

// Groups of two or more, by name, each with its panels in number order; every panel that stands
// alone goes in `single`.
export function pushGroups(candidates) {
  const by = new Map();
  for (const p of candidates) {
    const k = groupKey(p.name);
    by.set(k.toLowerCase(), [...(by.get(k.toLowerCase()) || []), { ...p, key: k }]);
  }
  const groups = [];
  const single = [];
  for (const list of by.values()) {
    const sorted = list.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));
    if (sorted.length > 1) groups.push({ key: sorted[0].key, ids: sorted.map((p) => p.id), panels: sorted });
    else single.push(sorted[0]);
  }
  groups.sort((a, b) => a.key.localeCompare(b.key));
  single.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));
  return { groups, single };
}

// The ids to push to: every panel of a ticked group plus the panels ticked one by one, once each,
// in the order of the list, never the source and never a panel the caller cannot change.
export function pushTargets(panels, srcId, { groups = [], ids = [] } = {}) {
  const cand = pushCandidates(panels, srcId);
  const keys = new Set(groups.map((g) => String(g).toLowerCase()));
  const one = new Set(ids);
  return cand.filter((p) => one.has(p.id) || keys.has(groupKey(p.name).toLowerCase())).map((p) => p.id);
}

// all, some or none of a group's panels are ticked, for the group's own box.
export function groupState(selected, group) {
  const n = group.ids.filter((id) => selected.has(id)).length;
  return n === 0 ? 'none' : n === group.ids.length ? 'all' : 'some';
}

export function setGroup(selected, group, on) {
  const next = new Set(selected);
  for (const id of group.ids) on ? next.add(id) : next.delete(id);
  return next;
}

export const pushBody = (ids) => ({ targets: [...ids] });

// Which of a set's panels a Push goes to: the ticked ones that the set holds, or all of them when
// none of its panels is ticked.
export function setTargets(set, selected) {
  const held = set.entries.map((e) => e.panel);
  const ticked = held.filter((id) => selected.has(id));
  return ticked.length ? ticked : held;
}

export const setBody = (set, selected) => ({ targets: setTargets(set, selected) });

const nameOf = (panels, id) => panels.find((p) => p.id === id)?.name || id;
const list = (names) => (names.length < 3 ? names.join(' and ') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`);

// The line after a Push, from the server's answer.
export function pushSummary(result, panels) {
  const done = (result.pushed || []).map((p) => nameOf(panels, p.panel));
  const skipped = (result.skipped || []).map((s) => nameOf(panels, s.panel));
  let text = done.length ? `Pushed to ${list(done)}. A checkpoint of each was kept.` : 'Nothing was pushed.';
  if (skipped.length) text += ` Skipped ${list(skipped)}, which no longer exists.`;
  return text;
}

// A checkpoint as one line: when, who, what it holds.
export function checkpointLabel(cp) {
  const when = String(cp.at || '')
    .replace('T', ' ')
    .slice(0, 16);
  return `${when}, ${cp.by}, ${cp.rows} by ${cp.cols}, ${cp.cells} ${cp.cells === 1 ? 'cell' : 'cells'}`;
}

export const scopeLabel = (scope) => (scope === 'everyone' ? 'Everyone' : 'Me');

export const setLabel = (set) => `${set.name} (${scopeLabel(set.scope)}, ${set.entries.length} ${set.entries.length === 1 ? 'panel' : 'panels'})`;

export const canSaveSet = (name, selected) => String(name).trim() !== '' && selected.size > 0;

export const ownerNote = (p, me) => (same(p.owner, me) ? '' : ` (${p.owner})`);
