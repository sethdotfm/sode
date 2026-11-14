// History as the views show it: rows and Groups from S.hist. Owns the rows memo (histRows),
// dropped when anything it reads changes.

import { el, S, on, onFirst, onMsg, emit, off, wsSend, srcItem, dstItem, labelNode, tallyText } from './core.js';
import { levelText } from './levelview.js';
import { filterFn } from './filter.js';
import { filterAllows } from './filterstore.js';
import { pref } from './device.js';

// ---- quick search boxes above Recent: reads back through the whole log and, unless this browser's setting says otherwise, ignores the filters. Not kept.

let query = { src: '', dst: '' };
const anything = () => true;
let searchSrc = anything,
  searchDst = anything;

export const searchActive = () => !!(query.src.trim() || query.dst.trim());
export function setSearch(next) {
  const was = searchActive();
  query = { src: String(next.src ?? ''), dst: String(next.dst ?? '') };
  if (was && !searchActive()) trimHist();
  searchSrc = filterFn(query.src, { fuzzy: true }) || searchSrc;
  searchDst = filterFn(query.dst, { fuzzy: true }) || searchDst;
  emit('search');
  emit('history');
}
export const searchRespectsFilters = () => pref('searchFilters') === 'on';

// Once a search is cleared, drop all but the newest entries again (whole groups). A page still
// on its way is for before the trim, so takePage ignores it.
const KEEP_AFTER_SEARCH = 200;
function trimHist() {
  let n = KEEP_AFTER_SEARCH;
  if (S.hist.length <= n) return;
  S.histLoading = false;
  const g = S.hist[n - 1].group;
  while (g && n < S.hist.length && S.hist[n].group === g) n++;
  S.hist = S.hist.slice(0, n);
  S.histMore = true;
}

// Names are the router's now and the ones logged with the entry, which differ when names
// changed (or another router is connected).
function found(e) {
  const either = (fn, n, now, then) => fn(n + off(), now) || (!!then && then !== now && fn(n + off(), then));
  const hit = (x) => either(searchSrc, x.src, srcItem(x.src, x.srcName).name, x.srcName) && either(searchDst, x.dst, dstItem(x.dst, x.dstName).name, x.dstName);
  const byPreset = !!e.preset && searchActive() && (!query.src.trim() || searchSrc(-1, e.preset)) && (!query.dst.trim() || searchDst(-1, e.preset));
  return hit(e) || byPreset || !!e.follows?.some(hit);
}

function passes(e) {
  if (searchActive()) {
    if (!found(e)) return false;
    if (!searchRespectsFilters()) return true;
  }
  return filterAllows(e);
}

// Changes elsewhere to one destination on the same levels within this long of each other show
// as one row (a monitor punched about).
const MERGE_MS = 30000;

// { e } for a take on its own, { f } for a folder (one multi-take confirm), placed where its
// newest take is. Repeated changes made elsewhere merge into one { e }: the newest, with the
// oldest's prev, and merged = count. A folder shows if any take passes the filters (f.shown;
// f.entries keeps all, for Undo all). Several calls per redraw, so the answer is kept
// (rowsCache).
let rowsCache = null;
for (const ev of ['history', 'filters', 'search', 'names', 'hello'])
  onFirst(ev, () => {
    rowsCache = null;
  });

export function histRows(list) {
  const c = rowsCache;
  if (c && c.list === list && c.len === list.length && c.first === list[0]) return c.rows;
  const rows = buildRows(list);
  rowsCache = { list, len: list.length, first: list[0], rows };
  return rows;
}

function buildRows(list) {
  const rows = [];
  const byGroup = new Map();
  const chains = new Map();
  // Follows go with the take they followed (e.follows) when loaded and shown; one whose take is
  // hidden stands alone if it passes the filters itself.
  const byId = new Map(list.map((e) => [e.id, e]));
  const followsOf = new Map();
  for (const e of list) {
    if (e.follow == null || !byId.has(e.follow)) continue;
    if (!followsOf.has(e.follow)) followsOf.set(e.follow, []);
    followsOf.get(e.follow).push(e);
  }
  for (const [id, fs] of followsOf) {
    if (!passes({ ...byId.get(id), follows: fs })) followsOf.delete(id);
  }
  const attached = new Set([...followsOf.values()].flat());
  // Followers of a multi-take confirm go in its folder's f.followers (the server doesn't say
  // which take), only when that folder is loaded and will show; otherwise they stand alone.
  const groups = new Set(list.filter((e) => e.group && !e.ext).map((e) => e.group));
  const shows = new Set(list.filter((e) => e.group && !e.ext && passes(e)).map((e) => e.group));
  const followers = new Map();
  for (const e of list) {
    if (!e.ext || !e.followGroup || !groups.has(e.followGroup)) continue;
    if (!shows.has(e.followGroup) && !(searchActive() && passes(e))) continue;
    attached.add(e);
    if (!followers.has(e.followGroup)) followers.set(e.followGroup, []);
    followers.get(e.followGroup).push(e);
  }
  for (const orig of list) {
    if (attached.has(orig)) {
      chains.delete(orig.dst);
      continue;
    }
    const e = followsOf.has(orig.id) ? { ...orig, follows: followsOf.get(orig.id) } : orig;
    // Merge into the row above if it's a change elsewhere to the same destination and levels
    // with nothing else on it between.
    const solo = !e.group || e.groupN < 2;
    const k = `${e.levels.join(',')}${e.found ? ' found' : ''}`; // found and announced don't mix
    const t = Date.parse(e.time);
    const c = chains.get(e.dst);
    if (solo && e.ext && e.follow == null && !e.followGroup && c && c.k === k && c.t - t <= MERGE_MS && !searchActive()) {
      c.list.push(orig);
      const follows = [...(c.row.e.follows || []), ...(e.follows || [])];
      c.row.e = { ...c.row.e, prev: e.prev, merged: c.list.length, chain: c.list, ...(follows.length ? { follows } : {}) };
      c.t = t;
      continue;
    }
    chains.delete(e.dst);
    if (solo) {
      if (!passes(e)) continue;
      const row = { e };
      if (e.ext && e.follow == null && !e.followGroup) chains.set(e.dst, { k, row, t, list: [orig] });
      rows.push(row);
      continue;
    }
    let f = byGroup.get(e.group);
    if (!f) {
      f = { group: e.group, n: e.groupN, entries: [], shown: [] };
      byGroup.set(e.group, f);
      rows.push({ f });
    }
    f.entries.push(e);
    if (passes(e)) f.shown.push(e);
  }
  for (const [g, fs] of followers) {
    const f = byGroup.get(g);
    if (!f) continue;
    const shown = fs.filter(passes);
    if (shown.length) f.followers = shown;
    if (shown.length && searchActive() && !f.shown.length) f.shown = f.entries;
  }
  return rows.filter((r) => !r.f || r.f.shown.length);
}

// A folder's takes arrive one by one; one still filling is "in flight" for a while, after that
// missing takes just aren't loaded.
const IN_FLIGHT_MS = 30000;

function groupStatus(f) {
  const got = f.entries.length;
  const bad = f.entries.filter((e) => !e.ok).length;
  if (got < f.n && Date.now() - Date.parse(f.entries[0].time) < IN_FLIGHT_MS) {
    return { dot: bad ? 'warn' : 'info', text: `${got}/${f.n} done…`, pulse: true };
  }
  const title = got < f.n ? `${got} of ${f.n} loaded` : '';
  if (f.entries.every((e) => e.ext))
    return extStatus(
      f.entries.every((e) => e.found),
      title,
    );
  if (!bad) return { dot: 'ok', text: 'ok', title };
  if (bad === got) return { dot: 'error', text: 'failed', title };
  return { dot: 'warn', text: `${bad}/${got} failed`, title };
}

const extStatus = (found, title = '') =>
  found
    ? { dot: 'warn', text: 'Found', title: title || 'An External the router made without announcing it, found when Sode re-read the router' }
    : { dot: 'info', text: 'External', title: title || 'A change on the router that Sode did not make' };

export function statusNode(row) {
  const st = row.f
    ? groupStatus(row.f)
    : row.e.ext && row.e.followGroup
      ? { dot: 'info', text: 'External follow', title: 'Changed along with this confirm, probably as a result of it' }
      : row.e.ext
        ? extStatus(row.e.found)
        : { dot: row.e.ok ? 'ok' : 'error', text: row.e.ok ? 'ok' : 'failed' };
  const dot = el('span', `pb-dot pb-dot--${st.dot}${st.pulse ? ' pb-dot--pulse' : ''}`);
  dot.setAttribute('aria-hidden', 'true');
  const node = el('span', 'pb-status', dot, el('span', 'sode__status-text', st.text));
  if (row.e && !row.e.ok) node.title = row.e.err || '';
  if (st.title) node.title = st.title;
  node.dataset.hint = node.title ? `${st.text}: ${node.title}` : st.text;
  return node;
}

export function groupTitle(f) {
  const srcs = [...new Set(f.entries.map((e) => e.src))];
  const dsts = [...new Set(f.entries.map((e) => e.dst))];
  const strong = (t) => el('strong', null, t);
  const src = srcs.length === 1 ? labelNode(srcItem(srcs[0], f.entries[0].srcName)) : `${srcs.length} sources`;
  const dst = dsts.length === 1 ? labelNode(dstItem(dsts[0], f.entries[0].dstName)) : `${dsts.length} destinations`;
  const kind = f.entries.every((e) => e.undo != null) ? 'undo' : f.entries.every((e) => e.retry != null) ? 'retry' : '';
  const nFollow = f.entries.reduce((n, e) => n + (e.follows?.length || 0), 0);
  const preset = f.entries.find((e) => e.preset)?.preset;
  const notes = [
    preset && `Preset: ${preset}`,
    kind,
    groupPartial(f) && `${f.shown.length} of ${f.entries.length} shown`,
    nFollow && `+${nFollow} external follow ${nFollow === 1 ? 'entry' : 'entries'}`,
  ].filter(Boolean);
  return [strong(src), ' → ', strong(dst), notes.length ? el('span', 'sode__hist-note', ` (${notes.join('; ')})`) : null].filter(Boolean);
}

export const groupPartial = (f) => !!f.shown && f.shown.length < f.entries.length;

export const groupLevels = (f) => [...new Set(f.entries.flatMap((e) => e.levels))].filter((u) => S.levelBy.has(u)).sort((a, b) => a - b);

export function groupIds(f) {
  const ids = f.entries.map((e) => e.id);
  return `#${Math.min(...ids)}–#${Math.max(...ids)}`;
}

// Ignored unless it's the page before the oldest loaded: one asked for before history was
// trimmed, cleared or reloaded would land after a gap. True if added.
export function takePage(m) {
  if (!S.histLoading || !S.hist.length || m.before !== S.hist[S.hist.length - 1].id) return false;
  S.hist.push(...m.entries);
  S.histMore = m.more;
  S.histLoading = false;
  return true;
}

function requestHistory(n = 0) {
  if (!S.histMore || S.histLoading || !S.hist.length) return false;
  S.histLoading = wsSend({ t: 'history', before: S.hist[S.hist.length - 1].id, ...(n ? { n } : {}) });
  return S.histLoading;
}

const AUTO_PAGES = 5;
const SEARCH_PAGES = 40;
const SEARCH_PAGE = 500;

// more() reveals the next step, fetching another page once loaded entries run out. visible:
// whether this layout is showing; the hidden one doesn't fetch.
export function historyPager(step, visible = () => true) {
  const cap = () => (searchActive() ? SEARCH_PAGES : AUTO_PAGES);
  const pg = {
    shown: step,
    auto: 0,
    reset() {
      pg.shown = step;
      pg.auto = 0;
    },
    list() {
      const rows = histRows(S.hist);
      const s = searchActive();
      if (visible() && rows.length < pg.shown && pg.auto < cap() && requestHistory(s ? SEARCH_PAGE : 0)) pg.auto++;
      return rows.slice(0, pg.shown);
    },
    fetching: () => S.histLoading || (visible() && S.histMore && histRows(S.hist).length < pg.shown && pg.auto < cap()),
    hasMore: () => histRows(S.hist).length > pg.shown || S.histMore,
    more() {
      pg.shown += step;
      pg.auto = 0;
      if (histRows(S.hist).length < pg.shown) requestHistory();
      emit('history');
    },
    button() {
      if (!S.hist.length || !pg.hasMore()) return null;
      const b = el('button', 'pb-btn pb-btn--outline', S.histLoading ? 'Loading…' : 'Load more');
      b.type = 'button';
      b.disabled = S.histLoading;
      b.addEventListener('click', pg.more);
      return b;
    },
  };
  on('filters', () => {
    pg.auto = 0;
  }); // a new filter may need older pages
  on('search', () => {
    pg.auto = 0;
  });
  return pg;
}
export function attachFollows(e) {
  const follows = S.hist.filter((x) => x.follow === e.id);
  return follows.length ? { ...e, follows } : e;
}

// One entry per source in order of first level: [{src, levels}]. src -1 = nothing routed, null
// = not known at the time.
export function beforeGroups(e) {
  const prev = new Map(e.prev);
  const groups = new Map();
  for (const u of [...e.levels].sort((a, b) => a - b)) {
    if (!S.levelBy.has(u)) continue;
    const s = prev.has(u) ? prev.get(u) : null;
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  return [...groups].map(([src, levels]) => ({ src, levels }));
}

// [{levels, text}]: "still on X" (it ignored the take), "on X" (another route got there) or "no
// answer" (may or may not have switched; also an unsent level). Empty for a take that worked
// and for entries recorded before this existed.
export function failureGroups(e) {
  if (e.ok || !e.failed?.length) return [];
  const now = new Map(e.now || []),
    prev = new Map(e.prev);
  const groups = new Map();
  for (const u of [...e.failed].sort((a, b) => a - b)) {
    if (!S.levelBy.has(u)) continue;
    const text = !now.has(u) ? 'no answer' : `${prev.get(u) === now.get(u) ? 'still on' : 'on'} ${tallyText(now.get(u))}`;
    if (!groups.has(text)) groups.set(text, []);
    groups.get(text).push(u);
  }
  return [...groups].map(([text, levels]) => ({ levels, text }));
}

// A known id (a failed take the server corrected) is replaced in place; a new one goes first.
// S.hist gets a new array either way (the filter cache compares by identity). An update for an
// entry that isn't loaded is skipped: it comes with its page. Returns 'new', 'replaced' or
// 'skipped'.
export function putEntry(e, hist = S, update = false) {
  const i = hist.hist.findIndex((x) => x.id === e.id);
  if (i < 0) {
    if (update) return 'skipped';
    hist.hist = [e, ...hist.hist];
    return 'new';
  }
  hist.hist = hist.hist.slice();
  hist.hist[i] = e;
  return 'replaced';
}

export function entryNote(e) {
  const parts = [];
  if (e.preset && !(e.group && e.groupN > 1)) parts.push(`Preset: ${e.preset}`);
  if (e.merged > 1) parts.push(`${e.merged} changes`);
  if (e.follows?.length) parts.push(`+${e.follows.length} external follow ${e.follows.length === 1 ? 'entry' : 'entries'}`);
  if (e.follow != null) parts.push(`external follow of #${e.follow}`);
  if (e.followGroup) parts.push('external follow of this confirm');
  if (e.undo != null) parts.push(`undo of #${e.undo}`);
  if (e.retry != null) parts.push(`retry of #${e.retry}`);
  if (e.already?.length) parts.push(`${levelText(e.already)} already`);
  if (e.late?.length) parts.push(`level${e.late.length === 1 ? '' : 's'} ${levelText(e.late)} confirmed late`);
  const why = failureGroups(e);
  if (why.length) parts.push(why.map((g) => `level${g.levels.length === 1 ? '' : 's'} ${levelText(g.levels)} ${g.text}`).join('; '));
  else if (!e.ok && !e.ext && e.err) parts.push(e.err);
  return parts.length ? el('span', 'sode__hist-note', ` (${parts.join('; ')})`) : null;
}

onMsg('recent', (m) => {
  const how = putEntry(m.e, S, m.update);
  if (how === 'new') S.newId = m.e.id; // views animate a new one in, once, not a correction
  if (how !== 'skipped') emit('history');
});
onMsg('historyCleared', () => {
  S.hist = [];
  S.histMore = false;
  S.histLoading = false;
  emit('history');
  emit('historyCleared');
});
onMsg('history', (m) => {
  if (takePage(m)) emit('history');
});
