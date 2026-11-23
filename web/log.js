// The /log page: Communication Log rows, newest first and live, with this browser's filters and the uptime graph.
import { $, el, S, on, connect, wsSend, connState, item, labelNode } from './core.js';
import { levelColor } from './levelview.js';
import { clockPref } from './device.js';
import { filterFn } from './filter.js';
import { readJSON, writeJSON } from './store.js';
import { newPager, rowKey, byNewest } from './logpager.js';
import { KINDS, KIND_IDS, WHY_LABEL, kindLabel, normalizeKinds, allKinds, pressKind, addNewKinds } from './logkinds.js';
import { isEvent, problem, logPasses, sortKeys, cmpKeyed, newestFirst as newestOf, sortRows } from './logfilter.js';
import { initDownloads } from './logdownload.js';
import { fieldText, saveSummary } from './logsaves.js';
import { statusText } from './statustext.js';
import { PRESETS, DEFAULT_PRESET, presetText, slotLabel, rangeFor, slotFor, summary } from './uptime.js';
import { uptimeBars } from './uptimebars.js';
import './brand.js';
import { lockState } from './lock.js';
import { lockCovers } from './lockrules.js';
import './inboxdialog.js';
import { showApprovalCards } from './approvalcards.js';
import './palette.js';
import './homekey.js';
import './themekey.js';
import { bigScrollAll } from './bigscroll.js';
import { pullToLoad } from './widgets.js';
import { panelsBar, barTitle } from './panelsbar.js';

panelsBar(document.querySelector('.sode-log__head'));

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-log__title');
  barTitle(title, text, title.dataset.page);
  document.title = title.dataset.pbTip;
});

// Entries are kept in finish order but stamped with when first sent (or queued), so the log
// is not strictly in time order. A retried message can finish after later ones. Reading back
// for a range goes this much past its start to catch those.
const TIME_SLACK = 30_000;
const FILTER_KEY = 'sode-log-filter';

function loadFilter() {
  const f = readJSON(FILTER_KEY) || {};
  return {
    kinds: Array.isArray(f.kinds) ? normalizeKinds(addNewKinds(f.kinds, f.known), []) : ['connect', 'command', 'api', 'link', 'settings'],
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
  writeJSON(FILTER_KEY, { ...filter, known: KIND_IDS });
}

const off = () => (S.cfg ? S.cfg.numberOffset : 0);

// Not kept: a range far back means reading the log back to it, which should not happen on every
// open.
const range = { from: null, to: null };

// What the page shows, and what Download filtered rows keeps of the whole log.
const pageFilter = () => ({ ...filter, from: range.from, to: range.to });
const passes = (e) => logPasses(e, pageFilter(), off(), { src: srcOk, dst: dstOk });

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
  if (!wsSend({ t: 'log', ...req })) pager.unsent();
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

// A locked page's asks are refused, and each refusal writes a row; 'unlocked' loads.
const reload = () => {
  if (lockCovers(lockState())) return;
  send(pager.open());
  render();
  refreshUptime();
};
on('unlocked', reload);
on('hello', reload);

on('logPage', (m) => {
  const r = pager.receive(m);
  if (!r) return;
  showCounts(m);
  // Older entries go at the end newest first. In any other order the table is drawn one time when
  // reading back stops, not after every page.
  if (r.newest) render();
  else if (newestFirst()) appendRows(r.fresh.filter(passes));
  // Read on while a range is not covered or the filters leave fewer than a page. Opening or Load
  // older should not look like nothing happened.
  const req = rangeNeedsMore() ? pager.back() : pager.next(pager.list.filter(passes).length);
  if (req) return send(req);
  if (!r.newest && !newestFirst()) render();
  else renderMore();
});

on('logAdd', (m) => {
  if (!pager.started) return;
  showCounts(m);
  const { fresh, dropped } = pager.live(m);
  if (!fresh.length) return;
  const pass = fresh.filter(passes);
  if (newestFirst()) {
    tbody.prepend(...pass.map((e) => row(e, true)));
    shown.unshift(...pass);
  } else {
    for (const e of pass) insertRow(e);
  }
  dropRows(dropped);
  renderMore();
});

function dropRows(es) {
  if (!es.length) return;
  const gone = new Set(es.map(rowKey));
  for (let i = shown.length - 1; i >= 0; i--) {
    if (!gone.has(rowKey(shown[i]))) continue;
    tbody.children[i].remove();
    shown.splice(i, 1);
  }
}

function showCounts(m) {
  const n = m.lost || 0;
  const note = $('log-lost');
  note.hidden = !n;
  note.textContent = n ? `${n} message${n === 1 ? '' : 's'} not saved since restart. See the server log.` : '';
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
  approved: ['ok', 'Approved'],
  denied: ['warn', 'Denied'],
  timeout: ['warn', 'No one answered'],
  cancelled: ['idle', 'Cancelled'],
};

function outcome(e) {
  if (isEvent(e)) {
    const [tone, text] = e.kind === 'server' && e.err ? ['warn', 'Started with a problem'] : EVENT_RESULT[e.result] || ['idle', e.result];
    const s = statusText(tone, text);
    if (e.err) s.dataset.pbTip = e.err;
    return s;
  }
  const tries = `${e.tries} ${e.tries === 1 ? 'try' : 'tries'}`;
  const [tone, text] = {
    ack: e.tries > 1 ? ['warn', `ACK after ${tries}`] : ['ok', e.ackMs ? `ACK in ${fmtMs(e.ackMs)}` : 'ACK'],
    nak: ['error', `NAK, gave up after ${tries}`],
    noack: ['error', `No ACK after ${tries}`],
    write: ['error', 'Write failed'],
    dropped: ['error', 'Not sent: link dropped'],
    unsent: ['error', 'Not sent'],
  }[e.result] || ['idle', e.result];
  const s = statusText(tone, text);
  if (e.err) s.dataset.pbTip = e.err;
  return s;
}

const fmtMs = (ms) => (ms < 10 ? ms.toFixed(1) : String(Math.round(ms))) + ' ms';

function levelCell(e) {
  const l = S.levelBy.get(e.level);
  if (l) return levelColor(el('span', 'sode__slot sode__slot--on sode-log__lvl', l.label), l);
  return e.wire >= 0 ? el('span', 'pb-text-muted', `router level ${e.wire}`) : '';
}

const side = (n, name) => (n >= 0 ? labelNode(item(n, name || '')) : '');

const saveFields = (s) =>
  el(
    'details',
    'sode-log__save',
    el('summary', null, saveSummary(s)),
    el(
      'ul',
      'sode-log__save-fields',
      ...(s.fields || []).map((f) => el('li', null, fieldText(f))),
      ...(s.interruptions || []).map((t) => el('li', null, `Interruption: ${t}`)),
    ),
  );

function row(e, fresh = false) {
  const why = WHY_LABEL[e.why] || e.why || '';
  const tr = el(
    'tr',
    fresh ? 'sode--enter' : null,
    el('td', 'sode-log__time', ...fmtStamp(e.time)),
    el('td', null, isEvent(e) ? '' : el('span', 'pb-text-muted', `0x${e.cmd.toString(16).toUpperCase().padStart(2, '0')} `), kindLabel(e)),
    el('td', null, levelCell(e)),
    el('td', 'sode-log__name', side(e.dst, e.dstName)),
    el('td', 'sode-log__name', side(e.src, e.srcName)),
    el(
      'td',
      null,
      why,
      e.by ? el('span', 'pb-text-muted sode-log__why-part', e.by) : '',
      e.note ? el('span', 'pb-text-muted sode-log__why-part', e.note) : '',
      e.kind === 'server' && e.err ? el('span', 'pb-text-muted sode-log__why-part', e.err) : '',
      e.save ? saveFields(e.save) : '',
    ),
    el('td', null, outcome(e)),
    el('td', 'sode-log__hex', e.hex),
  );
  if (problem(e)) tr.classList.add('sode-log__row--problem');
  return tr;
}

// ---- sorting (logfilter.js): ties keep newest first. Rows without the column go last in both directions.

const SORT_KEY = 'sode-log-sort';
const SORTS = sortKeys();
let sort = { col: 'time', dir: 'desc' };
{
  const saved = readJSON(SORT_KEY);
  if (SORTS[saved?.col] && ['asc', 'desc'].includes(saved.dir)) sort = saved;
}
const newestFirst = () => newestOf(sort);

function order(x, y) {
  if (newestFirst()) return byNewest(x, y);
  const key = SORTS[sort.col];
  return cmpKeyed(sort, x, key(x), y, key(y));
}

function shownRows() {
  const rows = pager.list.filter(passes);
  return newestFirst() ? rows : sortRows(rows, sort, SORTS);
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

// The server builds both files from the whole log (logdownload.js).
initDownloads((rows) => sortRows(rows.filter(passes), sort, SORTS));

const loadOlder = () => send(pager.older(pager.list.filter(passes).length));
$('log-older').addEventListener('click', loadOlder);
pullToLoad(window, $('log-older').parentElement, { hasMore: () => !$('log-older').hidden && !pager.loading, more: loadOlder }, () => true);

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
  if (S.wsState !== 'up') {
    pager.close();
    renderMore();
  }
});
// Uptime graph: its chips set the log's time range (From = that long ago, no To) and move with the clock. No times set = the last 24 hours. Only the newest reply is drawn.

let chipOn = null;
// Slot mode: a bar was clicked and the log shows only its slot. The strip keeps its window (view)
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
    b.dataset.pbTip = `Bar = ${slotLabel(slotFor(0, p.span))}`;
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
    upSum.textContent = 'From is after To.';
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
  note.replaceChildren(el('strong', null, 'Slot'), `${fmtDay(slotMode.from)} to ${fmtDay(slotMode.to)}`, back);
}

on('uptimeResult', (m) => {
  if (m.id !== upId) return;
  const data = { start: m.start, trans: m.trans, counts: m.counts, from: m.from, to: m.to, slot: m.slot, now: m.now };
  const text = summary(data);
  upLast = { from: m.from, to: m.to };
  upBars.replaceChildren(...uptimeBars(data, { fmt: fmtDay, pick: enterSlot, sel: slotMode?.from }));
  const label =
    slotMode || chipOn || !customTimes()
      ? slotMode
        ? `${fmtDay(m.from)} to ${fmtDay(m.to)}`
        : PRESETS.find((p) => p.id === (chipOn || DEFAULT_PRESET)).label
      : `${fmtDay(m.from)} to ${fmtDay(m.to)}`;
  upBars.setAttribute('aria-label', `Router uptime, ${label}: ${text}`);
  upSum.textContent = text;
});

setInterval(() => lockCovers(lockState()) || refreshUptime(), 60_000);

initFilters();
initUptime();
bigScrollAll();
showApprovalCards();
connect();
