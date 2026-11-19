// The export and import routes take an Administrator's session token in a header; exportSession gives one to a
// connection that has none. The token stays in memory here, so sign-in storage is not changed.

import { on, onMsg, wsSend } from './core.js';

const SESSION_HEADER = 'X-Sode-Session'; // sessionHeader in internal/app/exportroute.go

let token = null; // null: not asked yet; '' with no users
let askSeq = 0;
const asks = new Map();

onMsg('exportSessionResult', (m) => {
  const done = asks.get(m.id);
  if (!done) return;
  asks.delete(m.id);
  done(m);
});

function session() {
  if (token !== null) return Promise.resolve(token);
  const id = ++askSeq;
  return new Promise((resolve, reject) => {
    if (!wsSend({ t: 'exportSession', id })) return reject(new Error('Not connected to Sode. Try again when the link is back.'));
    asks.set(id, (m) => {
      if (!m.ok) return reject(new Error(m.err || 'Sode refused the export.'));
      token = m.token || '';
      resolve(token);
    });
  });
}

// Rejects with an Error whose code is the server's error code ('recheck', for example) when there is one.
export async function call(path, init = {}) {
  const tok = await session();
  const headers = { ...init.headers };
  if (tok) headers[SESSION_HEADER] = tok;
  const res = await fetch(path, { ...init, headers, cache: 'no-store' });
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    // A refused Critical import keeps its session: the token is still good.
    if (res.status === 401 || (res.status === 403 && err?.error?.code !== 'recheck')) token = null;
    throw Object.assign(new Error(err?.error?.message || err?.message || `Sode answered ${res.status}.`), { code: err?.error?.code || '' });
  }
  return res;
}

export const post = (path, b) => call(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(b) });

// A sign-in as someone else, or a sign-out, ends what the token stood for.
on('access', () => {
  token = null;
});
