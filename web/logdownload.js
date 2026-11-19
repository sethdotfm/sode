// /log downloads. The server builds each file from the whole log for this reader (internal/app/logdownload.go).
// A request names parts, so a new part needs no new route.
import { $, el, request } from './core.js';

const SESSION_HEADER = 'X-Sode-Session'; // sessionHeader in internal/app/exportroute.go
const FULL = ['log', 'uptimeStates', 'uptimeMinutes'];

// A Guest gets no token. The server then reads the browser as a Guest, which works while /log has no lock.
async function token() {
  const m = await request('exportSessionResult', { t: 'exportSession' });
  return m.ok ? m.token || '' : '';
}

const stamp = (d) => {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}`;
};

async function download(body, ext) {
  const tok = await token();
  const res = await fetch('api/log/download', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(tok ? { [SESSION_HEADER]: tok } : {}) },
    body: JSON.stringify(body),
    cache: 'no-store',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    throw new Error(err?.error?.message || `Sode answered ${res.status}.`);
  }
  const a = el('a');
  a.href = URL.createObjectURL(await res.blob());
  a.download = `sode-log-${stamp(new Date())}.${ext}`;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

// view() gives the page's filter and sort when the button is pressed.
export function initDownloads(view) {
  const note = $('log-download-err');
  const wire = (id, build) => {
    const b = $(id);
    const label = b.textContent;
    b.addEventListener('click', async () => {
      const buttons = [$('log-download-full'), $('log-download-rows')];
      for (const x of buttons) x.disabled = true;
      b.textContent = 'Preparing…';
      note.hidden = true;
      try {
        await download(...build());
      } catch (e) {
        note.textContent = `Download failed: ${e.message}`;
        note.hidden = false;
      } finally {
        b.textContent = label;
        for (const x of buttons) x.disabled = false;
      }
    });
  };
  wire('log-download-full', () => [{ parts: FULL }, 'zip']);
  wire('log-download-rows', () => {
    const { filter, sort } = view();
    return [{ parts: ['log'], filter, sort }, 'csv'];
  });
}
