// Rights rules, pure (no imports) so Node tests them. Rights on the wire: {sources, dsts, levels, presets, visibility}.
// A missing list means all and [] means none. Sources and dsts are the router's own numbers (no
// numberOffset, as in plan items' .n); levels are UI levels 1..N. Keep in step with config.Rights.

export const allowed = (rights, kind, v) => !rights || !Array.isArray(rights[kind]) || rights[kind].includes(v);

// The reason the server would refuse the plan whole, or ''. Same order and words as outsideRights in
// internal/app/rights.go. Undo restores what the router had, so it skips the Before source.
export function planOutside(rights, plan, off, levelName = String) {
  if (!rights) return '';
  if (plan.preset && !allowed(rights, 'presets', plan.preset)) return `preset "${plan.preset}" is outside your rights`;
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
  const shown = [...new Set(list.map((n) => n + off))].sort((a, b) => a - b);
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

export function draftOf(rights, off) {
  const r = rights || {};
  const d = { visibility: r.visibility === 'hidden' ? 'hidden' : 'readonly' };
  for (const [kind, , shifted] of NUMBERED)
    d[kind] = { all: !Array.isArray(r[kind]), text: Array.isArray(r[kind]) ? listText(r[kind], shifted ? off : 0) : '' };
  d.presets = { all: !Array.isArray(r.presets), text: Array.isArray(r.presets) ? r.presets.join('\n') : '' };
  return d;
}

// {rights} for the wire, or {error} naming the first bad list. levels: how many exist.
export function rightsOf(d, off, levels) {
  const rights = {};
  for (const [kind, name, shifted] of NUMBERED) {
    if (d[kind].all) continue;
    const p = parseList(d[kind].text, shifted ? off : 0);
    if (p.error) return { error: `${name}: ${p.error}` };
    if (kind === 'levels' && p.list.some((u) => u < 1 || u > levels)) return { error: `Levels: use 1-${levels}` };
    rights[kind] = p.list;
  }
  if (!d.presets.all) rights.presets = parseNames(d.presets.text);
  if (d.visibility === 'hidden') rights.visibility = 'hidden';
  return { rights };
}

// ---- the Rights tab's entries. Rows are keyed by a number that never changes, not by the name being
// edited, so a rename keeps the selection.

export const entryKeys = (users, clients) => [...users.map((u) => `user:${u.id}`), ...clients.map((c) => `client:${c.id}`), 'guest'];

export const pickValid = (keys, pick) => (keys.includes(pick) ? pick : keys[0]);

export const rightsTab = (id) => ({ tab: 'rights', pick: `user:${id}` });

// An entry is {orig, draft, dirty}: the rights as loaded, the editor's draft, and whether the
// draft was touched. Untouched rights go back as loaded, so saving never rewrites them through the
// editor's text (and a number offset change can't shift them).
function entryRights(e, off, levels) {
  if (!e) return {};
  if (!e.dirty) return e.orig ? { rights: e.orig } : {};
  return rightsOf(e.draft, off, levels);
}

// The rights part of a `users` save: {users: [{rights?, noAutoLock?}] in row order, apiClients,
// guestRights?, guestScopes?} or {error}. A missing field tells the server to keep what it has.
// flags: row id to the No auto-lock value, set once a row has loaded or edited it.
export function accessReq({ entries, users, clients, guestScopes, flags, off, levels }) {
  const out = { users: [], apiClients: [] };
  for (const u of users) {
    const r = entryRights(entries.get(`user:${u.rid}`), off, levels);
    if (r.error) return { error: `${u.name}: ${r.error}` };
    out.users.push({ ...(r.rights ? { rights: r.rights } : {}), ...(flags.has(u.rid) ? { noAutoLock: flags.get(u.rid) } : {}) });
  }
  for (const c of clients) {
    const r = entryRights(entries.get(`client:${c.rid}`), off, levels);
    if (r.error) return { error: `${c.name}: ${r.error}` };
    out.apiClients.push({ name: c.name.trim(), scopes: c.scopes, rights: r.rights || {} });
  }
  if (entries.has('guest')) {
    const r = entryRights(entries.get('guest'), off, levels);
    if (r.error) return { error: `Guest: ${r.error}` };
    out.guestRights = r.rights || {};
  }
  if (guestScopes) out.guestScopes = guestScopes;
  return out;
}

// req is usersReq's output. Rights go on each user; clients and Guest ride along only once the
// directory has loaded them, so a page that never did can't erase them.
export function mergeAccess(req, acc, loaded) {
  const out = { ...req, users: req.users.map((u, i) => ({ ...u, ...acc.users[i] })) };
  if (!loaded) return out;
  out.apiClients = acc.apiClients;
  if ('guestRights' in acc) out.guestRights = acc.guestRights;
  if ('guestScopes' in acc) out.guestScopes = acc.guestScopes;
  return out;
}

// The tab an arrow, Home or End key moves to, or null for any other key.
export function stepTab(ids, cur, key) {
  const i = ids.indexOf(cur);
  if (key === 'ArrowRight') return ids[(i + 1) % ids.length];
  if (key === 'ArrowLeft') return ids[(i + ids.length - 1) % ids.length];
  if (key === 'Home') return ids[0];
  if (key === 'End') return ids.at(-1);
  return null;
}
