// /log downloads. The server sends every Log row for this reader, with the values the reader's Role cannot see taken
// out (internal/app/logdownload.go). The page makes the file with its own filter, sort and words (logfile.js), so
// those rules exist only here. A request names parts, so a new part needs no new route.
import { $, el, S, on, request, routerData } from './core.js';
import { logCSV, statesCSV, minutesCSV } from './logfile.js';
import { zipFiles } from './zipfile.js';

const SESSION_HEADER = 'X-Sode-Session'; // sessionHeader in internal/app/exportroute.go

// A Guest gets no token. The server then reads the browser as a Guest, which works while /log has no lock.
async function token() {
  const m = await request('exportSessionResult', { t: 'exportSession' });
  return m.ok ? m.token || '' : '';
}

const stamp = (d) => {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}`;
};

// purpose is the button, full or filtered: the server's download row says which.
async function read(parts, purpose) {
  const tok = await token();
  const res = await fetch('api/log/download', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(tok ? { [SESSION_HEADER]: tok } : {}) },
    body: JSON.stringify({ parts, for: purpose }),
    cache: 'no-store',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    throw new Error(err?.error?.message || `Sode answered ${res.status}.`);
  }
  // A reply cut by an error on the server is not valid JSON, so the download fails and makes no part file.
  return res.json();
}

function save(blob, ext) {
  const a = el('a');
  a.href = URL.createObjectURL(blob);
  a.download = `sode-log-${stamp(new Date())}.${ext}`;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

const csvBlob = (text) => new Blob([text], { type: 'text/csv;charset=utf-8' });

// The router column's words: the Router prefix, with two or more routers only. A router that is gone shows its UID.
const routerWord = () => (S.routers.length > 1 ? (uid) => S.routers.find((r) => r.uid === uid)?.prefix || uid : null);

// Numbers and level names as the page shows them. Each row's level name is that of its router.
const words = () => ({ off: routerData('').off, level: (ui, uid) => routerData(uid).levelBy.get(ui)?.label, router: routerWord() });

async function fullLog() {
  const data = await read(['log', 'uptimeStates', 'uptimeMinutes'], 'full');
  const files = [
    { name: 'log.csv', text: logCSV(data.log, words()) },
    { name: 'uptime-states.csv', text: statesCSV(data.uptimeStates, routerWord()) },
    { name: 'uptime-minutes.csv', text: minutesCSV(data.uptimeMinutes, routerWord()) },
  ];
  save(await zipFiles(files), 'zip');
}

// pick(rows) gives the rows that the page's filters pass, in the page's order.
async function filteredRows(pick) {
  const data = await read(['log'], 'filtered');
  save(csvBlob(logCSV(pick(data.log), words())), 'csv');
}

// A download needs the Server link, so both buttons stay off while it is down and while one file is made.
let busy = false;
const paintButtons = () => {
  for (const x of [$('log-download-full'), $('log-download-rows')]) x.disabled = busy || S.wsState !== 'up';
};

export function initDownloads(pick) {
  const note = $('log-download-err');
  on('conn', paintButtons);
  paintButtons();
  const wire = (id, make) => {
    const b = $(id);
    const label = b.textContent;
    b.addEventListener('click', async () => {
      busy = true;
      paintButtons();
      b.textContent = 'Preparing…';
      note.hidden = true;
      try {
        await make();
      } catch (e) {
        note.textContent = `Download failed: ${e.message}`;
        note.hidden = false;
      } finally {
        b.textContent = label;
        busy = false;
        paintButtons();
      }
    });
  };
  wire('log-download-full', fullLog);
  wire('log-download-rows', () => filteredRows(pick));
}
