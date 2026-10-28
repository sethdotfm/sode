// The /log page: every message Sode wrote to the router (issue 72), newest
// first, live, with this browser's filters.
import { $, el, S, on, connect, wsSend, connState, routerLabel, setDot, item, labelNode } from './core.js';
import { levelColor } from './levelview.js';
import { clockPref } from './device.js';
import { filterFn } from './filter.js';
import { readJSON, writeJSON } from './store.js';
import { normalizeKinds, allKinds, pressAll, pressKind } from './logkinds.js';

// Entries read on open and per Load older.
const PAGE = 100;
// Entries per request while reading back to the start of a time range, or
// through pages the filters hide entirely (the server's most).
const FILL = 500;
// Requests in a row for one Load older while the filters hide everything.
const AUTO_PAGES = 20;
// Entries are kept in the order they finished but stamped with when they
// were first sent (or queued), so the log isn't strictly in time order: a
// message that retried can finish after later ones. Reading back for a
// range goes this much further than its start to catch those.
const TIME_SLACK = 30_000;
const FILTER_KEY = 'sode-log-filter';

// Filter groups, in the order shown. Connects alone by default: the routes.
const KINDS = [
  { id: 'connect', label: 'Connects', kinds: ['connect'] },
  { id: 'interrogate', label: 'Interrogates', kinds: ['interrogate'] },
  { id: 'keepalive', label: 'Keepalives', kinds: ['keepalive'] },
  { id: 'dump', label: 'Dumps', kinds: ['tallyDump', 'protectDump'] },
  { id: 'names', label: 'Name requests', kinds: ['srcNames', 'dstNames'] },
  { id: 'other', label: 'Other', kinds: ['other'] },
];
const KIND_LABEL = {
  connect: 'Connect', interrogate: 'Interrogate', keepalive: 'Keepalive', tallyDump: 'Tally dump',
  protectDump: 'Protect dump', srcNames: 'Source names', dstNames: 'Destination names', other: 'Other',
};
const WHY_LABEL = {
  take: 'Take', undo: 'Undo', retry: 'Retry', check: 'Check before take', confirm: 'Confirm take',
  resync: 'Resync', names: 'Name refresh', keepalive: 'Link check',
};

// ---- filters

const KIND_IDS = KINDS.map((g) => g.id);
function loadFilter() {
  const f = readJSON(FILTER_KEY) || {};
  return {
    kinds: normalizeKinds(f.kinds, KIND_IDS, ['connect']),
    problems: !!f.problems,
    src: typeof f.src === 'string' ? f.src : '',
    dst: typeof f.dst === 'string' ? f.dst : '',
  };
}
let filter = loadFilter();
const anything = () => true;
let srcOk = filterFn(filter.src) || anything;
let dstOk = filterFn(filter.dst) || anything;

function saveFilter() {
  writeJSON(FILTER_KEY, filter);
}

const kindOn = (k) => KINDS.some((g) => g.kinds.includes(k) && filter.kinds.includes(g.id));
const problem = (e) => e.result !== 'ack' || e.tries > 1;
const off = () => (S.cfg ? S.cfg.numberOffset : 0);

// The time range, as ms (null: open). Not kept: a range far back means
// reading the log back to it, which shouldn't happen on every open.
const range = { from: null, to: null };

// An expression filters out entries without that side.
function passes(e) {
  if (range.from != null && e.t < range.from) return false;
  if (range.to != null && e.t > range.to) return false;
  if (!kindOn(KINDS.some((g) => g.kinds.includes(e.kind)) ? e.kind : 'other')) return false;
  if (filter.problems && !problem(e)) return false;
  if (filter.src.trim() && (e.src < 0 || !srcOk(e.src + off(), e.srcName || ''))) return false;
  if (filter.dst.trim() && (e.dst < 0 || !dstOk(e.dst + off(), e.dstName || ''))) return false;
  return true;
}

function initFilters() {
  const box = $('log-kinds');
  const legend = box.querySelector('legend');
  const chip = (label, on, press) => {
    const b = el('button', 'pb-chip sode-log__chip', label);
    b.type = 'button';
    b.setAttribute('aria-pressed', String(on));
    b.addEventListener('click', () => {
      press();
      saveFilter();
      drawKinds();
      render();
    });
    return b;
  };
  // All | each kind | Problems only. Problems is its own dimension, so it
  // sits after a separator and stays out of the All logic.
  const drawKinds = () => {
    const all = allKinds(filter.kinds, KIND_IDS);
    const bar = el('span', 'sode-log__sep', '|');
    bar.setAttribute('aria-hidden', 'true');
    box.replaceChildren(
      legend,
      chip('All', all, () => { filter.kinds = pressAll(KIND_IDS); }),
      ...KINDS.map((g) => chip(g.label, !all && filter.kinds.includes(g.id), () => { filter.kinds = pressKind(filter.kinds, KIND_IDS, g.id); })),
      bar,
      chip('Problems only', filter.problems, () => { filter.problems = !filter.problems; }),
    );
  };
  drawKinds();
  // datetime-local is local time. To counts to the end of what it shows:
  // 14:30 is up to 14:30:59.999.
  const from = $('log-from'), to = $('log-to');
  const parse = (input, end) => {
    if (!input.value) return null;
    const t = new Date(input.value).getTime();
    if (Number.isNaN(t)) return undefined;
    return end ? t + (input.value.length <= 16 ? 59999 : 999) : t;
  };
  const applyRange = () => {
    const f = parse(from, false), t = parse(to, true);
    const bad = f === undefined || t === undefined || (f != null && t != null && f > t);
    from.classList.toggle('pb-input--invalid', bad);
    to.classList.toggle('pb-input--invalid', bad);
    if (bad) return;
    range.from = f;
    range.to = t;
    $('log-range-clear').hidden = f == null && t == null;
    render();
    fill();
  };
  from.addEventListener('change', applyRange);
  to.addEventListener('change', applyRange);
  $('log-range-clear').addEventListener('click', () => {
    from.value = to.value = '';
    applyRange();
  });

  for (const side of ['src', 'dst']) {
    const input = $(`log-${side}`);
    input.value = filter[side];
    input.addEventListener('input', () => {
      const fn = filterFn(input.value);
      input.classList.toggle('pb-input--invalid', !fn);
      if (!fn) return; // keeps the last good one
      filter[side] = input.value;
      if (side === 'src') srcOk = fn;
      else dstOk = fn;
      saveFilter();
      render();
    });
  }
}

// ---- entries: newest first

let list = [];
const seen = new Set(); // seqs in list
// The entries in the table, in its order: kept in step with tbody's rows
// so new ones go in place instead of redrawing everything.
let shown = [];
let more = false;
let total = 0; // entries in the log on the server, as last reported
let loading = false;
let autoLeft = 0;
let started = false; // asked for the newest page on this connection

function add(entries) {
  const fresh = entries.filter((e) => !seen.has(e.seq));
  for (const e of fresh) {
    seen.add(e.seq);
    e.t = Date.parse(e.time);
  }
  return fresh;
}

function request(before, n = PAGE) {
  if (wsSend({ t: 'sent', before, n })) loading = true;
  renderMore();
}

const oldest = () => list[list.length - 1];

// Whether entries older than those loaded can still fall in the range:
// the log hasn't been read back past its start (or, with only an end,
// to its end).
function rangeNeedsMore() {
  if (!more || !list.length) return false;
  if (range.from != null) return oldest().t >= range.from - TIME_SLACK;
  if (range.to != null) return oldest().t > range.to - TIME_SLACK;
  return false;
}

// Everything in the range is loaded: older pages are all before it.
const rangeDone = () => range.from != null && list.length > 0 && !rangeNeedsMore();

// Reads back until the range is covered. One request at a time.
function fill() {
  if (!loading && rangeNeedsMore()) request(oldest().seq, FILL);
}

on('hello', () => {
  // Fresh after a reconnect: entries may have been missed while away.
  list = [];
  seen.clear();
  more = false;
  started = true;
  request(-1);
  render();
});

on('sentPage', (m) => {
  if (!started) return;
  const last = list.length ? oldest().seq : -1;
  const newest = m.before === -1;
  let older = [];
  if (newest) {
    // Live entries may have come in first: merge.
    list = [...list, ...add(m.entries)].sort((a, b) => b.seq - a.seq);
  } else if (m.before === last) {
    older = add(m.entries);
    list.push(...older);
  } else {
    return; // for a list since replaced
  }
  more = m.more;
  loading = false;
  showCounts(m);
  // Older entries go at the end newest first. In any other order the
  // table is drawn once when reading back stops, not after every page.
  if (newest) render();
  else if (newestFirst()) appendRows(older.filter(passes));
  if (rangeNeedsMore()) return fill();
  // Keep reading back while the filters hide everything read so far.
  if (more && autoLeft > 0 && !shown.length && !older.some(passes)) {
    autoLeft--;
    request(oldest().seq, FILL);
    return;
  }
  autoLeft = 0;
  if (!newest && !newestFirst()) render();
  else renderMore();
});

on('sentAdd', (m) => {
  if (!started) return;
  showCounts(m);
  const fresh = add(m.entries).sort((a, b) => b.seq - a.seq);
  if (!fresh.length) return;
  list.unshift(...fresh);
  const pass = fresh.filter(passes);
  if (newestFirst()) {
    tbody.prepend(...pass.map((e) => row(e, true)));
    shown.unshift(...pass);
  } else {
    for (const e of pass) insertRow(e);
  }
  renderMore();
});

function showCounts(m) {
  if (m.total != null) total = m.total;
  const n = m.lost || 0;
  const note = $('log-lost');
  note.hidden = !n;
  note.textContent = n ? `${n} message${n === 1 ? '' : 's'} since the last restart ${n === 1 ? 'wasn’t' : 'weren’t'} saved to this log. The server log has details.` : '';
}

// ---- rendering

const tbody = $('log-rows');

// The time with milliseconds, dated, in this browser's Clock setting.
let timeFmt = null;
function fmtStamp(iso) {
  if (!timeFmt) {
    const pref = clockPref();
    const opts = { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 };
    if (pref !== 'locale') opts.hourCycle = pref === '12' ? 'h12' : 'h23';
    timeFmt = new Intl.DateTimeFormat([], opts);
  }
  const d = new Date(iso);
  const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  return [el('span', 'sode__date', `${date} `), timeFmt.format(d)];
}

// What the router made of it: status always with text.
function outcome(e) {
  const tries = `${e.tries} ${e.tries === 1 ? 'try' : 'tries'}`;
  const [dot, text] = {
    ack: e.tries > 1 ? ['warn', `ACK after ${tries}`] : ['ok', 'ACK'],
    nak: ['error', `NAK, gave up after ${tries}`],
    noack: ['error', `No ACK after ${tries}`],
    write: ['error', 'Write failed'],
    dropped: ['error', 'Not sent: link dropped'],
    unsent: ['error', 'Not sent'],
  }[e.result] || ['idle', e.result];
  const s = el('span', 'pb-status', el('span', `pb-dot pb-dot--${dot}`), text);
  if (e.err) s.title = e.err;
  return s;
}

function levelCell(e) {
  const l = S.levelBy.get(e.level);
  if (l) return levelColor(el('span', 'sode__slot sode__slot--on sode-log__lvl', l.label), l);
  return e.wire >= 0 ? el('span', 'pb-text-muted', `wire ${e.wire}`) : '';
}

const side = (n, name) => (n >= 0 ? labelNode(item(n, name || '')) : '');

function row(e, fresh = false) {
  const why = WHY_LABEL[e.why] || e.why || '';
  const tr = el('tr', fresh ? 'sode--enter' : null,
    el('td', 'sode-log__time', ...fmtStamp(e.time)),
    el('td', null, el('span', 'pb-text-muted', `0x${e.cmd.toString(16).toUpperCase().padStart(2, '0')}`), ' ', KIND_LABEL[e.kind] || e.kind),
    el('td', null, levelCell(e)),
    el('td', 'sode-log__name', side(e.dst, e.dstName)),
    el('td', 'sode-log__name', side(e.src, e.srcName)),
    el('td', null, why, e.by ? el('span', 'pb-text-muted', ` · ${e.by}`) : ''),
    el('td', null, outcome(e)),
    el('td', 'sode-log__hex', e.hex),
  );
  if (problem(e)) tr.classList.add('sode-log__row--problem');
  return tr;
}

// ---- sorting: click a column header. Newest first by default; ties
// keep newest first, and rows without that column go last either way.

const SORT_KEY = 'sode-log-sort';
const resultRank = (e) => (e.result === 'ack' ? (e.tries > 1 ? 1 : 0) : 2);
const SORTS = {
  time: (e) => e.seq,
  message: (e) => KIND_LABEL[e.kind] || e.kind,
  level: (e) => (e.level ? e.level : e.wire >= 0 ? 1000 + e.wire : null),
  dst: (e) => (e.dst >= 0 ? e.dst : null),
  src: (e) => (e.src >= 0 ? e.src : null),
  why: (e) => (e.why ? `${WHY_LABEL[e.why] || e.why} ${e.by || ''}` : null),
  result: (e) => [resultRank(e), e.result, e.tries],
  hex: (e) => e.hex,
};
let sort = { col: 'time', dir: 'desc' };
{
  const saved = readJSON(SORT_KEY);
  if (SORTS[saved?.col] && ['asc', 'desc'].includes(saved.dir)) sort = saved;
}
const newestFirst = () => sort.col === 'time' && sort.dir === 'desc';

function cmp(a, b) {
  if (Array.isArray(a)) {
    for (let i = 0; i < a.length; i++) {
      const c = cmp(a[i], b[i]);
      if (c) return c;
    }
    return 0;
  }
  if (typeof a === 'string') return a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' });
  return a - b;
}

// Sort order of two entries given their sort keys (xv, yv).
function cmpKeyed(x, xv, y, yv) {
  if (xv == null || yv == null) return xv == null ? (yv == null ? y.seq - x.seq : 1) : -1;
  return (sort.dir === 'asc' ? 1 : -1) * cmp(xv, yv) || y.seq - x.seq;
}

// Where one entry goes relative to another in the table.
function order(x, y) {
  if (newestFirst()) return y.seq - x.seq;
  const key = SORTS[sort.col];
  return cmpKeyed(x, key(x), y, key(y));
}

// The rows shown, in the order shown: also what the CSV holds.
function shownRows() {
  const rows = list.filter(passes);
  if (newestFirst()) return rows;
  const key = SORTS[sort.col];
  return rows
    .map((e) => ({ e, v: key(e) }))
    .sort((x, y) => cmpKeyed(x.e, x.v, y.e, y.v))
    .map((x) => x.e);
}

// Older entries, after everything shown (newest first only).
function appendRows(es) {
  tbody.append(...es.map((e) => row(e)));
  shown.push(...es);
}

// A new entry, in its place in the current order.
function insertRow(e) {
  let lo = 0, hi = shown.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (order(shown[mid], e) <= 0) lo = mid + 1;
    else hi = mid;
  }
  tbody.insertBefore(row(e, true), tbody.children[lo] || null);
  shown.splice(lo, 0, e);
}

function renderSort() {
  for (const th of document.querySelectorAll('th[data-sort]')) {
    th.setAttribute('aria-sort', th.dataset.sort !== sort.col ? 'none' : sort.dir === 'asc' ? 'ascending' : 'descending');
  }
}

// The same column flips; a new one starts newest first for time and
// A→Z / lowest first for the rest.
function setSort(col) {
  sort = sort.col === col ? { col, dir: sort.dir === 'asc' ? 'desc' : 'asc' } : { col, dir: col === 'time' ? 'desc' : 'asc' };
  writeJSON(SORT_KEY, newestFirst() ? null : sort);
  renderSort();
  render();
}

for (const th of document.querySelectorAll('th[data-sort]')) {
  th.tabIndex = 0;
  th.addEventListener('click', () => setSort(th.dataset.sort));
  th.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    e.preventDefault();
    setSort(th.dataset.sort);
  });
}
renderSort();

function render() {
  shown = shownRows();
  tbody.replaceChildren(...shown.map((e) => row(e)));
  renderMore();
}

function renderMore() {
  const shown = tbody.childElementCount;
  $('log-count').textContent = list.length ? `${shown.toLocaleString()} of ${total.toLocaleString()} in log` : '';
  $('log-csv').disabled = !shown;
  const older = $('log-older');
  const filling = loading && rangeNeedsMore();
  older.hidden = !more || rangeDone();
  older.disabled = loading;
  older.textContent = filling ? `Loading back to ${fmtDay(oldest().t)}…` : loading ? 'Loading…' : 'Load older';
  const empty = $('log-empty');
  empty.hidden = shown > 0 || loading;
  empty.textContent = !list.length
    ? (more ? '' : 'Nothing sent yet.')
    : rangeDone() || !more ? 'Nothing matches the filters.' : 'Nothing loaded matches the filters.';
}

// ---- CSV: the rows shown, newest first, as logged (numbers as displayed).

const CSV_COLS = [
  ['time', (e) => e.time],
  ['seq', (e) => e.seq],
  ['message', (e) => KIND_LABEL[e.kind] || e.kind],
  ['cmd', (e) => `0x${e.cmd.toString(16).toUpperCase().padStart(2, '0')}`],
  ['level', (e) => S.levelBy.get(e.level)?.label ?? ''],
  ['wire level', (e) => (e.wire >= 0 ? e.wire : '')],
  ['destination', (e) => (e.dst >= 0 ? e.dst + off() : '')],
  ['destination name', (e) => e.dstName || ''],
  ['source', (e) => (e.src >= 0 ? e.src + off() : '')],
  ['source name', (e) => e.srcName || ''],
  ['for', (e) => WHY_LABEL[e.why] || e.why || ''],
  ['by', (e) => e.by || ''],
  ['result', (e) => e.result],
  ['tries', (e) => e.tries],
  ['error', (e) => e.err || ''],
  ['bytes', (e) => e.hex],
];

// Quoted where needed; a leading = + - @ is defused so a spreadsheet
// doesn't run a router-supplied name as a formula.
function csvCell(v) {
  let s = String(v);
  if (/^[=+\-@\t\r]/.test(s) && !/^-?\d+(\.\d+)?$/.test(s)) s = `'${s}`;
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

$('log-csv').addEventListener('click', () => {
  const rows = shownRows();
  const text = [CSV_COLS.map(([h]) => h), ...rows.map((e) => CSV_COLS.map(([, f]) => f(e)))]
    .map((r) => r.map(csvCell).join(',')).join('\r\n') + '\r\n';
  const url = URL.createObjectURL(new Blob([text], { type: 'text/csv' }));
  const d = new Date();
  const stamp = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}-${String(d.getHours()).padStart(2, '0')}${String(d.getMinutes()).padStart(2, '0')}`;
  const a = el('a');
  a.href = url;
  a.download = `sode-sent-${stamp}.csv`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});

$('log-older').addEventListener('click', () => {
  if (loading || !list.length) return;
  autoLeft = AUTO_PAGES;
  request(oldest().seq);
});

// Where a range fill has got to.
const fmtDay = (t) => {
  const d = new Date(t);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

// ---- connection

on('conn', () => {
  const c = connState();
  setDot($('log-ws-dot'), $('log-ws-text'), [c.ws[0], 'Server']);
  setDot($('log-rt-dot'), $('log-rt-text'), [c.rt[0], 'Router']);
  $('log-addr').textContent = routerLabel()[0];
  if (S.wsState !== 'up') { started = false; loading = false; renderMore(); }
});
initFilters();
connect();
