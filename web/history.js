// History as the views show it: rows and Groups from S.hist. Owns the rows memo (histRows),
// dropped when anything it reads changes.

import { el, S, on, onFirst, onMsg, emit, off, wsSend, srcItem, dstItem, tallyText, routerData } from './core.js';
import { levelText, levelsPhrase, oneLevel } from './levelview.js';
import { filterFn } from './filter.js';
import { filterAllows } from './filterstore.js';
import { pref } from './device.js';
import { entryChip, protectFoldOpts } from './protectrules.js';
import { mergeRuns } from './historyruns.js';
import { undoIndex, markFor, groupMark, takeMark, takeLevels } from './undone.js';
import { statusText } from './statustext.js';
import { isHop, typedEnds, soloTies, hopEntriesOf, stackVia } from './tiehistoryrules.js';
import { forgetSteps } from './stepback.js';

// Quick search above History. It reads back through the whole log. It ignores the filters unless this browser's setting says otherwise. Not kept.

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

// After a search is cleared, drop all but the newest entries again (whole groups). A page still
// on its way is for before the trim, so takePage ignores it.
const KEEP_AFTER_SEARCH = 200;
// Live takes add entries with no end. Past this many, and while no search or pager reads past KEEP_AFTER_SEARCH,
// History drops back to the newest, as after a search.
const HIST_CAP = 1000;
const pagers = new Set();
const trimDue = () => S.hist.length > HIST_CAP && !searchActive() && [...pagers].every((p) => p.shown <= KEEP_AFTER_SEARCH);
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
// changed (or another router is connected). A tie take matches by its typed ends, and a hop entry never matches.
function found(e) {
  const either = (fn, n, now, then) => fn(n + off(), now) || (!!then && then !== now && fn(n + off(), then));
  const hit = (x) => {
    const ends = typedEnds(x);
    if (!ends) return false;
    const { src, dst } = ends;
    // A Protect entry has no source: the Source search matches its chip words (Protect, Router lock) instead.
    const srcHit = x.protect ? searchSrc(-1, entryChip(x).text) : either(searchSrc, src.n, srcItem(src.n, src.name, src.router).name, src.name);
    return srcHit && either(searchDst, dst.n, dstItem(dst.n, dst.name, dst.router).name, dst.name);
  };
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

// Changes to one destination on the same levels show as one stacked row when each comes at most
// this long after the one before.
// The gap is between neighbours, not the whole span. 0 is off. Keep in step with docs/agents/history.md.
export const stackGapMs = () => Number(pref('stackGap')) * 1000;

// A change that can join a stacked row: one route on its own, from any browser, panel or the
// router itself. A failed take stays apart for its Retry; a follow belongs to its take. An undo
// (also a Restore) stays apart, or it would join the row that it undid.
const stacks = (e) => alone(e) && e.ok && e.follow == null && !e.followGroup && e.undo == null && !e.protect;

// An entry with a row of its own: no Group, or the one destination of a tie take (tieSolo, soloTies).
const alone = (e) => !e.group || e.groupN < 2 || !!e.tieSolo;

// { e } for an entry on its own, { f } for a Group (a take that sent several routes), placed where its newest entry is.
// Rapid changes to one destination merge into one { e }: the newest, with the oldest's prev, and merged = count.
// A Group shows if any entry passes the filters (f.shown; f.entries keeps all, for Undo all). A redraw calls this several times, so rowsCache keeps the answer.
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

// No search stacks or folds.
function buildRows(list) {
  const gap = searchActive() ? 0 : stackGapMs();
  // External follow entries go with the take they probably followed (e.follows) when it is loaded and shown. One whose take is
  // hidden stands alone if it passes the filters itself.
  const byId = new Map(list.map((e) => [e.id, e]));
  // Tie takes: the hop entries go in the row of their destination entry (hopEntries). A tie take of one destination
  // is that row alone (soloTies), so it can stack. A take on its way can have no destination entry loaded yet. Until
  // that entry comes, its Group shows its hops, judged by their own ends.
  const solos = soloTies(list);
  const groupOf = new Map();
  for (const e of list) {
    if (!e.group || e.ext) continue;
    if (!groupOf.has(e.group)) groupOf.set(e.group, []);
    groupOf.get(e.group).push(e);
  }
  const asked = new Set(list.filter((e) => e.group && !e.ext && !isHop(e)).map((e) => e.group));
  const entryShows = (e) => (!isHop(e) ? passes(e) : !asked.has(e.group) && passes({ ...e, tie: '' }));
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
  // External follow entries of a Group go in f.followers (the server does not say which entry they followed).
  // That occurs only when the Group is loaded and will show; otherwise they stand alone.
  const groups = new Set(list.filter((e) => e.group && !e.ext && !solos.has(e.group)).map((e) => e.group));
  const shows = new Set(list.filter((e) => e.group && !e.ext && entryShows(e)).map((e) => e.group));
  const followers = new Map();
  for (const e of list) {
    if (!e.ext || !e.followGroup || !groups.has(e.followGroup)) continue;
    if (!shows.has(e.followGroup) && !(searchActive() && passes(e))) continue;
    attached.add(e);
    if (!followers.has(e.followGroup)) followers.set(e.followGroup, []);
    followers.get(e.followGroup).push(e);
  }
  // Each entry first stands alone, because a hidden entry, a Group's entry and an attached follow end a stack. Then it
  // goes to its Group or its take.
  for (const e of list) if (isHop(e) && solos.has(e.group)) attached.add(e);
  // The Undone mark reads every loaded entry, so a hidden undo row still marks its take (undone.js).
  const undone = undoIndex(list);
  const items = list.map((orig) => {
    const mark = orig.protect ? null : takeMark(orig, undone);
    let e =
      followsOf.has(orig.id) || mark
        ? { ...orig, ...(followsOf.has(orig.id) ? { follows: followsOf.get(orig.id) } : {}), ...(mark ? { undone: mark } : {}) }
        : orig;
    if (orig.origin && groupOf.has(orig.group)) {
      const hopEntries = hopEntriesOf(orig, groupOf.get(orig.group));
      // An undone tie take: its Undo left the tie lines as they were, unless a put-back undid a hop too.
      e = { ...e, hopEntries, ...(mark ? { tiesKept: !hopEntries.some((h) => undone.has(h.id)) } : {}) };
    }
    if (solos.get(orig.group) === orig.id) e = { ...e, tieSolo: true };
    const t = Date.parse(e.time);
    return { e, orig, span: [t, t], attached: attached.has(orig), shows: !attached.has(orig) && entryShows(e) };
  });
  const stacked = mergeRuns(items, {
    lane: (r) => r.e.dst,
    // Found and announced do not mix; a tie row stacks only with a row through the same tie lines.
    key: (r) => (stacks(r.e) ? `${r.e.levels.join(',')}${r.e.found ? ' found' : ''}${stackVia(r.e)}` : ''),
    gapMs: gap,
    span: (r) => r.span,
    join: (run) => stackRow(run, undone),
  });
  const rows = [];
  const byGroup = new Map();
  for (const r of stacked) {
    if (r.attached) continue;
    if (alone(r.e)) {
      rows.push({ e: r.e, shows: r.shows });
      continue;
    }
    let row = byGroup.get(r.e.group);
    if (!row) {
      row = { f: { group: r.e.group, n: r.e.groupN, entries: [], shown: [] } };
      byGroup.set(r.e.group, row);
      rows.push(row);
    }
    row.f.entries.push(r.e);
    if (r.shows) row.f.shown.push(r.e);
  }
  for (const [g, fs] of followers) {
    const f = byGroup.get(g)?.f;
    if (!f) continue;
    const shown = fs.filter(passes);
    if (shown.length) f.followers = shown;
    if (shown.length && searchActive() && !f.shown.length) f.shown = groupRowEntries(f);
  }
  for (const row of byGroup.values()) {
    row.shows = row.f.shown.length > 0;
    // Hop entries and the Protect of a tie take are no routes asked for, as External follows: they do not count.
    const mark = groupMark(row.f.entries.filter((e) => !isHop(e) && !e.protect));
    if (mark) row.f.undone = mark;
  }
  return mergeRuns(rows, protectFoldOpts(gap)).filter((r) => r.shows);
}

// A stacked row: the newest change, with the oldest's prev, merged = count and chain = each change, newest first.
function stackRow(run, undone) {
  const [top] = run;
  const follows = run.flatMap((r) => r.e.follows || []);
  const mark = markFor(
    run.map((r) => r.orig.id),
    takeLevels(top.e),
    undone,
  );
  const e = { ...top.e, prev: run[run.length - 1].e.prev, merged: run.length, chain: run.map((r) => r.orig), ...(follows.length ? { follows } : {}) };
  delete e.undone;
  if (mark) e.undone = mark;
  return { ...top, e };
}

// A Group's entries arrive one at a time; a Group that still fills is "in flight" for a while. After that,
// missing entries count as not loaded.
const IN_FLIGHT_MS = 30000;

function groupStatus(f) {
  const got = f.entries.length;
  const bad = f.entries.filter((e) => !e.ok).length;
  if (got < f.n && Date.now() - Date.parse(f.entries[0].time) < IN_FLIGHT_MS) {
    return { tone: bad ? 'warn' : 'info', text: `${got}/${f.n} done…` };
  }
  const title = got < f.n ? `${got} of ${f.n} loaded` : '';
  if (f.entries.every((e) => e.ext))
    return extStatus(
      f.entries.every((e) => e.found),
      title,
    );
  if (!bad) return { tone: 'ok', text: 'ok', title };
  if (bad === got) return { tone: 'error', text: 'failed', title };
  return { tone: 'warn', text: `${bad}/${got} failed`, title };
}

const extStatus = (found, title = '') =>
  found
    ? { tone: 'warn', text: 'Found', title: title || 'External with no announcement. Sode found it at a re-read.' }
    : { tone: 'info', text: 'External', title: title || 'A change on the router that Sode did not make' };

export function statusNode(row) {
  const st = row.f
    ? groupStatus(row.f)
    : row.e.ext && row.e.followGroup
      ? { tone: 'info', text: 'External follow', title: 'Changed with this take, probably because of it' }
      : row.e.ext
        ? extStatus(row.e.found)
        : { tone: row.e.ok ? 'ok' : 'error', text: row.e.ok ? 'ok' : 'failed' };
  const node = statusText(st.tone, el('span', 'sode__status-text', st.text));
  if (row.e && !row.e.ok) node.title = row.e.err || '';
  if (st.title) node.title = st.title;
  node.dataset.hint = node.title ? `${st.text}: ${node.title}` : st.text;
  return node;
}

// The words in brackets after a row's route (rowView's route notes); null for none.
export const routeNote = (notes) => (notes.length ? el('span', 'sode__hist-note', ` (${notes.join('; ')})`) : null);

// The entries a Group can show as rows: hop entries go in the row of their destination entry.
export const groupRowEntries = (f) => {
  const es = f.entries.filter((e) => !isHop(e));
  return es.length ? es : f.entries;
};
export const groupPartial = (f) => !!f.shown && f.shown.length < groupRowEntries(f).length;

// The router of a Group whose entries are all on one router, else blank (the first router).
export const groupRouter = (f) => {
  const r = f.entries[0]?.router || '';
  return f.entries.every((e) => (e.router || '') === r) ? r : '';
};
export const groupLevels = (f) =>
  [...new Set(f.entries.flatMap((e) => e.levels))].filter((u) => routerData(groupRouter(f)).levelBy.has(u)).sort((a, b) => a - b);

export function groupIds(f) {
  const ids = f.entries.map((e) => e.id);
  return `#${Math.min(...ids)}–#${Math.max(...ids)}`;
}

// Ignored unless it is the page before the oldest loaded: one asked for before history was
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

// more() reveals the next step, and fetches another page when loaded entries run out. visible:
// whether this layout is showing; the hidden one does not fetch.
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
  }); // it is possible that a new filter needs older pages
  on('search', () => {
    pg.auto = 0;
  });
  pagers.add(pg);
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
    if (!routerData('').levelBy.has(u)) continue;
    const s = prev.has(u) ? prev.get(u) : null;
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  return [...groups].map(([src, levels]) => ({ src, levels }));
}

// A stacked row's source before its first change: {src, text}. src is null when the levels had
// several sources, and text then counts them.
export function stackBefore(e) {
  const g = beforeGroups(e);
  return g.length === 1 ? { src: g[0].src, text: tallyText(g[0].src) } : { src: null, text: `${g.length} sources` };
}

// [{levels, text}]: "still on X" (it ignored the take), "on X" (another route got there) or "no
// answer" (it is not known if it switched; also an unsent level). Empty for a take that worked
// and for entries recorded before this existed.
export function failureGroups(e) {
  if (e.ok || !e.failed?.length) return [];
  const now = new Map(e.now || []),
    prev = new Map(e.prev);
  const groups = new Map();
  for (const u of [...e.failed].sort((a, b) => a - b)) {
    if (!routerData('').levelBy.has(u)) continue;
    const text = !now.has(u) ? 'no answer' : `${prev.get(u) === now.get(u) ? 'still on' : 'on'} ${tallyText(now.get(u))}`;
    if (!groups.has(text)) groups.set(text, []);
    groups.get(text).push(u);
  }
  return [...groups].map(([text, levels]) => ({ levels, text }));
}

// A known id (a failed take the server corrected) is replaced in place; a new one goes first.
// S.hist gets a new array either way (the filter cache compares by identity). An update for an
// entry that is not loaded is skipped: it comes with its page. Returns 'new', 'replaced' or
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

// The Undone mark's badge (words, not color alone); null for no mark. m is a row's mark or a Group's.
export const undoneBadge = (m) => (m ? el('span', 'pb-badge sode__status-badge', m.text) : null);

const undoneNote = (m) => {
  if (!m.by) return '';
  const by = `undone by ${m.by.map((id) => `#${id}`).join(', ')}`;
  return m.state === 'partly' ? `level${m.levels.length === 1 ? '' : 's'} ${levelText(m.levels)} ${by}` : by;
};

export function entryNote(e) {
  const parts = [];
  if (e.preset && !(e.group && e.groupN > 1)) parts.push(`Preset: ${e.preset}`);
  if (e.merged > 1) parts.push(`${e.merged} changes`);
  if (e.follows?.length) parts.push(`+${e.follows.length} External follow ${e.follows.length === 1 ? 'entry' : 'entries'}`);
  if (e.follow != null) parts.push(`External follow of #${e.follow}`);
  if (e.followGroup) parts.push('External follow of this take');
  if (e.protect && e.ext) parts.push(`device ${e.device ?? 0}`);
  if (e.undo != null) parts.push(`undo of #${e.undo}`);
  if (e.undone) parts.push(undoneNote(e.undone));
  if (e.undone && e.tiesKept) parts.push('tie lines left as they are');
  if (e.retry != null) parts.push(`retry of #${e.retry}`);
  if (e.already?.length) parts.push(`${oneLevel(e.router) ? 'all levels' : levelText(e.already, e.router)} already`);
  if (e.late?.length) parts.push(`${levelsPhrase(e.late, e.router)} confirmed late`);
  const why = failureGroups(e);
  if (why.length) parts.push(why.map((g) => `${levelsPhrase(g.levels, e.router)} ${g.text}`).join('; '));
  else if (!e.ok && !e.ext && e.err) parts.push(e.err);
  return parts.length ? el('span', 'sode__hist-note', ` (${parts.join('; ')})`) : null;
}

export function liveEntry(m) {
  const how = putEntry(m.e, S, m.update);
  if (how === 'new') S.newId = m.e.id; // views animate a new one in, one time, not a correction
  if (how === 'new' && trimDue()) trimHist();
  if (how !== 'skipped') emit('history');
}
onMsg('entry', liveEntry);
onMsg('historyCleared', () => {
  forgetSteps();
  S.hist = [];
  S.histMore = false;
  S.histLoading = false;
  emit('history');
  emit('historyCleared');
});
onMsg('history', (m) => {
  if (takePage(m)) emit('history');
});
