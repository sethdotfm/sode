// The token stays in memory, so sign-in storage does not change.

import { el, on, onMsg, wsSend } from './core.js';
import { PREFS } from './prefs.js';
import { ownPref } from './device.js';
import { localSets } from './filterstore.js';
import { localPresets } from './presetstore.js';
import { readStr, writeStr } from './store.js';
import { backupBody, fileNameOf } from './exportrules.js';

const SESSION_HEADER = 'X-Sode-Session'; // sessionHeader in internal/app/exportroute.go
const LAST_BACKUP = 'sode-last-backup'; // ISO time (docs/persisted-names.md)

let token = null; // null: not asked yet; '' with no users

// This browser: its own device preferences as top-level keys, with its filter sets and presets (browser.yaml in
// internal/app/testdata/export-v1).
export function browserValues() {
  const out = {};
  for (const n of Object.keys(PREFS)) if (ownPref(n)) out[n] = ownPref(n);
  return { ...out, filterSets: localSets(), presets: localPresets() };
}

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

export async function call(path, init = {}) {
  const tok = await session();
  const headers = { ...init.headers };
  if (tok) headers[SESSION_HEADER] = tok;
  const res = await fetch(path, { ...init, headers, cache: 'no-store' });
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    const code = err?.error?.code || '';
    // Ask for a new session next time; a refused Critical import keeps its session.
    if (res.status === 401 || (res.status === 403 && code !== 'recheck')) token = null;
    throw Object.assign(new Error(err?.error?.message || err?.message || `Sode answered ${res.status}.`), { code });
  }
  return res;
}

export const post = (path, b) => call(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(b) });

// Resolves { name, size } of the saved zip. path: api/own/export for My user's own items.
export async function downloadZip(b, path = 'api/export') {
  const res = await post(path, b);
  const blob = await res.blob();
  const a = el('a');
  a.href = URL.createObjectURL(blob);
  a.download = fileNameOf(res.headers.get('Content-Disposition'), new Date().toISOString().slice(0, 10));
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 0);
  return { name: a.download, size: blob.size };
}

export async function downloadBackup() {
  const got = await downloadZip(backupBody(browserValues()));
  writeStr(LAST_BACKUP, new Date().toISOString());
  return got;
}

export const lastBackup = () => readStr(LAST_BACKUP);

// A sign-in as someone else, or a sign-out, ends what the token stood for.
on('access', () => {
  token = null;
});
