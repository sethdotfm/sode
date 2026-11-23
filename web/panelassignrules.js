// Assign (GLOSSARY.md): the panels page's Assign to list and the panel page's switch to the target. Pure, so Node
// tests it. Server side: POST /api/v1/panels/{id}/assign and the instance stream's last line (internal/app/panelassign.go).
import { hasClient } from './panelsrules.js';
import { shortSecret } from './panellink.js';
import { durationText } from './panelwatchrules.js';
import { wordScore } from './findrules.js';
import { stepPanel } from './panelsrules.js';
import { INSTANCE_HOLD } from './panelkeyrules.js';

// How an Assign to the panel lands: its client's key, the device's own sign-in, or a choice of both with the key
// first. The values are the assign route's as.
export function assignWays(panel) {
  if (panel?.acts === 'both') return ['client', 'user'];
  return hasClient(panel) ? ['client'] : ['user'];
}

// Whether user can open the panel through a sign-in, as panelReach in internal/app/apipanels.go decides; keep in step.
function opens(panel, user, trusted) {
  const same = (n) => String(n || '').toLowerCase() === String(user || '').toLowerCase();
  const share = panel?.share || {};
  return trusted || same(panel?.owner) || share.mode === 'everyone' || (share.mode === 'named' && (share.users || []).some(same));
}

// signIn is the server's word for who opened the last stream; "ended" and "none" are a page with no working key.
export function signInText(i) {
  if (i.signIn === 'client') return "Signed in as the panel's API client";
  if (i.signIn === 'user' && i.user) return `Signed in as ${i.user}`;
  if (i.signIn === 'ended') return 'Key ended';
  return 'Not signed in';
}

// Whole minutes: the list redraws only when a row's text changes, and it is read every MON_POLL_MS.
export const uptimeText = (secs) => (secs < 60 ? 'under a minute' : durationText(Math.floor(secs / 60) * 60));

// Rows: the Sode panel pages of the panels read with instances, by panel name, then instance name. timedOut: no stream now
// (the server keeps an instance 24 hours). notShared (bySignIn only): the user cannot open the target.
export function assignRows(panels, targetId, now, bySignIn = false) {
  const target = panels.find((p) => p.id === targetId);
  const rows = [];
  const secs = (t) => (now - Date.parse(t)) / 1000;
  for (const p of panels) {
    for (const i of p.instances || []) {
      if (!i.page) continue;
      const here = p.id === targetId;
      const timedOut = !i.stream;
      rows.push({
        key: `${p.id}\n${i.device}`,
        panel: p.id,
        panelName: p.name,
        device: i.device,
        addr: i.addr || '',
        state: signInText(i),
        user: i.signIn === 'user' ? i.user || '' : '',
        uptime: timedOut ? `last seen ${uptimeText(secs(i.seen))} ago` : `up ${uptimeText(secs(i.streamSince))}`,
        here,
        timedOut,
        assignable: !here && !timedOut,
        notShared: bySignIn && i.signIn === 'user' && !opens(target, i.user, i.trusted),
        keyless: '',
      });
    }
    // A keyless page is never here: an Assign to the panel it shows is how it gets a key.
    for (const i of p.keyless || []) {
      const timedOut = !i.stream;
      rows.push({
        key: `${p.id}\n${i.device}\n${i.keyless}`,
        panel: p.id,
        panelName: p.name,
        device: i.device,
        addr: i.addr || '',
        state: signInText(i),
        user: '',
        uptime: timedOut ? `last seen ${uptimeText(secs(i.seen))} ago` : `up ${uptimeText(secs(i.streamSince))}`,
        here: false,
        timedOut,
        assignable: !timedOut,
        notShared: false,
        keyless: i.keyless,
      });
    }
  }
  const by = (a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' });
  return rows.sort((a, b) => by(a.panelName, b.panelName) || by(a.device, b.device));
}

// Find over the list, scored as the panel Find (wordScore). A name hit ranks above the rest; every word must hit some field.
// The stable sort keeps the list's order for equal scores.
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

export const stepRow = (rows, cur, step) =>
  stepPanel(
    rows.map((r) => r.key),
    curRow(rows, cur),
    step,
  );

export const enterRow = (rows, cur) => {
  const r = rows.find((x) => x.key === curRow(rows, cur));
  return r?.assignable ? r : null;
};

// A JSON line can arrive split across chunks; rest waits for the next one.
export function splitLines(rest, chunk) {
  const parts = (rest + chunk).split('\n');
  return { lines: parts.slice(0, -1).filter((l) => l.trim()), rest: parts.at(-1) };
}

// The keyless announcement (internal/app/panelkeyless.go): no key, the name the page holds and whether its key ended.
export function keylessPath(seg, device, ended) {
  const q = new URLSearchParams();
  const name = String(device || '').trim();
  if (name) q.set('instance', name);
  if (ended) q.set('ended', '1');
  const qs = q.toString();
  return `api/v1/panels/${encodeURIComponent(seg)}/instances/keyless${qs ? `?${qs}` : ''}`;
}

// An assign line, or null for the keepalive lines and anything malformed. key is '' when the target signs in a user.
export function assignOf(line) {
  let m;
  try {
    m = JSON.parse(line);
  } catch {
    return null;
  }
  if (m?.t !== 'assign' || !m.panel || !m.instance || !(m.key || m.signIn === 'user')) return null;
  return { panel: m.panel, slug: m.slug || '', instance: m.instance, key: m.key || '' };
}

// The target's address, built as linkFor builds one. The fragment keeps its kiosk part and drops an old key or setup token.
// seg is what the page stores the key under (it reads the stored key by address); stored false puts the key in the fragment.
export function assignAddress(msg, { search, hash }, stored) {
  const seg = msg.slug || msg.panel;
  const keep = hash
    .replace(/^#/, '')
    .split('&')
    .filter((p) => p && !p.startsWith('key=') && !p.startsWith('setup='));
  const frag = [...(stored || !msg.key ? [] : [`key=${shortSecret(msg.key)}`]), ...keep].join('&');
  // The keepalive reads ?instance= before the stored name, so a name chosen after a clash replaces it here.
  const q = new URLSearchParams(search || '');
  if (q.has('instance')) q.set('instance', msg.instance);
  const qs = q.toString();
  return { seg, href: `panels/${seg}${qs ? `?${qs}` : ''}${frag ? `#${frag}` : ''}` };
}

// 409 name_taken carries the server's suggestion; any other failure gives null.
export const clashOf = (err) => (err?.code === 'name_taken' ? err.suggest || '' : null);

// A name the server still finds taken comes back as another clash.
export function assignNameProblem(text, clashed) {
  const name = String(text || '').trim();
  if (!name) return INSTANCE_HOLD;
  if (name.toLowerCase() === String(clashed || '').toLowerCase()) return `The panel already has an instance named ${name}.`;
  return '';
}

// '' when only a non-Sode page holds the stream; 'offline' when no stream would hear it.
export const streamState = (i) => (!i?.page && !i?.keyless ? '' : i.stream ? 'ready' : 'offline');

export const renameRefusal = (err) => ({ why: err?.message || '' });

export function renameProblem(text) {
  const name = String(text || '').trim();
  if (!name) return INSTANCE_HOLD;
  return '';
}

// The new name from the stream's rename line, or '' for any other line.
export function renameOf(line) {
  try {
    const m = JSON.parse(line);
    return m?.t === 'rename' && typeof m.instance === 'string' ? m.instance.trim() : '';
  } catch {
    return '';
  }
}

// The address query of a page named by its link, which carries the new name so a reload keeps it.
export function renamedSearch(search, name) {
  const q = new URLSearchParams(search || '');
  q.set('instance', name);
  return `?${q}`;
}
