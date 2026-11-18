// Rights rules, pure (no imports) so Node tests them. Rights on the wire: {sources, dsts, levels, presets, visibility}.
// A missing list means all and [] means none. Sources and dsts are router references {router, n, name}.
// n is the router's own number, with no numberOffset, as in plan items' .n. The editor sends {n}, never a bare
// number; the server fills the router and the name.
// Levels are UI levels 1..N. Presets are preset ids. Keep in step with config.Rights.

// refN: a list entry's router number. v2.0 drives one router, so the router is not compared. Tests and plans
// pass plain numbers.
export const refN = (x) => (x && typeof x === 'object' ? x.n : x);

export const allowed = (rights, kind, v) => !rights || !Array.isArray(rights[kind]) || rights[kind].some((x) => refN(x) === v);

// The reason the server would refuse the plan whole, or ''. Same order and words as outsideRights in
// internal/app/rights.go. Undo restores what the router had, so it skips the Before source.
export function planOutside(rights, plan, off, levelName = String) {
  if (!rights) return '';
  if ((plan.preset || plan.presetId) && !allowed(rights, 'presets', plan.presetId)) return `preset "${plan.preset}" is outside your rights`;
  for (const it of plan.items) {
    if (!it.groups.length) continue;
    if (!allowed(rights, 'dsts', it.dst.n)) return `destination ${it.dst.n + off} is outside your rights`;
    for (const g of it.groups) {
      for (const u of g.levels) if (!allowed(rights, 'levels', u)) return `level ${levelName(u)} is outside your rights`;
      if (!it.undo && !allowed(rights, 'sources', g.src.n)) return `source ${g.src.n + off} is outside your rights`;
    }
  }
  return '';
}

// ---- the editor: a list is "all" or "only these", typed as shown numbers

export function listText(list, off) {
  const shown = [...new Set(list.map((x) => refN(x) + off))].sort((a, b) => a - b);
  const out = [];
  for (let i = 0; i < shown.length; ) {
    let j = i;
    while (shown[j + 1] === shown[j] + 1) j++;
    out.push(j > i ? `${shown[i]}-${shown[j]}` : String(shown[i]));
    i = j + 1;
  }
  return out.join(', ');
}

export function parseList(text, off) {
  const out = new Set();
  for (const tok of text.split(/[\s,]+/).filter(Boolean)) {
    const m = /^(\d+)(?:-(\d+))?$/.exec(tok);
    if (!m) return { error: `"${tok}" is not a number or a range like 3-7` };
    const [a, b] = [Number(m[1]), Number(m[2] ?? m[1])];
    if (b < a) return { error: `"${tok}" runs backwards` };
    if (a - off < 0) return { error: `"${tok}" is below the first number, ${off}` };
    if (b - a > 100000) return { error: `"${tok}" is too long a range` };
    for (let n = a; n <= b; n++) out.add(n - off);
  }
  return { list: [...out].sort((x, y) => x - y) };
}

export const parseNames = (text) => [
  ...new Set(
    text
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean),
  ),
];

const NUMBERED = [
  ['sources', 'Sources', true],
  ['dsts', 'Destinations', true],
  ['levels', 'Levels', false],
];

// presets: [{id, name}] the editor can name. The draft shows names. An id it cannot name stays in keep
// and goes back on save: a deleted preset, or one it does not list.
export function draftOf(rights, off, presets = []) {
  const r = rights || {};
  const d = { visibility: r.visibility === 'hidden' ? 'hidden' : 'readonly' };
  for (const [kind, , shifted] of NUMBERED)
    d[kind] = { all: !Array.isArray(r[kind]), text: Array.isArray(r[kind]) ? listText(r[kind], shifted ? off : 0) : '' };
  const ids = Array.isArray(r.presets) ? r.presets : [];
  const nameOf = new Map(presets.map((p) => [p.id, p.name]));
  d.presets = {
    all: !Array.isArray(r.presets),
    text: ids
      .filter((id) => nameOf.has(id))
      .map((id) => nameOf.get(id))
      .join('\n'),
    keep: ids.filter((id) => !nameOf.has(id)),
  };
  return d;
}

// Typed names to ids, ignoring case, or {error} for a name no preset has.
function presetIds(text, keep, presets) {
  const out = [];
  for (const name of parseNames(text)) {
    const p = presets.find((x) => x.name.toLowerCase() === name.toLowerCase());
    if (!p) return { error: `Presets: no preset is named "${name}"` };
    if (!out.includes(p.id)) out.push(p.id);
  }
  return { ids: [...out, ...(keep || []).filter((id) => !out.includes(id))] };
}

// {rights} for the wire, or {error} naming the first bad list. levels: how many exist.
export function rightsOf(d, off, levels, presets = []) {
  const rights = {};
  for (const [kind, name, shifted] of NUMBERED) {
    if (d[kind].all) continue;
    const p = parseList(d[kind].text, shifted ? off : 0);
    if (p.error) return { error: `${name}: ${p.error}` };
    if (kind === 'levels' && p.list.some((u) => u < 1 || u > levels)) return { error: `Levels: use 1-${levels}` };
    rights[kind] = shifted ? p.list.map((n) => ({ n })) : p.list;
  }
  if (!d.presets.all) {
    const p = presetIds(d.presets.text, d.presets.keep, presets);
    if (p.error) return p;
    rights.presets = p.ids;
  }
  if (d.visibility === 'hidden') rights.visibility = 'hidden';
  return { rights };
}

// ---- the Rights tab's entries. Rows are keyed by a number that never changes, not by the name under
// edit, so a rename keeps the selection.

export const entryKeys = (users, clients) => [...users.map((u) => `user:${u.rid}`), ...clients.map((c) => `client:${c.rid}`), 'guest'];

export const pickValid = (keys, pick) => (keys.includes(pick) ? pick : keys[0]);

export const rightsTab = (id) => ({ tab: 'rights', pick: `user:${id}` });

// An entry is {orig, draft, dirty}: what loaded, the editor's draft, and which fields the draft
// changed ('rights', 'noAutoLock', 'scopes'). extra holds noAutoLock (a user) or scopes (Guest).
// orig.rights is undefined for a row that never loaded. Untouched fields go back as loaded, so a save
// never rewrites rights through the editor's text (and a number offset change cannot shift them).
// presets: the [{id, name}] this entry can name, kept for the save.
export const newEntry = (rights, off, extra = {}, presets = []) => ({
  orig: { rights, ...extra },
  draft: { ...draftOf(rights, off, presets), ...extra },
  dirty: new Set(),
  presets,
});

function entryRights(e, off, levels) {
  if (!e) return {};
  if (!e.dirty.has('rights')) return e.orig.rights ? { rights: e.orig.rights } : {};
  return rightsOf(e.draft, off, levels, e.presets);
}

// The rights part of a `users` save: {users: [{rights?, noAutoLock?}] in row order, apiClients,
// guestRights?, guestScopes?} or {error}. A missing field tells the server to keep what it has.
// No auto-lock and Guest scopes go only after an edit.
export function accessReq({ entries, users, clients, off, levels }) {
  const out = { users: [], apiClients: [] };
  for (const u of users) {
    const e = entries.get(`user:${u.rid}`);
    const r = entryRights(e, off, levels);
    if (r.error) return { error: `${u.name}: ${r.error}` };
    out.users.push({ ...(r.rights ? { rights: r.rights } : {}), ...(e?.dirty.has('noAutoLock') ? { noAutoLock: e.draft.noAutoLock } : {}) });
  }
  for (const c of clients) {
    const r = entryRights(entries.get(`client:${c.rid}`), off, levels);
    if (r.error) return { error: `${c.name}: ${r.error}` };
    out.apiClients.push({ name: c.name.trim(), scopes: c.scopes, rights: r.rights || {} });
  }
  if (entries.has('guest')) {
    const g = entries.get('guest');
    const r = entryRights(g, off, levels);
    if (r.error) return { error: `Guest: ${r.error}` };
    out.guestRights = r.rights || {};
    if (g.dirty.has('scopes')) out.guestScopes = g.draft.scopes;
  }
  return out;
}

// req is usersReq's output. Rights go on each user; clients and Guest go along only after the
// directory has loaded them. So a page that never did cannot erase them.
export function mergeAccess(req, acc, loaded) {
  const out = { ...req, users: req.users.map((u, i) => ({ ...u, ...acc.users[i] })) };
  if (!loaded) return out;
  out.apiClients = acc.apiClients;
  if ('guestRights' in acc) out.guestRights = acc.guestRights;
  if ('guestScopes' in acc) out.guestScopes = acc.guestScopes;
  return out;
}
