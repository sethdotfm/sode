// Name matching for the import's remap step (importremap.js). The server answers each file router's rows and the
// target's names (remapInfo in internal/app/importremap.go) and applies the numbers sent back as names. No core.js
// import, so Node tests load it.

import { fuzzyScore } from './fuzzy.js';

// Keep in step with normName in internal/app/importremap.go.
export const norm = (s) => (s || '').trim().toLowerCase();

const NEEDS = ['missing', 'dup-target', 'blank'];
const ORDER = [...NEEDS, 'moved', 'as-is'];

// One row per file reference. as-is: number and name agree. moved: one exact match at another number. missing: no
// match. dup-target: more than one match, none on the file's number. blank: no file name, a named target number.
// preset is the preselected target numbers; [] is Skip.
export function remapRows(router, target) {
  const names = { source: new Map(), destination: new Map() };
  for (const x of target.sources || []) names.source.set(x.n, x.name);
  for (const x of target.destinations || []) names.destination.set(x.n, x.name);
  const fileCount = new Map();
  for (const r of router.refs) {
    const k = `${r.kind}|${norm(r.name)}`;
    if (norm(r.name)) fileCount.set(k, (fileCount.get(k) || 0) + 1);
  }
  const rows = router.refs.map((r) => {
    const live = names[r.kind] || new Map();
    const row = { ...r, state: 'as-is', preset: null, candidates: [], note: '' };
    const want = norm(r.name);
    if (norm(live.get(r.n)) === want) return row;
    if (!want) return { ...row, state: 'blank', preset: [] };
    const cands = [...live].filter(([, name]) => norm(name) === want).map(([n]) => n);
    cands.sort((a, b) => a - b);
    row.candidates = cands;
    if (cands.length === 1) {
      row.state = 'moved';
      row.preset = [cands[0]];
      if (fileCount.get(`${r.kind}|${want}`) > 1) row.note = 'This name is twice in the file.';
      return row;
    }
    if (cands.length > 1) return { ...row, state: 'dup-target', preset: [], addBoth: r.kind === 'destination' };
    return { ...row, state: 'missing', preset: [], suggestion: suggest(r.name, live) };
  });
  return rows.sort((a, b) => ORDER.indexOf(a.state) - ORDER.indexOf(b.state) || (a.kind < b.kind ? -1 : a.kind > b.kind ? 1 : a.n - b.n));
}

export const needsChoice = (row) => NEEDS.includes(row.state);

// The target number whose name is nearest, or undefined. A suggestion is one click, never preselected.
function suggest(name, live) {
  const q = norm(name);
  let best;
  let score = -1;
  for (const [n, s] of live) {
    if (!norm(s)) continue;
    const v = fuzzyScore({ num: '', lname: norm(s) }, q);
    if (v > score) [best, score] = [n, v];
  }
  return best;
}

export function newChoices(rows) {
  const out = {};
  for (const r of rows) if (r.state !== 'as-is') out[r.key] = [...r.preset];
  return out;
}

// importReq.names: Skip is left out, since the server keeps an unchosen reference's number.
export function namesReq(rows, choices) {
  const out = {};
  for (const r of rows) if (r.state !== 'as-is' && choices[r.key]?.length) out[r.key] = [...choices[r.key]];
  return out;
}

const same = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);

export function remapSummary(rows, choices) {
  const n = { asIs: 0, auto: 0, hand: 0, skipped: 0 };
  for (const r of rows) {
    const ch = choices[r.key] || [];
    if (r.state === 'as-is') n.asIs++;
    else if (!ch.length) n.skipped++;
    else if (r.state === 'moved' && same(ch, r.preset)) n.auto++;
    else n.hand++;
  }
  return n;
}

// The item's Broken list less what the choices move. rowsBy is by file router UID; targets maps file UID to target.
export function stillBroken(broken, rowsBy, targets, choices) {
  return (broken || []).filter((m) => {
    if (m.kind !== 'source' && m.kind !== 'destination') return true;
    for (const [uid, rows] of Object.entries(rowsBy)) {
      if (targets[uid] !== m.ref?.router) continue;
      const r = rows.find((x) => x.kind === m.kind && x.n === m.ref.n);
      if (r && choices[r.key]?.length) return false;
    }
    return true;
  });
}

// The target picker for one file router. A target another file router holds is taken.
export function routerOptions(router, targets) {
  const out = [{ value: '', text: 'None: its items are not imported', disabled: false }];
  for (const c of router.candidates || []) {
    const taken = Object.entries(targets).some(([uid, t]) => uid !== router.uid && t === c.uid);
    out.push({ value: c.uid, text: `${c.title} (${c.found} of ${c.total} names found)`, disabled: taken });
  }
  return out;
}

// offset: the shown number of source or destination 0.
export const rowLabel = (r, offset) => (norm(r.name) ? `${r.name.trim()} (${r.n + offset})` : String(r.n + offset));

const count = (n, one) => `${n} ${one}${n === 1 ? '' : 's'}`;

export function usedBy(r) {
  const parts = [];
  if (r.presets) parts.push(count(r.presets, 'preset'));
  if (r.panels) parts.push(count(r.panels, 'panel'));
  if (r.users) parts.push(count(r.users, 'user'));
  return parts.length ? `used by ${parts.join(', ')}` : '';
}

// The note above a target's rows when its names are not a fresh read. fmt formats an ISO time.
export function namesNote(t, fmt) {
  if (t.noNames) return 'No router names yet. Import after Sode reads them.';
  const from = t.savedAt ? `the saved names table from ${fmt(t.savedAt)}` : t.namesAt ? `the names read at ${fmt(t.namesAt)}` : 'the last names read';
  if (t.offline) return `Router offline: using ${from}. Sode checks again on connect.`;
  if (t.savedAt) return `Using ${from} until Sode reads the router.`;
  return '';
}

// The bulk buttons. keys: the selected rows, or none for every row that needs a choice. suggest: Use all
// suggestions. keepBlank: Keep numbers for all blank names. skip: Reset to Skip.
export function bulk(rows, choices, action, keys) {
  const on = rows.filter((r) => r.state !== 'as-is' && (keys?.size ? keys.has(r.key) : needsChoice(r)));
  for (const r of on) {
    if (action === 'skip') choices[r.key] = [];
    else if (action === 'suggest' && r.suggestion !== undefined) choices[r.key] = [r.suggestion];
    else if (action === 'keepBlank' && r.state === 'blank') choices[r.key] = [r.n];
  }
}

// A picker value (skip, keep, suggest, all, n:<number>) on one row. With that row selected it goes to every
// selected row where it applies. Keep number goes to blank rows, and a suggestion or Add both to rows that have
// one. A number goes to rows of the same kind.
export function applyChoice(rows, choices, key, value, keys) {
  const row = rows.find((r) => r.key === key);
  if (!row) return;
  const on = keys?.has(key) ? rows.filter((r) => r.state !== 'as-is' && keys.has(r.key)) : [row];
  for (const r of on) {
    if (value === 'skip') choices[r.key] = [];
    else if (value === 'keep') {
      if (r.state === 'blank') choices[r.key] = [r.n];
    } else if (value === 'suggest') {
      if (r.suggestion !== undefined) choices[r.key] = [r.suggestion];
    } else if (value === 'all') {
      if (r.addBoth) choices[r.key] = [...r.candidates];
    } else if (value.startsWith('n:') && r.kind === row.kind) choices[r.key] = [+value.slice(2)];
  }
}
