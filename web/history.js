// History as the views show it: rows and folders built from S.hist, the
// quick search above Recent, pages, and the notes and statuses of entries.
// Owns the rows memo (histRows), dropping it when anything it reads changes.

import { el, S, on, onFirst, onMsg, emit, off, wsSend, srcItem, dstItem, labelNode, tallyText } from './core.js';
import { levelText } from './levelview.js';
import { filterFn } from './filter.js';
import { filterAllows } from './filterstore.js';
import { pref } from './device.js';

// ---- quick search: source and destination boxes above Recent, for
// auditing. Not kept. It reads back through the whole log and, unless this
// browser's setting says otherwise, ignores the filters.

let query = { src: '', dst: '' };
const anything = () => true;
let searchSrc = anything, searchDst = anything;

export const searchState = () => ({ ...query });
export const searchActive = () => !!(query.src.trim() || query.dst.trim());
// Errors per box for text that doesn't parse (the last good one applies).
export function setSearch(next) {
  const was = searchActive();
  query = { src: String(next.src ?? ''), dst: String(next.dst ?? '') };
  if (was && !searchActive()) trimHist();
  searchSrc = filterFn(query.src, { fuzzy: true }) || searchSrc;
  searchDst = filterFn(query.dst, { fuzzy: true }) || searchDst;
  emit('search');
  emit('history');
}
// pref() is a lookup, not a storage read: it's asked for every entry while filtering.
export const searchRespectsFilters = () => pref('searchFilters') === 'on';

// A search reads far back through the log; once it's cleared, drop all
// but the newest entries again (whole groups), so Recent doesn't keep
// reworking thousands of them.
// A page still on its way is for before the trim, so it will be ignored
// (takePage) and another can be asked for.
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

// Whether a search finds e: its route, or one of its follows'. Names are
// the router's now and the ones logged with the entry, which differ when
// names changed (or another router is connected).
function found(e) {
  const either = (fn, n, now, then) => fn(n + off(), now) || (!!then && then !== now && fn(n + off(), then));
  const hit = (x) => either(searchSrc, x.src, srcItem(x.src, x.srcName).name, x.srcName) &&
    either(searchDst, x.dst, dstItem(x.dst, x.dstName).name, x.dstName);
  return hit(e) || !!e.follows?.some(hit);
}

function passes(e) {
  if (searchActive()) {
    if (!found(e)) return false;
    if (!searchRespectsFilters()) return true;
  }
  return filterAllows(e);
}

// Changes made elsewhere to one destination on the same levels, each
// within this long of the next, show as one row (a monitor punched about).
const MERGE_MS = 30000;

// History as rows, newest first: { e } for a take on its own, or { f } for
// a folder holding the takes of one multi-take confirm. A folder sits
// where its newest take is. Repeated changes made elsewhere are merged
// into one { e }: the newest, with the oldest's prev (so Before and Undo
// go back to before the first) and merged = how many. Entries this
// browser's filters hide are left out; a folder shows if any of its takes
// pass, with those in f.shown (f.entries keeps them all, for Undo all).
// histRows is asked for several times per redraw (both pagers, the
// search count, keyboard moves), so its answer is kept (rowsCache) until
// history, filters, search or names change.
let rowsCache = null;
// First in line, so a view that redraws on the same event reads fresh rows.
for (const ev of ['history', 'filters', 'search', 'names', 'hello']) onFirst(ev, () => { rowsCache = null; });

export function histRows(list) {
  const c = rowsCache;
  if (c && c.list === list && c.len === list.length && c.first === list[0]) return c.rows;
  const rows = buildRows(list);
  rowsCache = { list, len: list.length, first: list[0], rows };
  return rows;
}

function buildRows(list) {
  const rows = [];
  const folders = new Map();
  const chains = new Map(); // dst → { k: levels, row, t: oldest time, list: entries merged }
  // Follows go with the take they followed, as e.follows, when it's loaded
  // and shown. One whose take the filters hide stands on its own, if it
  // passes them itself.
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
  // Followers of a multi-take confirm (followGroup) go in its folder's
  // f.followers, not on any one take: the server doesn't say which. Only
  // when the folder is loaded; otherwise they stay rows of their own.
  // And only when that folder will show (a take of it passes the filters, or
  // a search matches the follower): otherwise a follower that passes
  // stands on its own, as a follow does when its take is hidden.
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
    // Merge into the row above if it's a change made elsewhere to the same
    // destination and levels, with nothing else on that destination between.
    // chain keeps what was merged, newest first.
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
    let f = folders.get(e.group);
    if (!f) {
      f = { group: e.group, n: e.groupN, entries: [], shown: [] };
      folders.set(e.group, f);
      rows.push({ f });
    }
    f.entries.push(e);
    if (passes(e)) f.shown.push(e);
  }
  for (const [g, fs] of followers) {
    const f = folders.get(g);
    if (!f) continue;
    const shown = fs.filter(passes);
    if (shown.length) f.followers = shown;
    // A search that only a follower matches still shows the confirm.
    if (shown.length && searchActive() && !f.shown.length) f.shown = f.entries;
  }
  return rows.filter((r) => !r.f || r.f.shown.length);
}

// A folder's takes arrive one by one as each finishes; one still filling
// is "in flight" for a while, after that missing takes just aren't loaded.
const IN_FLIGHT_MS = 30000;

// Master status of a folder: { dot, text, pulse, title }.
export function folderStatus(f) {
  const got = f.entries.length;
  const bad = f.entries.filter((e) => !e.ok).length;
  if (got < f.n && Date.now() - Date.parse(f.entries[0].time) < IN_FLIGHT_MS) {
    return { dot: bad ? 'warn' : 'info', text: `${got}/${f.n} done…`, pulse: true };
  }
  const title = got < f.n ? `${got} of ${f.n} loaded` : '';
  if (f.entries.every((e) => e.ext)) return extStatus(f.entries.every((e) => e.found), title);
  if (!bad) return { dot: 'ok', text: 'ok', title };
  if (bad === got) return { dot: 'error', text: 'failed', title };
  return { dot: 'warn', text: `${bad}/${got} failed`, title };
}

// Status of a change made elsewhere: announced by the router, or found
// changed when Sode re-read it.
const extStatus = (found, title = '') => (found
  ? { dot: 'warn', text: 'found', title: title || 'Changed without notice; found when Sode re-read the router' }
  : { dot: 'info', text: 'panel', title: title || 'From another panel' });

// A pb-status node for a single take or a folder.
export function statusNode(row) {
  const st = row.f ? folderStatus(row.f)
    : row.e.ext && row.e.followGroup ? { dot: 'info', text: 'follow', title: 'Changed along with this confirm' }
    : row.e.ext ? extStatus(row.e.found)
    : { dot: row.e.ok ? 'ok' : 'error', text: row.e.ok ? 'ok' : 'failed' };
  const dot = el('span', `pb-dot pb-dot--${st.dot}${st.pulse ? ' pb-dot--pulse' : ''}`);
  dot.setAttribute('aria-hidden', 'true');
  const node = el('span', 'pb-status', dot, el('span', 'sode__status-text', st.text));
  if (row.e && !row.e.ok) node.title = row.e.err || '';
  if (st.title) node.title = st.title;
  node.dataset.hint = node.title ? `${st.text}: ${node.title}` : st.text;
  return node;
}

// "CAM 1 → 5 destinations", as nodes, with a note for undos and retries.
export function folderTitle(f) {
  const srcs = [...new Set(f.entries.map((e) => e.src))];
  const dsts = [...new Set(f.entries.map((e) => e.dst))];
  const strong = (t) => el('strong', null, t);
  const src = srcs.length === 1 ? labelNode(srcItem(srcs[0], f.entries[0].srcName)) : `${srcs.length} sources`;
  const dst = dsts.length === 1 ? labelNode(dstItem(dsts[0], f.entries[0].dstName)) : `${dsts.length} destinations`;
  const kind = f.entries.every((e) => e.undo != null) ? 'undo' : f.entries.every((e) => e.retry != null) ? 'retry' : '';
  const follows = f.entries.reduce((n, e) => n + (e.follows?.length || 0), 0);
  const notes = [kind, folderPartial(f) && `${f.shown.length} of ${f.entries.length} shown`,
    follows && `+${follows} follow${follows === 1 ? '' : 's'}`].filter(Boolean);
  return [strong(src), ' → ', strong(dst), notes.length ? el('span', 'sode__hist-note', ` (${notes.join('; ')})`) : null].filter(Boolean);
}

// Filters hide some of a folder's takes (Undo all still covers them all).
export const folderPartial = (f) => !!f.shown && f.shown.length < f.entries.length;

// Every level a folder's takes touched.
export const folderLevels = (f) => [...new Set(f.entries.flatMap((e) => e.levels))].filter((u) => S.levelBy.has(u)).sort((a, b) => a - b);

// "#4–#9", oldest to newest.
export function folderIds(f) {
  const ids = f.entries.map((e) => e.id);
  return `#${Math.min(...ids)}–#${Math.max(...ids)}`;
}

// Adds a page of history the server sent, if it's the one asked for: the
// page before the oldest entry loaded. One asked for before history was
// trimmed, cleared or reloaded would land after a gap, so it's ignored.
// True if it was added.
export function takePage(m) {
  if (!S.histLoading || !S.hist.length || m.before !== S.hist[S.hist.length - 1].id) return false;
  S.hist.push(...m.entries);
  S.histMore = m.more;
  S.histLoading = false;
  return true;
}

// Asks for the next page of history. True if a request went out.
export function requestHistory(n = 0) {
  if (!S.histMore || S.histLoading || !S.hist.length) return false;
  S.histLoading = wsSend({ t: 'history', before: S.hist[S.hist.length - 1].id, ...(n ? { n } : {}) });
  return S.histLoading;
}

// Pages list() fetches by itself to fill a short page: a few normally, and
// big ones far back while searching.
const AUTO_PAGES = 5;
const SEARCH_PAGES = 40;
const SEARCH_PAGE = 500;

// Shows history `step` rows at a time. more() reveals the next step,
// fetching another page from Sode once the loaded entries run out.
// visible: whether this pager's layout is the one showing; the hidden
// one doesn't fetch.
export function historyPager(step, visible = () => true) {
  const cap = () => (searchActive() ? SEARCH_PAGES : AUTO_PAGES);
  const pg = {
    shown: step,
    auto: 0, // pages fetched by list() since the last reset or more()
    reset() { pg.shown = step; pg.auto = 0; },
    list() {
      const rows = histRows(S.hist);
      // Hidden entries can leave a page short: fetch a few more pages to
      // fill it, then leave the rest to Load more.
      const s = searchActive();
      if (visible() && rows.length < pg.shown && pg.auto < cap() && requestHistory(s ? SEARCH_PAGE : 0)) pg.auto++;
      return rows.slice(0, pg.shown);
    },
    // Whether more is on its way without Load more being pressed.
    fetching: () => S.histLoading || (visible() && S.histMore && histRows(S.hist).length < pg.shown && pg.auto < cap()),
    hasMore: () => histRows(S.hist).length > pg.shown || S.histMore,
    more() {
      pg.shown += step;
      pg.auto = 0;
      if (histRows(S.hist).length < pg.shown) requestHistory();
      emit('history');
    },
    // The "Load more" button, or null when there's nothing more.
    button() {
      if (!S.hist.length || !pg.hasMore()) return null;
      const b = el('button', 'pb-btn pb-btn--outline', S.histLoading ? 'Loading…' : 'Load more');
      b.type = 'button';
      b.disabled = S.histLoading;
      b.addEventListener('click', pg.more);
      return b;
    },
  };
  on('filters', () => { pg.auto = 0; }); // a new filter may need older pages
  on('search', () => { pg.auto = 0; });
  return pg;
}
// e with the loaded changes that followed it, as histRows attaches them.
export function attachFollows(e) {
  const follows = S.hist.filter((x) => x.follow === e.id);
  return follows.length ? { ...e, follows } : e;
}

// What dst had on a take's levels just before it, one entry per source in
// order of first level: [{src, levels}]. src -1 = nothing routed, null =
// not known at the time.
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

// What the router reported for the levels a take never got confirmed, as
// [{levels, text}]: "still on X" (it ignored the take), "on X" (some other
// route got there) or "no answer" (it may or may not have switched; also a
// level that wasn't sent). Empty for a take that worked and for entries
// from before this was recorded.
export function failureGroups(e) {
  if (e.ok || !e.failed?.length) return [];
  const now = new Map(e.now || []), prev = new Map(e.prev);
  const groups = new Map();
  for (const u of [...e.failed].sort((a, b) => a - b)) {
    if (!S.levelBy.has(u)) continue;
    const text = !now.has(u) ? 'no answer' : `${prev.get(u) === now.get(u) ? 'still on' : 'on'} ${tallyText(now.get(u))}`;
    if (!groups.has(text)) groups.set(text, []);
    groups.get(text).push(u);
  }
  return [...groups].map(([text, levels]) => ({ levels, text }));
}

// Add a recent entry: a known id (a failed take the server corrected) is
// replaced where it stands; a new one goes first. S.hist gets a new array
// either way, as the filter candidate cache compares it by identity.
// An update (the server amending an old take) for an entry that isn't
// loaded is left out: it isn't new, and comes with its page when loaded.
// Returns 'new', 'replaced' or 'skipped'.
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

// " (undo of #3; 4–17 already)" for a history entry, or null.
export function entryNote(e) {
  const parts = [];
  if (e.merged > 1) parts.push(`${e.merged} changes`);
  if (e.follows?.length) parts.push(`+${e.follows.length} follow${e.follows.length === 1 ? '' : 's'}`);
  if (e.follow != null) parts.push(`follow of #${e.follow}`);
  if (e.followGroup) parts.push('follow of this confirm');
  if (e.undo != null) parts.push(`undo of #${e.undo}`);
  if (e.retry != null) parts.push(`retry of #${e.retry}`);
  if (e.already?.length) parts.push(`${levelText(e.already)} already`);
  if (e.late?.length) parts.push(`level${e.late.length === 1 ? '' : 's'} ${levelText(e.late)} confirmed late`);
  const why = failureGroups(e);
  if (why.length) parts.push(why.map((g) => `level${g.levels.length === 1 ? '' : 's'} ${levelText(g.levels)} ${g.text}`).join('; '));
  else if (!e.ok && !e.ext && e.err) parts.push(e.err);
  return parts.length ? el('span', 'sode__hist-note', ` (${parts.join('; ')})`) : null;
}

// Server messages for history: a new or corrected entry, a page, a clear.
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
});
onMsg('history', (m) => { if (takePage(m)) emit('history'); });
