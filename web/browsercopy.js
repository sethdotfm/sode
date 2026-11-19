// browserCopy makes the server write one Settings saves row (docs/agents/profiles.md).

import { S, on, onMsg, wsSend } from './core.js';
import { buildExport } from './exportfmt.js';
import { browserValues } from './exportcalls.js';
import { copyDoc, toClear } from './browsercopyrules.js';
import { openReview } from './reviewsheet.js';
import { setPref } from './device.js';
import { moveLocalSets } from './filterstore.js';
import { localPresets, putLocalPresets } from './presetstore.js';

let seq = 0;
const asks = new Map();
onMsg('browserCopyResult', (m) => {
  const done = asks.get(m.id);
  if (!done) return;
  asks.delete(m.id);
  done({ ok: !!m.ok, err: m.err || '' });
});
on('wsclose', () => {
  for (const done of asks.values()) done({ ok: false, err: 'Not connected to Sode.' });
  asks.clear();
});

function send(parts) {
  return new Promise((resolve) => {
    const id = ++seq;
    if (!wsSend({ t: 'browserCopy', id, ...parts })) return resolve({ ok: false, err: 'Not connected to Sode.' });
    asks.set(id, resolve);
  });
}

function clear(moved, keep) {
  const c = toClear(moved, keep);
  for (const name of c.prefs) setPref(name, '');
  const gone = (names) => (s) => !names.some((n) => n.toLowerCase() === s.name.toLowerCase());
  if (c.filterSets.length) moveLocalSets(c.filterSets);
  if (c.presets.length) putLocalPresets(localPresets().filter(gone(c.presets)));
}

const srcName = (n) => S.srcBy.get(n)?.name || '';
const dstName = (n) => S.dstBy.get(n)?.name || '';

export function openBrowserCopy() {
  const d = copyDoc(browserValues());
  const doc = buildExport({ ...d, srcName, dstName });
  // A move keeps each preset's id and pin; an export file carries neither.
  if (doc.presets) doc.presets = doc.presets.map((p, i) => ({ ...p, id: d.presets[i].id, pinned: d.presets[i].pinned !== false }));
  openReview({
    doc: { ...doc, browserOnly: d.browserOnly },
    scope: 'user',
    title: 'Copy this browser to My user',
    source: 'This browser',
    copy: { send, clear },
  });
}

on('openBrowserCopy', openBrowserCopy);
