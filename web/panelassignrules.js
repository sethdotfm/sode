// Assign (GLOSSARY.md): the panels page's Assign to list and the panel page's switch to the target. Pure, so Node
// tests it. Server side: POST /api/v1/panels/{id}/assign and the instance stream's last line (internal/app/panelassign.go).
import { hasClient } from './panelsrules.js';
import { shortSecret } from './panellink.js';
import { durationText } from './panelwatchrules.js';
import { wordScore } from './findrules.js';
import { stepPanel } from './panelsrules.js';
import { INSTANCE_MAX, INSTANCE_HOLD } from './panelkeyrules.js';

// Only a panel that acts as an API client has a key to give an instance.
export const canAssignTo = (panel) => hasClient(panel);

// signIn is the server's word for who opened the last stream; "ended" and "none" are a page with no working key.
export function signInText(i) {
  if (i.signIn === 'client') return "Signed in as the panel's client";
  if (i.signIn === 'user' && i.user) return `Signed in as ${i.user}`;
  if (i.signIn === 'ended') return 'Key ended';
  return 'Not signed in';
}

// Whole minutes: the list redraws only when a row's text changes, and it is read every MON_POLL_MS.
export const uptimeText = (secs) => (secs < 60 ? 'under a minute' : durationText(Math.floor(secs / 60) * 60));

// The Sode panel pages (page: they held the instance stream) of the panels the page can read with instances, sorted
// by their current panel's name, then the instance name. here marks the ones already on the target; timedOut the ones
// with no stream now, which have nothing to send on. The server keeps an instance 24 hours after it was last heard.
export function assignRows(panels, targetId, now) {
  const rows = [];
  for (const p of panels) {
    for (const i of p.instances || []) {
      if (!i.page) continue;
      const here = p.id === targetId;
      const timedOut = !i.stream;
      const secs = (t) => (now - Date.parse(t)) / 1000;
      rows.push({
        key: `${p.id}\n${i.device}`,
        panel: p.id,
        panelName: p.name,
        device: i.device,
        addr: i.addr || '',
        state: signInText(i),
        uptime: timedOut ? `last seen ${uptimeText(secs(i.seen))} ago` : `up ${uptimeText(secs(i.streamSince))}`,
        here,
        timedOut,
        assignable: !here && !timedOut,
      });
    }
  }
  const by = (a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' });
  return rows.sort((a, b) => by(a.panelName, b.panelName) || by(a.device, b.device));
}

// Find over the list: the panel Find's scoring (wordScore) on each row's name, current panel, address and sign-in
// state, a name hit outranking the rest. Every word must hit some field. Best first; the stable sort keeps the list's
// own order for equal scores and for an empty query.
export function rankAssign(rows, q) {
  const words = String(q || '')
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
  if (!words.length) return rows;
  const scored = [];
  for (const r of rows) {
    let total = 0;
    for (const w of words) {
      const f = (text, k) => wordScore({ label: text || '' }, w) * k;
      const best = Math.max(f(r.device, 1), f(r.panelName, 0.9), f(r.addr, 0.8), f(r.state, 0.7));
      if (!best) {
        total = -1;
        break;
      }
      total += best;
    }
    if (total > 0) scored.push([r, total]);
  }
  return scored.sort((a, b) => b[1] - a[1]).map(([r]) => r);
}

// The row key the highlight sits on: cur while it is still listed, else the first row.
export const curRow = (rows, cur) => (rows.some((r) => r.key === cur) ? cur : rows[0]?.key || '');

// Ctrl/Cmd+J, K (as the panels list) move the highlight, stopping at the ends.
export const stepRow = (rows, cur, step) =>
  stepPanel(
    rows.map((r) => r.key),
    curRow(rows, cur),
    step,
  );

// Enter assigns the highlighted row; a greyed or timed-out one does nothing.
export const enterRow = (rows, cur) => {
  const r = rows.find((x) => x.key === curRow(rows, cur));
  return r?.assignable ? r : null;
};

// The stream answers JSON lines that may arrive split across chunks; rest waits for the next one.
export function splitLines(rest, chunk) {
  const parts = (rest + chunk).split('\n');
  return { lines: parts.slice(0, -1).filter((l) => l.trim()), rest: parts.at(-1) };
}

// An assign line, or null for the keepalive lines and anything malformed.
export function assignOf(line) {
  let m;
  try {
    m = JSON.parse(line);
  } catch {
    return null;
  }
  if (m?.t !== 'assign' || !m.panel || !m.key || !m.instance) return null;
  return { panel: m.panel, slug: m.slug || '', instance: m.instance, key: m.key };
}

// The target's address, built as linkFor builds one: the short name when there is one, the page's own ?instance=, and
// the fragment with its kiosk part and without an old key or setup token. seg is what the page stores the key under,
// since it reads the stored key by its address. stored false puts the key in the fragment instead.
export function assignAddress(msg, { search, hash }, stored) {
  const seg = msg.slug || msg.panel;
  const keep = hash
    .replace(/^#/, '')
    .split('&')
    .filter((p) => p && !p.startsWith('key=') && !p.startsWith('setup='));
  const frag = [...(stored ? [] : [`key=${shortSecret(msg.key)}`]), ...keep].join('&');
  // The keepalive reads ?instance= before the stored name, so a name chosen after a clash replaces it here.
  const q = new URLSearchParams(search || '');
  if (q.has('instance')) q.set('instance', msg.instance);
  const qs = q.toString();
  return { seg, href: `panels/${seg}${qs ? `?${qs}` : ''}${frag ? `#${frag}` : ''}` };
}

// A name clash answers 409 name_taken with the server's suggestion (its name plus 2, or the next free number); null
// for any other failure, which has no name to offer.
export const clashOf = (err) => (err?.code === 'name_taken' ? err.suggest || '' : null);

// What is wrong with a name typed in the clash dialog, or ''. A name the server still finds taken comes back as
// another clash with a new suggestion.
export function assignNameProblem(text, clashed) {
  const name = String(text || '').trim();
  if (!name) return INSTANCE_HOLD;
  if (name.length > INSTANCE_MAX) return `Use at most ${INSTANCE_MAX} characters.`;
  if (name.toLowerCase() === String(clashed || '').toLowerCase()) return `The panel already has an instance named ${name}.`;
  return '';
}

// The seam for the editor's Authentication mode Confirm: an Assign never edits a panel, but the instance may land on a
// panel whose mode differs from the one it shows now. Where that Confirm should also cover an Assign, it hooks in here.
export const assignChangesMode = (from, to) => (from?.acts || '') !== (to?.acts || '');
