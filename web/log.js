// The /log page: every message Sode wrote to the router, plus browser connects, commands and
// router link changes, newest first, live, with this browser's filters and the router uptime
// graph.
import { $, el, S, on, connect, wsSend, connState, routerLabel, setDot, item, labelNode } from './core.js';
import { levelColor } from './levelview.js';
import { clockPref } from './device.js';
import { filterFn } from './filter.js';
import { readJSON, writeJSON } from './store.js';
import { newPager } from './logpager.js';
import { KINDS, KIND_IDS, EVENT_KINDS, normalizeKinds, allKinds, pressKind } from './logkinds.js';
import { PRESETS, DEFAULT_PRESET, presetText, slotLabel, rangeFor, slotFor, slots, summary, slotText } from './uptime.js';
import './brand.js';
import './lock.js';

on('brand', ({ text }) => {
  document.querySelector('.sode-log__title').textContent = `${text} Communication Log`;
  document.title = `${text} Communication Log \u00b7 ${text}`;
});

// Entries are kept in finish order but stamped with when first sent (or queued), so the log
// isn't strictly in time order (a retried message can finish after later ones). Reading back
// for a range goes this much past its start to catch those.
const TIME_SLACK = 30_000;
const FILTER_KEY = 'sode-log-filter';

const KIND_LABEL = {
  connect: 'Connect',
  interrogate: 'Interrogate',
  keepalive: 'Keepalive',
  tallyDump: 'Tally dump',
  protectDump: 'Protect dump',
  srcNames: 'Source names',
  dstNames: 'Destination names',
  other: 'Other',
  salvoStage: 'Salvo stage',
  salvoGo: 'Salvo go',
  browser: 'Browser',
  command: 'Command',
  link: 'Router link',
  server: 'Sode',
};
const WHY_LABEL = {
  take: 'Take',
  undo: 'Undo',
  retry: 'Retry',
  check: 'Check before take',
  confirm: 'Confirm take',
  resync: 'Resync',
  names: 'Name refresh',
  keepalive: 'Link check',
  connect: 'WebSocket connected',
  disconnect: 'WebSocket dropped',
  refreshNames: 'Refresh names',
  clearHistory: 'Clear history',
  config: 'Settings saved',
  filterSets: 'Filter sets saved',
  presets: 'Presets saved',
  salvos: 'Presets saved', // salvos: rows logged before the rename
  unlock: 'Settings PIN',
  lock: 'Settings locked',
  salvo: 'Native salvo',
  userUnlock: 'User PIN',
  userLock: 'User locked',
  adminUnlock: 'Admin password (old rows)',
  users: 'Users saved',
};
const isEvent = (e) => EVENT_KINDS.has(e.kind);

function loadFilter() {
  const f = readJSON(FILTER_KEY) || {};
  return {
    kinds: normalizeKinds(f.kinds, ['connect', 'command', 'link']),
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
const EVENT_PROBLEM = { command: ['refused', 'locked', 'error', 'fallback'], link: ['down'] };
const problem = (e) => (isEvent(e) ? !!EVENT_PROBLEM[e.kind]?.includes(e.result) : e.result !== 'ack' || e.tries > 1);
const off = () => (S.cfg ? S.cfg.numberOffset : 0);

// Not kept: a range far back means reading the log back to it, which shouldn't happen on every
// open.
const range = { from: null, to: null };

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
  const chip = (label, press) => {
    const b = el('button', 'pb-chip sode-log__chip', label);
    b.type = 'button';
    b.addEventListener('click', () => {
      press();
      saveFilter();
      sync();
      render();
    });
    return b;
  };
  // Problems is its own dimension: after a separator, outside the All logic.
  const allChip = chip('All', () => {
    filter.kinds = KIND_IDS.slice();
  });
  const kindChips = KINDS.map((g) => ({
    id: g.id,
    b: chip(g.label, () => {
      filter.kinds = pressKind(filter.kinds, g.id);
    }),
  }));
  const problemsChip = chip('Problems only', () => {
    filter.problems = !filter.problems;
  });
  const bar = el('span', 'sode-log__sep', '|');
  bar.setAttribute('aria-hidden', 'true');
  const sync = () => {
    const all = allKinds(filter.kinds);
    allChip.setAttribute('aria-pressed', String(all));
    for (const k of kindChips) k.b.setAttribute('aria-pressed', String(!all && filter.kinds.includes(k.id)));
    problemsChip.setAttribute('aria-pressed', String(filter.problems));
  };
  box.replaceChildren(legend, allChip, ...kindChips.map((k) => k.b), bar, problemsChip);
  sync();
  // datetime-local is local time. To counts to the end of what it shows: 14:30 is up to
  // 14:30:59.999.
  const from = $('log-from'),
    to = $('log-to');
  const parse = (input, end) => {
    if (!input.value) return null;
    const t = new Date(input.value).getTime();
    if (Number.isNaN(t)) return undefined;
    return end ? t + (input.value.length <= 16 ? 59999 : 999) : t;
  };
  const applyRange = () => {
    const f = parse(from, false),
      t = parse(to, true);
    const bad = f === undefined || t === undefined || (f != null && t != null && f > t);
    from.classList.toggle('pb-input--invalid', bad);
    to.classList.toggle('pb-input--invalid', bad);
    if (bad) return;
    range.from = f;
    range.to = t;
    $('log-range-clear').hidden = f == null && t == null;
    render();
    fill();
    refreshUptime();
  };
  const byHand = (e) => {
    if (e.isTrusted) {
      chipOn = null;
      leaveSlot();
    }
    applyRange();
  };
  from.addEventListener('change', byHand);
  to.addEventListener('change', byHand);
  $('log-range-clear').addEventListener('click', () => {
    chipOn = null;
    leaveSlot();
    from.value = to.value = '';
    applyRange();
  });

  for (const side of ['src', 'dst']) {
    const input = $(`log-${side}`);
    input.value = filter[side];
    input.addEventListener('input', () => {
      const fn = filterFn(input.value);
      input.classList.toggle('pb-input--invalid', !fn);
      if (!fn) return;
      filter[side] = input.value;
      if (side === 'src') srcOk = fn;
      else dstOk = fn;
      saveFilter();
      render();
    });
  }
}

const pager = newPager();
// Kept in step with tbody's rows so new ones go in place instead of redrawing everything.
let shown = [];

function send(req) {
  if (!req) return;
  if (!wsSend({ t: 'sent', ...req })) pager.unsent();
  renderMore();
}

function rangeNeedsMore() {
  if (!pager.more || !pager.list.length) return false;
  if (range.from != null) return pager.oldest.t >= range.from - TIME_SLACK;
  if (range.to != null) return pager.oldest.t > range.to - TIME_SLACK;
  return false;
}

const rangeDone = () => range.from != null && pager.list.length > 0 && !rangeNeedsMore();

const fill = () => rangeNeedsMore() && send(pager.back());

on('unlocked', () => {
  send(pager.open());
  render();
  refreshUptime();
});

on('hello', () => {
  send(pager.open());
  render();
  refreshUptime();
});

on('sentPage', (m) => {
  const r = pager.receive(m);
  if (!r) return;
  showCounts(m);
  // Older entries go at the end newest first; in any other order the table is drawn once when
  // reading back stops, not after every page.
  if (r.newest) render();
  else if (newestFirst()) appendRows(r.fresh.filter(passes));
  // Read on while a range isn't covered or the filters leave fewer than a page: opening or Load
  // older shouldn't look like nothing happened.
  const req = rangeNeedsMore() ? pager.back() : pager.next(pager.list.filter(passes).length);
  if (req) return send(req);
  if (!r.newest && !newestFirst()) render();
  else renderMore();
});

on('sentAdd', (m) => {
  if (!pager.started) return;
  showCounts(m);
  const fresh = pager.live(m);
  if (!fresh.length) return;
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
  const n = m.lost || 0;
  const note = $('log-lost');
  note.hidden = !n;
  note.textContent = n
    ? `${n} message${n === 1 ? '' : 's'} since the last restart ${n === 1 ? 'wasn’t' : 'weren’t'} saved to this log. The server log has details.`
    : '';
}

const tbody = $('log-rows');

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

const EVENT_RESULT = {
  up: ['ok', 'Router up'],
  down: ['error', 'Router down'],
  changed: ['warn', 'Router changed'],
  connected: ['ok', 'Connected'],
  disconnected: ['idle', 'Disconnected'],
  sent: ['ok', 'Sent'],
  ok: ['ok', 'Done'],
  refused: ['error', 'Refused'],
  locked: ['warn', 'Locked'],
  error: ['error', 'Failed'],
  start: ['ok', 'Started'],
  stop: ['idle', 'Stopped'],
};

function outcome(e) {
  if (isEvent(e)) {
    const [dot, text] = EVENT_RESULT[e.result] || ['idle', e.result];
    const s = el('span', 'pb-status', el('span', `pb-dot pb-dot--${dot}`), text);
    if (e.err) s.title = e.err;
    return s;
  }
  const tries = `${e.tries} ${e.tries === 1 ? 'try' : 'tries'}`;
  const [dot, text] = {
    ack: e.tries > 1 ? ['warn', `ACK after ${tries}`] : ['ok', e.ackMs ? `ACK · ${fmtMs(e.ackMs)}` : 'ACK'],
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

const fmtMs = (ms) => (ms < 10 ? ms.toFixed(1) : String(Math.round(ms))) + ' ms';

function levelCell(e) {
  const l = S.levelBy.get(e.level);
  if (l) return levelColor(el('span', 'sode__slot sode__slot--on sode-log__lvl', l.label), l);
  return e.wire >= 0 ? el('span', 'pb-text-muted', `wire ${e.wire}`) : '';
}

const side = (n, name) => (n >= 0 ? labelNode(item(n, name || '')) : '');

function row(e, fresh = false) {
  const why = WHY_LABEL[e.why] || e.why || '';
  const tr = el(
    'tr',
    fresh ? 'sode--enter' : null,
    el('td', 'sode-log__time', ...fmtStamp(e.time)),
    el('td', null, isEvent(e) ? '' : el('span', 'pb-text-muted', `0x${e.cmd.toString(16).toUpperCase().padStart(2, '0')} `), KIND_LABEL[e.kind] || e.kind),
    el('td', null, levelCell(e)),
    el('td', 'sode-log__name', side(e.dst, e.dstName)),
    el('td', 'sode-log__name', side(e.src, e.srcName)),
    el('td', null, why, e.by ? el('span', 'pb-text-muted', ` · ${e.by}`) : '', e.note ? el('span', 'pb-text-muted', ` · ${e.note}`) : ''),
    el('td', null, outcome(e)),
    el('td', 'sode-log__hex', e.hex),
  );
  if (problem(e)) tr.classList.add('sode-log__row--problem');
  return tr;
}

// ---- sorting: ties keep newest first; rows without that column go last either way.

const SORT_KEY = 'sode-log-sort';
const resultRank = (e) => (isEvent(e) ? (problem(e) ? 2 : 0) : e.result === 'ack' ? (e.tries > 1 ? 1 : 0) : 2);
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

function cmpKeyed(x, xv, y, yv) {
  if (xv == null || yv == null) return xv == null ? (yv == null ? y.seq - x.seq : 1) : -1;
  return (sort.dir === 'asc' ? 1 : -1) * cmp(xv, yv) || y.seq - x.seq;
}

function order(x, y) {
  if (newestFirst()) return y.seq - x.seq;
  const key = SORTS[sort.col];
  return cmpKeyed(x, key(x), y, key(y));
}

function shownRows() {
  const rows = pager.list.filter(passes);
  if (newestFirst()) return rows;
  const key = SORTS[sort.col];
  return rows
    .map((e) => ({ e, v: key(e) }))
    .sort((x, y) => cmpKeyed(x.e, x.v, y.e, y.v))
    .map((x) => x.e);
}

function appendRows(es) {
  tbody.append(...es.map((e) => row(e)));
  shown.push(...es);
}

function insertRow(e) {
  let lo = 0,
    hi = shown.length;
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
  $('log-count').textContent = pager.list.length ? `${shown.toLocaleString()} of ${pager.total.toLocaleString()} in log` : '';
  $('log-csv').disabled = !shown;
  const older = $('log-older');
  const busy = pager.loading;
  const filling = busy && rangeNeedsMore();
  older.hidden = !pager.more || rangeDone();
  older.disabled = busy;
  older.textContent = filling ? `Loading back to ${fmtDay(pager.oldest.t)}…` : busy ? 'Loading…' : 'Load older';
  const empty = $('log-empty');
  empty.hidden = shown > 0 || busy;
  empty.textContent = !pager.list.length
    ? pager.more
      ? ''
      : 'Nothing sent yet.'
    : rangeDone() || !pager.more
      ? 'Nothing matches the filters.'
      : 'Nothing loaded matches the filters.';
}

const CSV_COLS = [
  ['time', (e) => e.time],
  ['seq', (e) => e.seq],
  ['message', (e) => KIND_LABEL[e.kind] || e.kind],
  ['cmd', (e) => (isEvent(e) ? '' : `0x${e.cmd.toString(16).toUpperCase().padStart(2, '0')}`)],
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
  ['note', (e) => e.note || ''],
  ['ack ms', (e) => (e.ackMs ? e.ackMs : '')],
  ['bytes', (e) => e.hex],
];

// A leading = + - @ is defused so a spreadsheet doesn't run a router-supplied name as a formula.
function csvCell(v) {
  let s = String(v);
  if (/^[=+\-@\t\r]/.test(s) && !/^-?\d+(\.\d+)?$/.test(s)) s = `'${s}`;
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

$('log-csv').addEventListener('click', () => {
  const rows = shownRows();
  const text = [CSV_COLS.map(([h]) => h), ...rows.map((e) => CSV_COLS.map(([, f]) => f(e)))].map((r) => r.map(csvCell).join(',')).join('\r\n') + '\r\n';
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
  send(pager.older(pager.list.filter(passes).length));
});

const fmtDay = (t) => {
  const d = new Date(t);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

let routerWas = null;
on('conn', () => {
  const c = connState();
  if (routerWas !== null && c.rt[0] !== routerWas) refreshUptime();
  routerWas = c.rt[0];
  setDot($('log-ws-dot'), $('log-ws-text'), [c.ws[0], 'Server']);
  setDot($('log-rt-dot'), $('log-rt-text'), [c.rt[0], 'Router']);
  $('log-addr').textContent = routerLabel()[0];
  if (S.wsState !== 'up') {
    pager.close();
    renderMore();
  }
});
// ---- uptime graph. One strip whose toggles set the log's time range (From = that long ago, no To)
// and keep rolling with the clock; with no times set it shows the last 24 hours. Only the
// newest reply is drawn.

let chipOn = null;
// Slot mode: a bar was clicked and the log shows just its slot. The strip keeps its window (view)
// so neighbouring bars stay clickable; what was set before (prev) comes back with Back.
let slotMode = null;
let upLast = null;
let upId = 0;
const upBars = $('log-uptime-bars');
const upSum = $('log-uptime-sum');
const customTimes = () => range.from != null || range.to != null;

function localStamp(t) {
  const d = new Date(t),
    p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

function initUptime() {
  const box = $('log-uptime-presets');
  const chips = PRESETS.map((p) => {
    const b = el('button', 'pb-chip sode-log__chip', presetText(p));
    b.type = 'button';
    b.title = `Show the last ${p.label} and limit the log to it. Each bar of the graph is ${slotLabel(slotFor(0, p.span))}.`;
    b.addEventListener('click', () => {
      const from = $('log-from');
      from.value = localStamp(Date.now() - p.span);
      $('log-to').value = '';
      leaveSlot();
      chipOn = p.id;
      from.dispatchEvent(new Event('change'));
    });
    return { id: p.id, b };
  });
  box.replaceChildren(...chips.map((c) => c.b));
  syncPresets = () => {
    const on = chipOn || (customTimes() ? null : DEFAULT_PRESET);
    for (const c of chips) c.b.setAttribute('aria-pressed', String(c.id === on));
  };
  syncPresets();
}
let syncPresets = () => {};

function refreshUptime() {
  if (!customTimes()) chipOn = null;
  syncPresets();
  const now = Date.now();
  const r = slotMode ? slotMode.view : chipOn ? rangeFor(chipOn, null, null, now) : rangeFor(DEFAULT_PRESET, range.from, range.to, now);
  if (!r) {
    upBars.replaceChildren();
    upSum.textContent = 'The From time is after the To time.';
    return;
  }
  const id = ++upId;
  if (!wsSend({ t: 'uptime', id, from: r.from, to: r.to, slot: slotFor(r.from, r.to) })) return;
  if (!upBars.childElementCount) upSum.textContent = 'Loading…';
}

function enterSlot(s) {
  const from = $('log-from'),
    to = $('log-to');
  const prev = slotMode ? slotMode.prev : { from: from.value, to: to.value, chip: chipOn, sort: { ...sort } };
  const view = slotMode ? slotMode.view : upLast;
  slotMode = { from: s.from, to: s.to, view, prev };
  chipOn = null;
  sort = { col: 'time', dir: 'desc' };
  writeJSON(SORT_KEY, null);
  renderSort();
  from.value = localStamp(s.from);
  to.value = localStamp(s.to - 1000); // To counts to the end of the second it shows
  from.dispatchEvent(new Event('change'));
  showSlot();
}

function leaveSlotBack() {
  const { prev } = slotMode;
  slotMode = null;
  chipOn = prev.chip;
  sort = prev.sort;
  writeJSON(SORT_KEY, newestFirst() ? null : sort);
  renderSort();
  $('log-from').value = prev.from;
  $('log-to').value = prev.to;
  $('log-from').dispatchEvent(new Event('change'));
  showSlot();
}

function leaveSlot() {
  if (!slotMode) return;
  slotMode = null;
  showSlot();
}

function showSlot() {
  const note = $('log-slot');
  note.hidden = !slotMode;
  if (!slotMode) return;
  const back = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Back');
  back.type = 'button';
  back.addEventListener('click', leaveSlotBack);
  note.replaceChildren(el('strong', null, 'One slot of the graph'), ` · ${fmtDay(slotMode.from)} to ${fmtDay(slotMode.to)} · newest first`, back);
}

on('uptimeResult', (m) => {
  if (m.id !== upId) return;
  const data = { start: m.start, trans: m.trans, counts: m.counts, from: m.from, to: m.to, slot: m.slot, now: m.now };
  const text = summary(data);
  upLast = { from: m.from, to: m.to };
  upBars.replaceChildren(
    ...slots(data).map((s) => {
      const bar = el('button', `sode-log__up-bar sode-log__up-bar--${s.colour}`);
      bar.type = 'button';
      bar.title = slotText(s, fmtDay);
      bar.setAttribute('aria-label', bar.title);
      bar.disabled = s.colour === 'future';
      const sel = !!slotMode && s.from === slotMode.from;
      if (sel) bar.classList.add('sode-log__up-bar--sel');
      bar.setAttribute('aria-pressed', String(sel));
      bar.addEventListener('click', () => enterSlot(s));
      return bar;
    }),
  );
  const label =
    slotMode || chipOn || !customTimes()
      ? slotMode
        ? `${fmtDay(m.from)} to ${fmtDay(m.to)}`
        : PRESETS.find((p) => p.id === (chipOn || DEFAULT_PRESET)).label
      : `${fmtDay(m.from)} to ${fmtDay(m.to)}`;
  upBars.setAttribute('aria-label', `Router uptime, ${label}: ${text}`);
  upSum.textContent = text;
});

setInterval(refreshUptime, 60_000);

initFilters();
initUptime();
connect();
