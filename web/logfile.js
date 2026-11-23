// The /log download files, made in the page from the rows the server sent (docs/agents/comm-log.md). Pure (node
// tests). The server took out the values the reader's Role cannot see before it sent the rows.
import { WHY_LABEL, kindLabel, EVENT_KINDS } from './logkinds.js';
import { show, list, saveSummary } from './logsaves.js';

// The columns of the old Download CSV. Keep the names and order, since spreadsheets made from older files use them.
export const LOG_COLS = [
  'time',
  'seq',
  'message',
  'cmd',
  'level',
  'router level',
  'destination',
  'destination name',
  'source',
  'source name',
  'for',
  'by',
  'result',
  'tries',
  'error',
  'note',
  'ack ms',
  'bytes',
];

const NUMBER = /^-?\d+(\.\d+)?$/;

// A leading = + - @ is defused, so a spreadsheet does not run a router-supplied name as a formula.
export function csvCell(v) {
  let s = v == null ? '' : String(v);
  if (/^[=+\-@\t\r]/.test(s) && !NUMBER.test(s)) s = `'${s}`;
  return /[",\r\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s;
}

const csvLine = (cells) => `${cells.map(csvCell).join(',')}\r\n`;

// "Section: label: old → new". A secret, or a value hidden from the reader's Role, is "changed".
function fieldLine(f) {
  let what;
  if (f.changed) what = 'changed';
  else if (f.added || f.removed) {
    const parts = [];
    if (f.added?.length) parts.push(`added ${list(f.added)}`);
    if (f.removed?.length) parts.push(`removed ${list(f.removed)}`);
    what = parts.join('; ');
  } else what = `${show(f.old)} → ${show(f.new)}`;
  const text = f.section ? `${f.section}: ${f.label}: ${what}` : `${f.label}: ${what}`;
  return f.note ? `${text} (${f.note})` : text;
}

// The row's note, then for a Settings saves row its summary and one line per field.
function noteText(e) {
  const out = e.note ? [e.note] : [];
  if (e.save) {
    out.push(saveSummary(e.save), ...(e.save.fields || []).map(fieldLine));
    out.push(...(e.save.interruptions || []).map((t) => `Interruption: ${t}`));
  }
  return out.join('\n');
}

// opts: off is the numberOffset; level(ui) gives a level's label.
function logCells(e, { off = 0, level = () => '' }) {
  const num = (n) => (n >= 0 ? n + off : '');
  const event = EVENT_KINDS.has(e.kind);
  return [
    e.time,
    e.seq,
    kindLabel(e),
    event ? '' : `0x${e.cmd.toString(16).toUpperCase().padStart(2, '0')}`,
    e.level ? level(e.level) || '' : '',
    e.wire >= 0 ? e.wire : '',
    num(e.dst),
    e.dstName,
    num(e.src),
    e.srcName,
    e.why ? WHY_LABEL[e.why] || e.why : '',
    e.by,
    e.result,
    e.tries ?? 0,
    e.err,
    noteText(e),
    e.ackMs || '',
    e.hex,
  ];
}

// With two or more routers each file gets a last column, router: the Router prefix of the row's router. It is blank
// for a row that names no router. opts.router(uid) gives the prefix; with no opts.router the files keep their old columns.
const withRouter = (cells, router, uid) => (router ? [...cells, uid ? router(uid) : ''] : cells);
const routerCols = (cols, router) => (router ? [...cols, 'router'] : cols);

// rows in the order they go in the file.
export const logCSV = (rows, opts) =>
  csvLine(routerCols(LOG_COLS, opts.router)) + rows.map((e) => csvLine(withRouter(logCells(e, opts), opts.router, e.router))).join('');

const stamp = (ms) => new Date(ms).toISOString();
const STATE_NOTE = { up: 'Router link up', down: 'Router link down or connecting', off: 'Sode not running' };

export const statesCSV = (states, router = null) =>
  csvLine(routerCols(['time', 'state', 'note'], router)) +
  states.map((x) => csvLine(withRouter([stamp(x.t), x.s, STATE_NOTE[x.s] || ''], router, x.router))).join('');

// Only minutes with a count. m is in minutes since 1970.
export const minutesCSV = (mins, router = null) =>
  csvLine(routerCols(['minute', 'bad', 'slow'], router)) +
  mins.map((x) => csvLine(withRouter([stamp(x.m * 60_000), x.bad, x.slow], router, x.router))).join('');
