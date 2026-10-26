// The /log page: every message Sode wrote to the router (issue 72), newest
// first, live, with this browser's filters.
import { $, el, S, on, connect, wsSend, connState, routerLabel, setDot, item, labelNode, levelColor, clockPref } from './core.js';
import { filterFn } from './filter.js';

const PAGE = 500;
// Pages read in a row for one Load older while the filters hide them all.
const AUTO_PAGES = 20;
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

function loadFilter() {
  let f = {};
  try { f = JSON.parse(localStorage.getItem(FILTER_KEY)) || {}; } catch (e) {}
  return {
    kinds: Array.isArray(f.kinds) ? f.kinds.filter((k) => KINDS.some((g) => g.id === k)) : ['connect'],
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
  try { localStorage.setItem(FILTER_KEY, JSON.stringify(filter)); } catch (e) {}
}

const kindOn = (k) => KINDS.some((g) => g.kinds.includes(k) && filter.kinds.includes(g.id));
const problem = (e) => e.result !== 'ack' || e.tries > 1;
const off = () => (S.cfg ? S.cfg.numberOffset : 0);

// An expression filters out entries without that side.
function passes(e) {
  if (!kindOn(KINDS.some((g) => g.kinds.includes(e.kind)) ? e.kind : 'other')) return false;
  if (filter.problems && !problem(e)) return false;
  if (filter.src.trim() && (e.src < 0 || !srcOk(e.src + off(), e.srcName || ''))) return false;
  if (filter.dst.trim() && (e.dst < 0 || !dstOk(e.dst + off(), e.dstName || ''))) return false;
  return true;
}

function initFilters() {
  const box = $('log-kinds');
  for (const g of KINDS) {
    const input = el('input', 'pb-check__input');
    input.type = 'checkbox';
    input.checked = filter.kinds.includes(g.id);
    input.addEventListener('change', () => {
      filter.kinds = KINDS.map((k) => k.id).filter((id) => (id === g.id ? input.checked : filter.kinds.includes(id)));
      saveFilter();
      render();
    });
    const label = el('span', 'pb-check__label', g.label);
    if (g.id === 'keepalive') {
      const n = el('span', 'pb-text-muted');
      n.id = 'log-keep';
      label.append(' ', n);
    }
    box.append(el('label', 'pb-check', input, label));
  }
  const problems = $('log-problems');
  problems.checked = filter.problems;
  problems.addEventListener('change', () => {
    filter.problems = problems.checked;
    saveFilter();
    render();
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
let more = false;
let loading = false;
let autoLeft = 0;
let started = false; // asked for the newest page on this connection

function add(entries) {
  const fresh = entries.filter((e) => !seen.has(e.seq));
  for (const e of fresh) seen.add(e.seq);
  return fresh;
}

function request(before) {
  if (wsSend({ t: 'sent', before, n: PAGE })) loading = true;
  renderMore();
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
  const oldest = list.length ? list[list.length - 1].seq : -1;
  if (m.before === -1) {
    // Live entries may have come in first: merge.
    list = [...list, ...add(m.entries)].sort((a, b) => b.seq - a.seq);
  } else if (m.before === oldest) {
    list.push(...add(m.entries));
  } else {
    return; // for a list since replaced
  }
  more = m.more;
  loading = false;
  showCounts(m);
  render();
  // Keep reading back while the filters hide everything read so far.
  if (more && autoLeft > 0 && !tbody.childElementCount) {
    autoLeft--;
    request(list[list.length - 1].seq);
  } else {
    autoLeft = 0;
  }
});

on('sentAdd', (m) => {
  if (!started) return;
  showCounts(m);
  const fresh = add(m.entries).sort((a, b) => b.seq - a.seq);
  if (!fresh.length) return;
  list = [...fresh, ...list];
  if (newestFirst()) {
    tbody.prepend(...fresh.filter(passes).map((e) => row(e, true)));
    renderMore();
  } else {
    render(new Set(fresh.map((e) => e.seq)));
  }
});

// Link checks that went fine are counted on the server, not listed.
function showCounts(m) {
  const k = m.keepalives || 0;
  $('log-keep').textContent = k ? `(${k.toLocaleString()} OK since restart, not shown)` : '';
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
    el('td', null, KIND_LABEL[e.kind] || e.kind, ' ', el('span', 'pb-text-muted', `0x${e.cmd.toString(16).toUpperCase().padStart(2, '0')}`)),
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
try {
  const saved = JSON.parse(localStorage.getItem(SORT_KEY));
  if (SORTS[saved?.col] && ['asc', 'desc'].includes(saved.dir)) sort = saved;
} catch (e) {}
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

// The rows shown, in the order shown: also what the CSV holds.
function shownRows() {
  const rows = list.filter(passes);
  if (newestFirst()) return rows;
  const key = SORTS[sort.col], k = sort.dir === 'asc' ? 1 : -1;
  return rows
    .map((e) => ({ e, v: key(e) }))
    .sort((x, y) => {
      if (x.v == null || y.v == null) return x.v == null ? (y.v == null ? y.e.seq - x.e.seq : 1) : -1;
      return k * cmp(x.v, y.v) || y.e.seq - x.e.seq;
    })
    .map((x) => x.e);
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
  try {
    if (newestFirst()) localStorage.removeItem(SORT_KEY);
    else localStorage.setItem(SORT_KEY, JSON.stringify(sort));
  } catch (e) {}
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

// fresh: seqs that just arrived, animated in.
function render(fresh) {
  tbody.replaceChildren(...shownRows().map((e) => row(e, !!fresh?.has(e.seq))));
  renderMore();
}

function renderMore() {
  const shown = tbody.childElementCount;
  $('log-count').textContent = list.length ? `${shown} of ${list.length} loaded` : '';
  $('log-csv').disabled = !shown;
  const older = $('log-older');
  older.hidden = !more;
  older.disabled = loading;
  older.textContent = loading ? 'Loading…' : 'Load older';
  const empty = $('log-empty');
  empty.hidden = shown > 0 || loading;
  empty.textContent = !list.length
    ? (more ? '' : 'Nothing sent yet.')
    : more ? 'Nothing loaded matches the filters.' : 'Nothing matches the filters.';
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
  request(list[list.length - 1].seq);
});

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
